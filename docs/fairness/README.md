# Fairness
KAI Scheduler utilizes hierarchical scheduling queues, where a leaf queue can represent an individual researcher or a group of researchers working on the same project. 
Additional levels of hierarchy can be added to group multiple scheduling queues together, enabling the application of resource distribution rules to these groups.

Before proceeding, make sure you are familiar with the concepts of [Scheduling Queues](../queues/README.md)

## Resource Division Algorithm
At the start of each scheduling cycle, the resources in the cluster are allocated across the various scheduling queues. 
First, the total available resources are distributed among the top-level queues, with each receiving a fair share. 
Then, the fair share of each top-level queue is further divided among its direct child queues. 

This process is carried out in the following order:
1. Quota resources are allocated to all queues.
2. If there are remaining resources, the queues are sorted by priority. Within each priority group, additional resources are distributed based on the over-quota weight assigned to each queue.

These two steps are repeated across all hierarchy levels until every leaf queue receives its fair share. Queues that have already received their full requested resources will not be allocated any further resources.

## Fair Share
Once the fair share for each queue is calculated, it serves two primary purposes:
1. Queue Order - Queues with a fair share further below their allocation will be prioritized for scheduling.
2. Reclaim action – When reclamation is required, the scheduler compares the **Saturation Ratio** (`Allocated / FairShare`) of queues that share the same parent. A queue can only reclaim resources if, **after** the transfer, its utilisation ratio remains lower than that of every sibling queue. For more details see the reclaim strategies.

## Reclaim Strategies
There are two main reclaim strategies:
1. Workloads from queues with resources below their fair share can evict workloads from queues that have exceeded their fair share.
2. Workloads from queues under their quota can evict workloads from queues that have exceeded their quota.

In both strategies, the scheduler ensures that the relative ordering is preserved: a queue that had the lowest utilisation ratio in its level before reclamation will still have the lowest ratio afterwards. Likewise, a queue that was below its quota will remain below its quota.
The scheduler will prioritize the first strategy.

### Reclaim Ratio Adjustment
The Saturation Ratio comparison can be adjusted using the `reclaimerUtilizationMultiplier` plugin argument. This multiplier is applied to the reclaimer's Saturation Ratio before comparison:
- Values > 1.0 make it harder for jobs to reclaim resources (more conservative)
- Minimum value is 1.0 (standard comparison, default)
- Values < 1.0 are not allowed and will be set to 1.0 - These values could cause infinite reclaim cycles that we want to avoid.

Example configuration:
```yaml
pluginArguments:
  proportion:
    reclaimerUtilizationMultiplier: "1.2"  # Makes reclamation 20% more conservative
```
