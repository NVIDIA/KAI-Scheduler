# Cluster Autoscaling
In Kubernetes, cluster autoscalers automatically adjust the size of a node pool in response to resource demands from running pods.
The autoscaler monitors for unschedulable pods - those that can't be placed on any current node due to insufficient resources.  
When such pods are detected and no existing nodes can host them, the autoscaler prompts the cloud provider to provision new nodes.

KAI Scheduler natively supports Kubernetes node autoscalers. However, when using GPU Sharing, GPU requests are specified in pod annotations rather than the pod specification. 
This makes them invisible to the cluster autoscaler. To address this, KAI Scheduler provides a component called node-scale-adjuster, which tracks unschedulable pods that use GPU sharing. 
When such a pod is found, `node-scale-adjuster` launches a temporary utility pod that requests full GPUs. This mimics the original pod's constraints and triggers the autoscaler to take appropriate scaling action.

## Prerequisites
GPU sharing autoscaling is disabled by default. To enable it, add the following flag to the helm install command:
```
--set "global.clusterAutoscaling=true"
```

## Handling Multiple pods
The `node-scale-adjuster` sums up the GPU fractions requested by all unschedulable pods to determine how many utility pods to launch.
For example, if there are two pods each requesting 0.5 GPU, only one utility pod will be created, requesting a full GPU.
If the autoscaler later provisions a node that can host only one of the GPU-sharing pods, an additional utility pod will be deployed to prompt further scaling.


### GPU Memory Considerations
When GPU memory is specified instead of fractions, the number of utility pods created depends on the GPU memory of the new node - information not known in advance.
To handle this, `node-scale-adjuster` assumes a default value of 0.1 GPU per pod when calculating memory-based requests.
This means one utility pod is created for every 10 GPU memory requesting pods.
You can adjust this behavior by changing the `--gpu-memory-to-fraction-ratio` flag in the `node-scale-adjuster` deployment.

### Grace Period Configuration
By default, `node-scale-adjuster` immediately creates scaling pods when unschedulable GPU-sharing pods are detected.
In some scenarios, this can race with the default Kubernetes scheduler, causing unnecessary node provisioning when the scheduler was about to place the pod.

To prevent this, you can configure a grace period using the `--unschedulable-grace-period` flag (or `unschedulableGracePeriod` in the KAI Config).
This setting delays the creation of scaling pods until a pod has been unschedulable for the specified duration (in seconds).

For example, setting `--unschedulable-grace-period=30` means scaling pods will only be created for pods that have been unschedulable for at least 30 seconds.
The default value is `0`, i.e. scale immediately when an unschedulable pod is found.

More details on supported arguments can be found [here](../../../cmd/nodescaleadjuster/app/options.go)
