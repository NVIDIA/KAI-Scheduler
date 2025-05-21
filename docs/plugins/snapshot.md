# KAI Scheduler Snapshot Plugin and Tool

## Overview

The KAI Scheduler provides a snapshot plugin and tool that allows capturing and analyzing the state of the scheduler and cluster resources. This documentation covers both the snapshot plugin and the snapshot tool.

## Snapshot Plugin

The snapshot plugin is a framework plugin that provides an HTTP endpoint to capture the current state of the scheduler and cluster resources.

### Features

- Captures scheduler configuration and parameters
- Collects raw Kubernetes objects that the scheduler uses to perform its actions including:
  - Pods
  - Nodes
  - Queues
  - PodGroups
  - BindRequests
  - PodDisruptionBudgets
  - PriorityClasses
  - ConfigMaps
  - PersistentVolumeClaims
  - CSIStorageCapacities
  - StorageClasses
  - CSIDrivers
  - ResourceClaims
  - ResourceSlices
  - DeviceClasses

### Usage

The plugin registers an HTTP endpoint `/get-snapshot` that returns a ZIP file containing a JSON snapshot of the cluster state.
Example for the scheduler pod that is deployed in `kai` namespace:
```bash
kubectl port-forward -n kai deployment/scheduler 8081 &
curl -vv "localhost:8081/get-snapshot"  > snapshot.gzip
./bin/snapshot-tool-amd64 --filename snapshot.gzip --verbosity 8
```

### Response Format

The snapshot is returned as a ZIP file containing a single JSON file (`snapshot.json`) with the following structure:

```json
{
  "config": {
    // Scheduler configuration
  },
  "schedulerParams": {
    // Scheduler parameters
  },
  "rawObjects": {
    // Raw Kubernetes objects
  }
}
```

## Snapshot Tool

The snapshot tool is a command-line utility that can load and analyze snapshots captured by the snapshot plugin.

### Features

- Loads snapshots from ZIP files
- Recreates the scheduler environment from a snapshot
- Supports running scheduler actions on the snapshot data
- Provides detailed logging of operations

### Usage

```bash
snapshot-tool --filename <snapshot-file> [--verbosity <log-level>]
```

#### Arguments

- `--filename`: Path to the snapshot ZIP file (required)
- `--verbosity`: Logging verbosity level (default: 4)

### Example

```bash
# Load and analyze a snapshot
snapshot-tool --filename snapshot.zip

# Load and analyze a snapshot with increased verbosity
snapshot-tool --filename snapshot.zip --verbosity 5
```

## Implementation Details

### Snapshot Plugin

The snapshot plugin (`pkg/scheduler/plugins/snapshot/snapshot.go`) implements the following key components:

1. `RawKubernetesObjects`: Structure containing all captured Kubernetes objects
2. `Snapshot`: Main structure containing configuration, parameters, and raw objects
3. `snapshotPlugin`: Plugin implementation with HTTP endpoint handler

### Snapshot Tool

The snapshot tool (`cmd/snapshot-tool/main.go`) implements:

1. Snapshot loading and parsing
2. Fake client creation with snapshot data
3. Scheduler cache initialization
4. Session management
5. Action execution

## Limitations

- The snapshot tool runs in a simulated environment
- Some real-time cluster features may not be available
- Resource constraints may differ from the original cluster 
