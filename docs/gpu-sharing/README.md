# GPU Sharing
KAI Scheduler supports GPU sharing, allowing multiple pods to utilize the same GPU device efficiently by allocating a GPU device to multiple pods.

There are several ways for users to request a portion of GPU for their pods:
* Pod can request a specific GPU memory amount (e.g. 2000Mib), leaving the remaining GPU memory for other pods.
* Or, it can request a portion of a GPU device memory (e.g. 0.5) that the pod intends to consume from the mounted GPU device.

KAI Scheduler does not enforce memory allocation limit or performs memory isolation between processes.
In order to make sure the pods share the GPU device nicely it is important that the running processes will allocate GPU memory up to the requested amount and not beyond that.
In addition, note that pods sharing a single GPU device can reside in different namespaces.

In order to reserve a GPU device, KAI Scheduler will run a reservation pod in `kai-resource-reservation` namespace.


### Prerequisites
GPU sharing is disabled by default. To enable it, add the following flag to the helm install command:
```
--set "global.gpuSharing=true"
```

### RuntimeClass Requirement
KAI Scheduler requires the use of a specific RuntimeClass for GPU sharing. The recommended RuntimeClass is `nvidia`.

KAI explicitly sets `runtimeClassName: "nvidia"` in the resource reservation pod spec. Ensure that your cluster has the `nvidia` RuntimeClass configured — this is typically provided by the NVIDIA device plugin.

### GPU Sharing Pod
To submit a pod that can share a GPU device, run this command:
```
kubectl apply -f gpu-sharing.yaml
```

In the gpu-sharing.yaml file, the pod includes a `gpu-fraction` annotation with a value of 0.5, meaning:
* The pod is allowed to consume up to half of a GPU device memory
* Other pods with total request of up to 0.5 GPU memory will be able to share this device as well


### GPU Memory Pod
To submit a pod that request a specific amount of GPU memory, run this command:
```
kubectl apply -f gpu-memory.yaml
```
In the gpu-memory.yaml file, the pod includes a `gpu-memory` annotation with a value of 2000 (in Mib), meaning:
* The pod is allowed to consume up to 2000 Mib of a GPU device memory
* The remaining GPU device memory can be shared with other pods in the cluster