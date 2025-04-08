# Scheduling Queues
A queue is an object which represents a job queue in the cluster. Queues are an essential scheduling primitive, and can reflect different scheduling guarantees, such as resource quota and priority.
KAI Scheduler offers several features that make it easier to optimize resource distribution among researchers in a way that maximizes resource fairness between queues.

The following attributes can be applied to any scheduling queue for CPU, memory and GPU resources:
* Quota: The designated amount of resources allocated to the scheduling queue upon request.
* Over-Quota Priority: Determines the order in which queues will receive resources that exceed their allocated quota.
* Over-Quota Weight: Dictates how excess resources are distributed among queues within the same priority level.
* Limit: The hard cap on the amount of resources a scheduling queue can consume.

## Queue Specification
```
spec:
  displayName: string
  parentQueue: string
  priority: integer
  resources: QueueResources
```

### Display name (Optional)
The `displayName` field is used solely for logging purposes. It represents the human-readable name of the queue that appears in scheduler-generated events for pods.

### Parent Queue (Optional)
The `parentQueue` field defines a hierarchical structure for queues, enabling multi-level resource allocation within the cluster. This allows for more granular and complex resource distribution.

### Priority (Optional)
The `priority` field determines the queue's precedence when allocating unused resources. Queues with higher priority values receive resources first. Only after fulfilling all higher-priority queues will the scheduler allocate remaining resources to lower-priority queues.

## Queue Resources
```
cpu: ResourceQuota
memory: ResourceQuota
gpu: ResourceQuota
```

### CPU
The `cpu` field defines the quota policy for CPU resources allocated to the queue. CPU limits are measured in millicores (1 CPU = 1000 millicores).

### Memory
The `memory` field specifies the quota policy for memory allocation within the queue. Memory is measured in megabytes (MB), where 1 MB = 10⁶ bytes.

### GPU
The `gpu` field sets the quota policy for GPU resources assigned to the queue. GPU usage is measured as units, where 1 represents a full GPU device.

## Resource Quota
```
quota: integer
overQuotaWeight: integer
limit: integer
```

### Quota (Optional)
The quota field defines the guaranteed resource allocation for the queue. Jobs within a queue that stays within its quota are protected from being reclaimed.

For unlimited quota set the value to -1

When not set, the default value is 0

### Over Quota Weight (Optional)
The `overQuotaWeight` field determines the queue’s relative weight when distributing unused resources among queues. This ensures fair allocation based on priority.

### Limit (Optional)
The `limit` field sets a strict upper boundary on resource usage. The scheduler will not assign additional workloads to the queue once this limit is reached.

For disabling the limit set the value to -1

When not set, the default value is 0
