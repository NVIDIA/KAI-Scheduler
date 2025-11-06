# Time Aware fairness

Time aware fairness is a feature in KAI-Scheduler which makes use of historical resource usage by queues for making allocation and reclaim decisions. Key features are:

1. Consider past usage for order of allocation: all else being equal, queues with higher past usage will get to run jobs after queues with lower usage
2. Reclaim based on usage: queues which are starved over time will reclaim resources from queues which used a lot of resources.
    1. Note: this does not effect in-quota allocation: deserved quota still takes precedence over time-aware fairness


> **Prerequisites**: Familiarity with [fairness](../fairness/README.md)

## How it works

In high level: resource usage data in the cluster is collected and persisted in prometheus. It is then used by the scheduler to make resource fairness calculations: the more resources consumed by a queue, the less over-quota resources it will get compared to other queues. This will eventually result in the queues' over-quota resources being reclaimed by more starved queues, thus achieving a more fair allocation of resources over time.

### Resource usage data

Queue historical resource usage data is collected in a prometheus instance in the cluster *(external prometheus instances can be used - see [External prometheus](#external-prometheus))*. The scheduler configuration determines the time period that will be considered, as well as allows configuration for time-decay, which, if configured, gives more weight to recent usage than past usage.

The metrics are collected continuously: the pod-group-controller publishes resource usage for individual pod-groups on their status, which are then aggregated by the queue-controller and published as a metric, which gets collected and persisted by prometheus.

If configured, the scheduler applies an [exponential time decay](https://en.wikipedia.org/wiki/Exponential_decay) formula which is configured by a half-life period. This can be more intuitively understood with an example: for a half life of one hour, a usage (for example, 1 gpu-second) that occurred an hour ago will be considered half as significant as a gpu-second that was consumed just now.

The aggregated usage for each queue is then normalized to the **cluster capacity** at the relevant time period: the scheduler looks at the available resources in the cluster for that time period, and normalizes all resource usage to it. For example, in a cluster with 10 GPUs, and considering a time period of 10 hours, a queue which consumed 24 GPU hours (wether it's 8 GPUs for 3 hours, or 12 GPUs for 2 hours), will get a normalized usage score of 0.24 (used 24 GPU hours out of a potential 100). This normalization ensures that a small amount of resource usage in a vacant cluster will not result in a heavy penalty.

### Effect on fair share

Usually, over quota resources is divided to each queue proportionally to it's Over Quota Weight. With time-aware fairness, queues with historical usage will get relatively less resources in over-quota. The significance of the resource usage in this calculation can be controlled with a parameter called "kValue": the bigger it is, the more significant the historical usage be.

Check out the [time aware simulator](../../cmd/time-aware-simulator/README.md) to understand scheduling behavior over time better.

### Example

The following plot demonstrates the GPU allocation over time in a 16 GPU cluster, with two queues, each having 0 deserved quota and 1 Over Quota weight for GPUs, each trying to run 16-GPU, single-pod Jobs.

![Time-aware fairness GPU allocation over time](./results.png)

*Time units are intentionally omitted*

## Configuration

### Usage Database

The scheduler can be configured 

### External prometheus

## Troubleshooting

Prometheus connectivity
Metrics availability
