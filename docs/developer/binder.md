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
