# Elastic Workloads
Elastic workloads specify minimum (gang threshold) and maximum pod counts. If the number of running pods falls below the minimum threshold, the entire workload is evicted. 
KAI Scheduler intelligently manages pod roles—prioritizing eviction of non-leader pods when possible.

#### Prerequisites
This requires the [training-operator](https://github.com/kubeflow/trainer) to be installed in the cluster.

### Elastic Pytorch
To submit an elastic pytorch job, run this command:
```
kubectl apply -f pytorch-elastic.yaml
```
It will create a PytorchJob with a minimum of 1 worker, and will be able to start running as soon as there are enough resource in the cluster for the one pod.
And, if additional resources are available, the workload will be able to add 2 additional workers.
If resources are requested by more prioritized workload, KAI Scheduler will be able to evict only part of its pods and the workload will continue running.
