# Quick Start Setup

## Scheduling queues
A queue is an object which represents a job queue in the cluster. Queues are an essential scheduling primitive, and can reflect different scheduling guarantees, such as resource quota and priority. 
Queues are typically assigned to different consumers in the cluster (users, groups, or initiatives). A workload must belong to a queue in order to be scheduled.
KAI Scheduler operates with two levels of hierarchical scheduling queue system.

This command sets up two scheduling queue hierarchies:
* `default` – A top-level queue that governs resource division of other leaf queues.
* `test` – A leaf queue under the default top-level queue. Workloads should reference this queue.

For this example, these queues do not have resource limits, meaning they can consume cluster resources freely.
```
kubectl apply -f queues.yaml
```
Pods can now be assigned to the `test` queue and submitted to the cluster for scheduling.

### Assigning Pods to Queues
To schedule a pod using KAI Scheduler, ensure the following:
1. Specify the queue name using the `kai.scheduler/queue: test` label on the pod/workload.
2. Set the scheduler name in the pod specification as `kai-scheduler`
This ensures the pod is placed in the correct scheduling queue and managed by KAI Scheduler.

### ⚠️ Workload namespaces
When submitting workloads, make sure to use a dedicated namespace. Do not use the `kai-scheduler` namespace for workload submission.

### Submitting Example Pods
#### CPU-Only Pods
To submit a very simple pod that requests CPU and memory resources, use the following command:
```
kubectl apply -f pods/cpu-only-pod.yaml
```

#### GPU Pods
Before you run the below, make sure the [NVIDIA GPU-Operator](https://github.com/NVIDIA/gpu-operator) is installed in the cluster.

To submit a pod that requests a GPU resource, use the following command:
```
kubectl apply -f pods/gpu-pod.yaml
```
