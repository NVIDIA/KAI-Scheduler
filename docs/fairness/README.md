# Fairness

KAI Scheduler implements hierarchical fair-share scheduling using multi-level queues to distribute cluster resources equitably across users and projects.

> **Prerequisites**: Familiarity with [Scheduling Queues](../queues/README.md) concepts

### Fair Share Simulator - coming soon

## Table of Contents
- [Resource Allocation](#resource-allocation)
- [Fair Share Calculation](#fair-share-calculation)
- [Reclaim Strategies](#reclaim-strategies)
- [Configuration](#configuration)

## Resource Allocation

> Resource Allocation is done on each scheduling cycle

### Fair-Share Algorithm

Resource allocation and fair-share calculation is done on each scheduling cycle.

1. **Top-level distribution**: Total available resources are disributed to top-level queues according with respect to **deserved** quota.
    * **Hierarchical division**: Each queue's resources are further distributed among its child queues
    * **Recursive allocation**: The process continues down the queue hierarchy until all levels are allocated
2. **Over-quota share**: if resources remain unallocated after all deserved quotas are distributed, a fair share of the over quota part is calculated based on **priority** and **weight** attributes
3. **Job scheduling**: The scheduler schedules jobs from each queue to utilize their allocated resources, aiming to keep actual usage as close to the fair share as possible  

> Deserved resouces calculation:
```python
remainingResources = totalResources
for q in queues:
    q.fairShare += min(q.deserved, requested)
    remainingResources -= min(q.deserved, requested)
```
> Fair share calculation:
```python
while remainingResources > 0:
    totalWeights = sum(q.OverQuotaWeight for q in queues)
    for q in queues:
        resourcesToAssign = min(remainingResources * q.OverQuotaWeight / totalWeights, q.RemainingRequest)
        q.fairShare += resourcesToAssign
        remainingResources -= resourcesToAssign
```


## Reclaim Resources
Fair share determines queue scheduling priority and reclaim eligibility:

- **Scheduling Priority**: Queues below fair share are prioritized for receiving more resources
- **Reclaim Eligibility**: Queues can be reclaimed if they are allocated more resources than their fair share
- **Saturation Ratio**: `Allocated / FairShare` used for reclaim decisions. Higher ratio == higher probability of reclaim

KAI scheduler uses two main reclaim strategies:
1. **Fair Share Reclaim** - Workloads from queues with resources below their fair share can evict workloads from queues that have exceeded their fair share.
2. **Quota Reclaim** - Workloads from queues under their quota can evict workloads from queues that have exceeded their quota.

In both strategies, the scheduler ensures that the **relative ordering is preserved**: a queue that had the lowest Saturation ratio in its level before reclamation will still have the lowest ratio afterwards. Likewise, a queue that was below its quota will remain below its quota.
The scheduler will prioritize the first strategy.
> **Note:** because of the hierarchical nature & priority/weight parametes of job queues in KAI, there are scenarios that a queue will have lower resources allocated than its siblings, yet it'll receive no additional resources via reclaim.

## Configuration

### Reclaim Sensitivity
Adjust reclaim aggressiveness using `reclaimerUtilizationMultiplier`:

```yaml
pluginArguments:
  proportion:
    reclaimerUtilizationMultiplier: "1.2"  # 20% more conservative
```

| Value | Behavior |
|-------|----------|
| `1.0` | Standard comparison (default) |
| `> 1.0` | More conservative reclaim |
| `< 1.0` | Not allowed (prevents infinite cycles) |
