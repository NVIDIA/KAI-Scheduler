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

> Note: this is not finalized and is expected to change in an upcoming KAI release

### Enabling prometheus

To enable prometheus via kai-operator, apply the following patch:
```sh
kubectl patch config kai-scheduler --type merge -p '{"spec":{"prometheus":{"enabled":true}}}'
```

You can also customize the following configurations:

```
  externalPrometheusHealthProbe	<Object>
    ExternalPrometheusPingConfig defines the configuration for external
    Prometheus connectivity validation, with defaults.

  externalPrometheusUrl	<string>
    ExternalPrometheusUrl defines the URL of an external Prometheus instance to
    use
    When set, KAI will not deploy its own Prometheus but will configure
    ServiceMonitors
    for the external instance and validate connectivity

  retentionPeriod	<string>
    RetentionPeriod defines how long to retain data (e.g., "2w", "1d", "30d")

  sampleInterval	<string>
    SampleInterval defines the interval of sampling (e.g., "1m", "30s", "5m")

  serviceMonitor	<Object>
    ServiceMonitor defines ServiceMonitor configuration for KAI services

  storageClassName	<string>
    StorageClassName defines the name of the storageClass that will be used to
    store the TSDB data. defaults to "standard".

  storageSize	<string>
    StorageSize defines the size of the storage (e.g., "20Gi", "30Gi")
```

Alternatively, you can use your own prometheus. Make sure that it's configured to collect metrics from the queue controller via a service monitor. For example:

```yaml
apiVersion: monitoring.coreos.com/v1
kind: ServiceMonitor
metadata:
  labels:
    app: queuecontroller
  name: queuecontroller
  namespace: kai-scheduler
spec:
  endpoints:
  - bearerTokenFile: /var/run/secrets/kubernetes.io/serviceaccount/token
    interval: 30s
    port: metrics
    scrapeTimeout: 10s
  jobLabel: queuecontroller
  namespaceSelector:
    matchNames:
    - kai-scheduler
  selector:
    matchLabels:
      app: queuecontroller
```

### Usage Database

To configure the scheduler to connect to prometheus, the usageDBConfig section of the scheduling shard needs to be edited:
```sh
kubectl edit schedulingshard default
```
*Replace `default` with the shard name if relevant*

Add the following section under `spec`:
```yaml
  usageDBConfig:
    clientType: prometheus
    connectionString: http://prometheus-operated.kai-scheduler.svc.cluster.local:9090
    usageParams:
      halfLifePeriod: 10m
      windowSize: 10m
      windowType: sliding
```
*This configuration assumes using the kai operated prometheus. Change connectionString if relevant.*

Configure windowSize and halfLifePeriod to desired values.

### External prometheus

You can configure kai-scheduler to connect to any external DB that's compatible with the prometheus API - simply edit the connectionString accordingly. Note that it has to be accessible from the scheduler pod, and have access to queue controller and kube-state metrics.

### kValue

KValue is a parameter used by the proportion plugin to determine the significance of historical usage in fairness calculations - higher values mean more aggressive effects on fairness. To set it, add it to the scheduling shard spec:
```sh
kubectl edit schedulingshard default
```

```yaml
spec:
  kValue: 0.5
```

### Advanced: overriding metrics

> *This configuration should not be changed under normal conditions*

In some cases, the admin might want to configure the scheduler to query different metrics for usage and capacity of certain resources. This can be done with the following config:

```sh
kubectl edit schedulingshard default
```

```yaml
  usageDBConfig:
    extraParams:
      gpuAllocationMetric: kai_queue_allocated_gpus
      cpuAllocationMetric: kai_queue_allocated_cpu_cores
      memoryAllocationMetric: kai_queue_allocated_memory_bytes
      gpuCapacityMetric: sum(kube_node_status_capacity{resource=\"nvidia_com_gpu\"})
      cpuCapacityMetric: sum(kube_node_status_capacity{resource=\"cpu\"})
      memoryCapacityMetric: sum(kube_node_status_capacity{resource=\"memory\"})
```

## Troubleshooting

Prometheus connectivity
Metrics availability
