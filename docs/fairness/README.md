# Fairness

KAI Scheduler implements hierarchical fair-share scheduling using multi-level queues to distribute cluster resources equitably across users and projects.

> **Prerequisites**: Familiarity with [Scheduling Queues](../queues/README.md) concepts

## Table of Contents
- [Resource Allocation](#resource-allocation)
- [Fair Share Calculation](#fair-share-calculation)
- [Reclaim Strategies](#reclaim-strategies)
- [Configuration](#configuration)

## Resource Allocation

> Resource Allocation is done on each scheduling cycle

Resources are allocated hierarchically across queue levels:

1. **Quota allocation**: Guaranteed resources distributed first
2. **Over-quota distribution**: Remaining resources allocated by priority and weight
3. **Hierarchical propagation**: Process repeated at each queue level

## Fair Share Calculation

Fair share determines queue scheduling priority and reclaim eligibility:

- **Scheduling Priority**: Queues below fair share are prioritized
- **Saturation Ratio**: `Allocated / FairShare` used for reclaim decisions
- **Reclaim Eligibility**: Queues can only reclaim if their saturation ratio remains lowest among siblings

## Reclaim Strategies
KAI scheduler uses two main reclaim strategies:
1. **Fair Share Reclaim** - Workloads from queues with resources below their fair share can evict workloads from queues that have exceeded their fair share.
2. **Quota Reclaim** - Workloads from queues under their quota can evict workloads from queues that have exceeded their quota.

In both strategies, the scheduler ensures that the **relative ordering is preserved**: a queue that had the lowest utilisation ratio in its level before reclamation will still have the lowest ratio afterwards. Likewise, a queue that was below its quota will remain below its quota.
The scheduler will prioritize the first strategy.

> **Priority**: The scheduler prioritize Fair-Share reclaim over Quota reclaim

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
