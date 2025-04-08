# Binder

## Overview
The Binder is a controller responsible for handling the pod binding process in Kubernetes. The binding process involves actually placing a pod on its selected node, as well as it's dependencies - volumes, resource claims, etc.

### Why a Separate Binder?

Traditional Kubernetes schedulers handle both node selection and binding within the same component. However, this approach has several limitations:

1. **Error Resilience**: The binding process can fail for various reasons (node state changes, resource contention, API server issues). When this happens in a monolithic scheduler, it might affect the scheduling of other pods.

2. **Performance**: Binding operations involve multiple API calls and can be slow, especially when handling Dynamic Resource Allocation (DRA) or other dependencies, such as volumes. Having the scheduler wait for these operations to complete reduces its throughput.

3. **Retry Management**: Failed bindings often need sophisticated retry mechanisms with exponential backoff, which adds complexity to the scheduler.

By separating the binding logic into its own controller, the scheduler can quickly move on to schedule other pods while the binder handles the potentially slow or error-prone binding process asynchronously.

### Communication via BindRequest API

The scheduler and binder communicate through a custom resource called `BindRequest`. When the scheduler decides where a pod should run, it creates a BindRequest object that contains:

- The pod to be scheduled
- The selected node
- Information about resource allocations (including GPU resources)
- DRA (Dynamic Resource Allocation) binding information
- Retry settings

The BindRequest API serves as a clear contract between the scheduler and binder, allowing them to operate independently.

### Example BindRequest for NVIDIA GPU DRA Driver

The NVIDIA [k8s-dra-driver-gpu](https://github.com/NVIDIA/k8s-dra-driver-gpu) leverages the upstream Dynamic Resource Allocation (DRA) API to support NVIDIA Multi-Node NVLink available in GB200 GPUs via a ComputeDomain CRD that lets you define resource templates which you can reference in your workloads.

Before running this example, make sure you have the k8s-dra-driver-gpu installed by following the [instructions from here](https://github.com/NVIDIA/k8s-dra-driver-gpu/discussions/249). And DRA feature gate enabled in the KAI-scheduler. You can set the DRA flag like this:

```bash
helm upgrade -i kai-scheduler nvidia-k8s/kai-scheduler -n kai-scheduler --set "global.registry=nvcr.io/nvidia/k8s" --set scheduler.additionalArgs[0]=--feature-gates=DynamicResourceAllocation=true --set binder.additionalArgs[0]=--feature-gates=DynamicResourceAllocation=true
```

Example pod spec with ComputeDomain

```bash
cat <<EOF > gpu-imex-pod.yaml
---
apiVersion: resource.nvidia.com/v1beta1
kind: ComputeDomain
metadata:
  name: imex-channel-injection
spec:
  numNodes: 1
  channel:
    resourceClaimTemplate:
      name: imex-channel-0
---
apiVersion: v1
kind: Pod
metadata:
  name: gpu-imex-pod
  labels:
    runai/queue: test
spec:
  schedulerName: kai-scheduler
  containers:
    - name: main
      image: ubuntu
      command: ["bash", "-c"]
      args: ["nvidia-smi; ls -la /dev/nvidia-caps-imex-channels; trap 'exit 0' TERM; sleep 9000 & wait"]
      resources:
        limits:
          nvidia.com/gpu: "1"
        claims:
        - name: imex-channel-0
  resourceClaims:
  - name: imex-channel-0
    resourceClaimTemplateName: imex-channel-0
EOF

kubectl apply -f gpu-imex-pod.yaml
```

Scheuler will autogenarte the BindRequest for the ComputeDomain

```bash
kubectl get BindRequest gpu-imex-pod-8g6vlrjxpp -o yaml

apiVersion: scheduling.run.ai/v1alpha2
kind: BindRequest
metadata:
  creationTimestamp: "2025-04-08T21:55:32Z"
  generation: 1
  labels:
    pod-name: gpu-imex-pod
    selected-node: sc-starwars-mab10-b00
  name: gpu-imex-pod-8g6vlrjxpp
  namespace: default
  ownerReferences:
  - apiVersion: v1
    kind: Pod
    name: gpu-imex-pod
    uid: 6306ffe2-a348-467a-b9aa-7176f2e95f53
  resourceVersion: "17426791"
  uid: 3c56aca5-027c-4bcd-9e9a-755e4c61ee8b
spec:
  podName: gpu-imex-pod
  receivedGPU:
    count: 1
    portion: "1.00"
  receivedResourceType: Regular
  resourceClaimAllocations:
  - allocation:
      devices:
        config:
        - opaque:
            driver: compute-domain.nvidia.com
            parameters:
              apiVersion: resource.nvidia.com/v1beta1
              domainID: 83479f70-e292-43d6-aa67-dd4ba8adab8f
              kind: ComputeDomainChannelConfig
          requests:
          - channel
          source: FromClaim
        results:
        - device: channel-0
          driver: compute-domain.nvidia.com
          pool: sc-starwars-mab10-b00
          request: channel
      nodeSelector:
        nodeSelectorTerms:
        - matchFields:
          - key: metadata.name
            operator: In
            values:
            - sc-starwars-mab10-b00
    name: imex-channel-0
  selectedNode: sc-starwars-mab10-b00
status:
  phase: Succeeded
```

### Binding Process

1. The scheduler creates a BindRequest for each pod that needs to be bound
2. The binder controller watches for BindRequest objects
3. When a new BindRequest is detected, the binder:
   - Attempts to bind the pod to the specified node
   - Handles any DRA or Persistent Volume allocations
   - Updates the BindRequest status to reflect success or failure
   - Retries failed bindings according to the backoff policy
4. Until the pod is bound, the scheduler considers the bind request status as the expected scheduling result for this pod and it's dependencies.

### Error Handling

Binding can fail for various reasons:
- The node may no longer have sufficient resources
- API server connectivity issues
- Intermittent issues with dependencies

The binder tracks failed attempts and can retry up to a configurable limit (BackoffLimit). If binding ultimately fails, the BindRequest is marked as failed, allowing the scheduler to potentially reschedule the pod.

## Extending the binder

### Binder Plugins

The binder uses a plugin-based architecture that allows for extending its functionality without modifying core binding logic. Plugins can participate in different stages of the binding process and implement specialized handling for various resource types or pod requirements.

#### Plugin Interface

All binder plugins must implement the following interface:

```go
type Plugin interface {
    // Name returns the name of the plugin
    Name() string
    
    // Validate checks if the pod configuration is valid for this plugin
    Validate(*v1.Pod) error
    
    // Mutate allows the plugin to modify the pod before scheduling
    Mutate(*v1.Pod) error
    
    // PreBind is called before the pod is bound to a node and can perform
    // additional setup operations required for successful binding
    PreBind(ctx context.Context, pod *v1.Pod, node *v1.Node, 
            bindRequest *v1alpha2.BindRequest, state *state.BindingState) error
    
    // PostBind is called after the pod is successfully bound to a node
    // and can perform cleanup or logging operations
    PostBind(ctx context.Context, pod *v1.Pod, node *v1.Node, 
             bindRequest *v1alpha2.BindRequest, state *state.BindingState)
}
```

Each method serves a specific purpose in the binding lifecycle:

- **Name**: Returns the unique identifier of the plugin.
- **Validate**: Verifies that pod configuration is valid for this plugin's concerns. For example, the GPU plugin validates that GPU resource requests are properly specified.
- **Mutate**: Allows the plugin to modify the pod spec before binding, such as injecting environment variables or container settings.
- **PreBind**: Executes before binding occurs and can perform prerequisite operations like volume or resource claim allocation.
- **PostBind**: Runs after successful binding for cleanup or logging purposes.

#### Example Plugins

##### Dynamic Resources Plugin

The Dynamic Resources plugin handles the binding of Dynamic Resource Allocation (DRA) resources to pods. It:

1. Checks if a pod has any resource claims
2. Processes the resource claim allocations specified in the BindRequest
3. Updates each resource claim with the appropriate allocation and reservation for the pod

This plugin exemplifies how to interact with Kubernetes API objects during the binding process, including handling retries for API conflicts.

##### GPU Request Validator Plugin

The GPU Request Validator plugin ensures that GPU resource requests are properly formatted and valid. It:

1. Validates that GPU resource requests/limits follow the expected patterns
2. Checks for consistency between GPU-related annotations and resource specifications
3. Ensures that fractional GPU requests are valid and well-formed

This plugin demonstrates validation logic that prevents invalid configurations from causing binding failures later in the process.

### Creating Custom Plugins

To create a custom binder plugin:

1. Implement the Plugin interface
2. Register your plugin with the binder's plugin registry
3. Ensure your plugin handles errors gracefully and provides clear error messages

Custom plugins can address specialized use cases such as:
- Network configuration and policy enforcement
- Custom resource binding and setup
- Integration with external systems
- Advanced validation and mutation based on organizational policies