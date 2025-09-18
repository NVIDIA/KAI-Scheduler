# Quick Start Setup

## Scheduling queues
A queue is an object which represents a job queue in the cluster. Queues are an essential scheduling primitive, and can reflect different scheduling guarantees, such as resource quota and priority. 
Queues are typically assigned to different consumers in the cluster (users, groups, or initiatives). A workload must belong to a queue in order to be scheduled.
KAI Scheduler operates with two levels of hierarchical scheduling queue system.

### Default Queue on Fresh Install

After installing KAI Scheduler, a **default unlimited queue** named `default` is automatically created. This queue has no reserved resource quotas, allowing you to schedule workloads immediately—no manual queue setup required.

This command sets up two scheduling queue hierarchies:
* `default` – Automatically created top-level unlimited queue governing resource distribution for leaf queues.
* `test` – A leaf queue under the default top-level queue. Workloads should reference this queue.

You can start submitting jobs to the `default` queue right away, making onboarding fast and simple. To customize scheduling, you can create additional queues or modify existing ones to set quotas, priorities, and hierarchies.

### Creating Additional Queues

To add custom queues (e.g., a `test` queue), apply your queue configuration:
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
