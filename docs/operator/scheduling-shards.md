# Scheduling Shards

Scheduling shards partition a cluster into logical groups, each with a dedicated scheduler instance that operates on a subset of nodes, queues, and pod groups.

## How It Works

Each shard creates a separate KAI scheduler deployment that filters resources by the `partitionLabelValue`. The scheduler only considers resources with matching labels:

- **Nodes**: `<nodePoolLabelKey>=<partitionLabelValue>`
- **Queues**: Same label structure
- **Pod Groups**: Same label structure

The `nodePoolLabelKey` is configurable in the global configuration (default: `kai.scheduler/node-pool`). For example, with the default configuration, nodes are labeled as `kai.scheduler/node-pool=gpu-nodes`.

Shards operate independently with their own configuration for placement strategies, queue depths, and runtime requirements.

## Creating Scheduling Shards

### Basic Shard Default Configuration

This shard will define the partition of all nodes that are not labeled by the partition label:

```yaml
apiVersion: kai.scheduler/v1
kind: SchedulingShard
metadata:
  name: default
```

### Advanced Shard Configuration

```yaml
apiVersion: kai.scheduler/v1
kind: SchedulingShard
metadata:
  name: gpu-shard
spec:
  partitionLabelValue: gpu-nodes
  
  # Custom scheduler arguments
  args:
    leader-elect: "true"
    log-level: "3"
  
  # Placement strategy
  placementStrategy:
    gpu: binpack
    cpu: binpack
  
  # Queue depth configuration
  queueDepthPerAction:
    preempt: 15
    reclaim: 8
    allocate: 25
  
  # Minimum runtime requirements
  minRuntime:
    preemptMinRuntime: "10m"
    reclaimMinRuntime: "5m"
```

## Node Preparation

### Labeling Nodes

To use the scheduling shards, label your nodes appropriately:

```bash
# Label GPU nodes
kubectl label nodes node-1 kai.scheduler/node-pool=gpu-nodes
kubectl label nodes node-2 kai.scheduler/node-pool=gpu-nodes

# Label CPU nodes
kubectl label nodes node-3 kai.scheduler/node-pool=cpu-nodes
kubectl label nodes node-4 kai.scheduler/node-pool=cpu-nodes

# Label high-memory nodes
kubectl label nodes node-5 kai.scheduler/node-pool=high-memory-nodes
```

## Queue Configuration

### Shard-Specific Queues

Create queues that target specific shards:

```yaml
apiVersion: kai.scheduler/v1
kind: Queue
metadata:
  name: gpu-queue
  labels:
    kai.scheduler/node-pool: gpu-nodes  # Targets GPU shard
spec:
  priority: 100
  resourceQuota:
    gpu: 10
    cpu: 100
    memory: 500Gi
```

```yaml
apiVersion: kai.scheduler/v1
kind: Queue
metadata:
  name: cpu-queue
  labels:
    kai.scheduler/node-pool: cpu-nodes  # Targets CPU shard
spec:
  priority: 50
  resourceQuota:
    cpu: 200
    memory: 1Ti
```

## Pod Group Configuration

### Shard-Specific Pod Groups

Pod groups will automatically inherit shard targeting from their queue:

```yaml
apiVersion: kai.scheduler/v1
kind: PodGroup
metadata:
  name: gpu-training-job
  labels:
    kai.scheduler/queue: gpu-queue  # Inherits shard from queue
spec:
  minMember: 1
  priority: 100
  resourceQuota:
    gpu: 2
    cpu: 8
    memory: 32Gi
```

### Direct Shard Targeting

Pod groups can also directly target shards:

```yaml
apiVersion: kai.scheduler/v1
kind: PodGroup
metadata:
  name: memory-intensive-job
  labels:
    kai.scheduler/node-pool: high-memory-nodes
spec:
  minMember: 1
  priority: 75
  resourceQuota:
    cpu: 16
    memory: 128Gi
```

## Monitoring and Observability

### Shard Status

Monitor shard status and health:

```bash
# Check shard status
kubectl get schedulingshard
kubectl describe schedulingshard gpu-shard

# Check shard deployments
kubectl get deployments -n kai-system -l kai.scheduler/shard=gpu-shard
```

### Shard Logs

View logs for specific shards:

```bash
# View shard scheduler logs
kubectl logs -n kai-system deployment/kai-scheduler-gpu-shard
```