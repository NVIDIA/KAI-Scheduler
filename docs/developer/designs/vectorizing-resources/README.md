<!--
Copyright 2025 NVIDIA CORPORATION
SPDX-License-Identifier: Apache-2.0
-->

# Vectorizing Resource Representation

## Overview

This document describes the design for converting KAI Scheduler's resource representation from discrete struct-based types to vector-based representations. The vectorization enables efficient bulk operations on resource data, facilitating faster scenario feasibility checks in the scheduler's allocation and reclaim logic.

The goal is to improve scheduler performance at scale (2000+ nodes) by accelerating the scenario filtering phase of the reclaim algorithm through vectorized resource comparisons and multi-resource bin-packing heuristics.

## Motivation

Current scheduler performance degrades significantly with cluster scale:

- **Scale test performance**: Full scheduling cycles take 3-4 minutes for 1000 nodes, 20+ minutes for 1000+ nodes
- **Bottleneck**: Node ordering functions dominate during allocation simulations (filtering scenarios)
- **Root cause**: Resource comparisons iterate over individual nodes and resources in sequence; no bulk operations

With topology-aware scheduling, time-aware scheduling, and large reclaim scenarios becoming more common, the scheduler will face increasingly complex allocation decisions. Vectorizing resources allows:

1. **Vectorized comparisons**: Compare resources for multiple nodes simultaneously
2. **Efficient bin-packing**: Use normalized resource metrics (sum or DRF) for node sorting heuristics
3. **Scenario filtering acceleration**: Pre-compute vector representations to enable quick feasibility checks

## Goals / Non-Goals

### Goals

- Design a vector representation for resources that maintains semantic equivalence with current Resource and ResourceRequirements types
- Enable efficient resource comparisons and arithmetic operations (add, subtract, less-than-or-equal)
- Support multi-resource scenarios (CPU, memory, GPUs, custom resources, MIG profiles)
- Provide clear migration path from struct-based to vector-based representations
- Maintain backward compatibility during transition

### Non-Goals

- Implement resource vectors in this commit (deferred to commit 3)
- Redesign the scenario filtering algorithm itself (only optimize existing heuristics)
- Change the dominant-resource-fairness (DRF) algorithm for fairness calculations
- Implement concurrent/parallel scenario filtering (prerequisite for future work)

## Current Implementation

### Resource Type

The current resource representation uses a struct with discrete fields:

```go
// pkg/scheduler/api/resource_info/resource_info.go
type Resource struct {
    BaseResource
    gpus float64
}

// pkg/scheduler/api/resource_info/base_resources.go
type BaseResource struct {
    milliCpu        float64
    memory          float64
    scalarResources map[v1.ResourceName]int64
}
```

**Key operations**:
- `Resource.LessEqual(other *Resource) bool` - Check if resource requirements can fit
- `Resource.Add(other *Resource)` - Aggregate resources
- `Resource.Sub(other *Resource)` - Remove allocated resources
- `Resource.Get(resourceName) float64` - Retrieve specific resource value

### ResourceRequirements Type

Pod and job resource requirements use a similar structure:

```go
// pkg/scheduler/api/resource_info/resource_requirment.go
type ResourceRequirements struct {
    BaseResource
    GpuResourceRequirement
}

// GpuResourceRequirement supports both whole and fractional GPU allocation
type GpuResourceRequirement struct {
    count   float64 // Number of whole GPUs
    portion float64 // Fractional GPU portion
}
```

**Limitations**:
1. **Struct overhead**: Each resource allocation carries full struct overhead
2. **Sequential comparisons**: LessEqual iterates field-by-field
3. **Map overhead**: scalarResources map has lookup/iteration overhead
4. **No bulk operations**: Cannot compare multiple resources in a vectorized manner

### Current Usage Locations

The `Resource` and `ResourceRequirements` structs are used throughout the scheduler codebase. This section documents all locations that will need to be migrated to vector-based representations.

#### Core API Struct Fields

| File | Field | Type |
|------|-------|------|
| `api/node_info/node_info.go` | `Releasing`, `Idle`, `Used`, `Allocatable` | `*resource_info.Resource` |
| `api/podgroup_info/job_info.go` | `Allocated` | `*resource_info.Resource` |
| `api/podgroup_info/job_info.go` | `tasksToAllocateInitResource` | `*resource_info.Resource` |
| `api/pod_info/pod_info.go` | `ResReq`, `AcceptedResource` | `*resource_info.ResourceRequirements` |
| `plugins/topology/topology_structs.go` | `IdleOrReleasingResources` | `*resource_info.Resource` |
| `plugins/proportion/reclaimable/reclaimer_info.go` | `RequiredResources` | `*resource_info.Resource` |
| `k8s_internal/predicates/maxNodeResources.go` | `maxResources` | `*resource_info.Resource` |

#### NodeInfo Resource Field Accessors

Methods and code paths that access NodeInfo's `Idle`, `Used`, `Releasing`, `Allocatable` fields:

| File | Location | Usage |
|------|----------|-------|
| `framework/statement.go` | lines 252, 309 | Logging node resource state |
| `api/node_info/node_info.go` | `NonAllocatedResources()` | Returns `*resource_info.Resource` |
| `api/node_info/node_info.go` | `isTaskAllocatableOnNonAllocatedResources()` | Resource comparison |
| `api/node_info/node_info.go` | `FittingError()` | Error message generation |
| `api/node_info/gpu_sharing_node_info.go` | `getAcceptedTaskResourceWithoutSharedGPU()` | GPU sharing calculations |
| `cache/cluster_info/cluster_info_test.go` | Test assertions | Verifying `node.Idle`, `node.Used` |

#### PodGroupInfo Resource Field Accessors

| File | Location | Usage |
|------|----------|-------|
| `api/podgroup_info/job_info.go` | `GetTasksActiveAllocatedReqResource()` | Returns `*resource_info.Resource` |
| `api/podgroup_info/allocation_info.go` | `GetAllocatedResource()` | Returns `*resource_info.Resource` |

#### Plugins Using `*resource_info.Resource`

| Plugin | File | Functions/Usage |
|--------|------|-----------------|
| **proportion** | `proportion.go` | `totalVictimsResources map`, `getVictimResources()`, `getResources()` |
| **proportion/reclaimable** | `reclaimable.go` | `reclaimeeResourcesByQueue`, `reclaimedResources`, `getInvolvedResourcesNames()` |
| **proportion/reclaimable/strategies** | `strategies.go` | `reclaimerResources` parameter, `reclaimerWillGoOverQuota()` |
| **proportion/utils** | `utils.go` | `QuantifyResource(resource *resource_info.Resource)` |
| **topology** | `job_filtering.go` | `getTasksAllocationMetadata()`, `calcSubTreeFreeResources()`, `sortTree()`, `getJobRatioToFreeResources()` |

#### Error Handling and Display

These locations use Resource structs to generate human-readable error messages:

| File | Function | Usage |
|------|----------|-------|
| `api/common_info/pod_errors.go` | `NewInsufficientNodeResourcesError()` | `usedResource, capacityResource *resource_info.Resource` |
| `api/common_info/job_errors.go` | `NewInsufficientClusterResourcesError()` | `resourceRequested, availableResource *resource_info.Resource` |

#### Test Utilities

| File | Functions |
|------|-----------|
| `test_utils/resources_fake/resources.go` | `BuildResource()` returns `*resource_info.Resource` |
| `test_utils/jobs_fake/jobs.go` | `BuildJobInfo()`, `generateTasks()`, `CalcJobAndPodResources()` |
| `api/common_info/test_utils.go` | `BuildResource()`, `BuildResourceWithGpu()` |
| `framework/statement_test_utils.go` | Test helper structs and functions |

#### Test Files with Resource Assertions

The following test files contain assertions or test data using `*resource_info.Resource`:

- `framework/statement_test.go` - Statement execution tests
- `api/node_info/node_info_test.go` - NodeInfo unit tests
- `cache/cluster_info/cluster_info_test.go` - Cluster snapshot tests
- `plugins/proportion/reclaimable/reclaimable_test.go` - Reclaimable plugin tests
- `plugins/proportion/reclaimable/strategies/strategies_test.go` - Strategy tests
- `plugins/topology/node_scoring_test.go` - Topology scoring tests
- `api/common_info/pod_errors_test.go` - Pod error message tests
- `api/common_info/job_errors_test.go` - Job error message tests

## Vector Representation Design

### Core Types

```go
// pkg/scheduler/api/resource_info/resource_vector.go

// resourceVector represents a single entity's resources as a fixed-length array
// All vectors use the same index mapping defined by resourceVectorMap
type resourceVector []float64

// resourceVectorMap maintains the mapping from indices to resource names
// This is created once during cluster info snapshot and reused throughout
type resourceVectorMap struct {
    indexToName  []v1.ResourceName
    nameToIndex  map[v1.ResourceName]int
}

// ResourceVectorContext holds the vector map for a scheduling session
type ResourceVectorContext struct {
    vectorMap resourceVectorMap
}
```

### Resource Vector Mapping Example

For a cluster with resources: CPU, Memory, GPUs, EFA, Custom resources:

```
resourceVectorMap:
  Index 0: v1.ResourceCPU       → milliCPU value
  Index 1: v1.ResourceMemory    → memory bytes
  Index 2: nvidia.com/gpu       → GPU count
  Index 3: example.com/efa      → EFA count
  Index 4: custom-resource      → custom value

Example Vector:
  Node capacity:  [64000, 256e9, 8, 4, 100]   (64 cores, 256GB memory, 8 GPUs, 4 EFA, 100 custom)
  Pod request:    [1000, 4e9, 0.5, 0, 0]     (1 core, 4GB memory, 0.5 GPU, 0 EFA, 0 custom)
```

### Vector Operations

```go
// Comparison: Check if request can fit in available capacity
// Equivalent to Resource.LessEqual(other)
func VectorLessEqual(request, available resourceVector) bool {
    for i := range request {
        if request[i] > available[i] {
            return false
        }
    }
    return true
}

// Addition: Aggregate resource allocations
// Equivalent to Resource.Add(other)
func VectorAdd(dst, src resourceVector) {
    for i := range src {
        dst[i] += src[i]
    }
}

// Subtraction: Remove allocated resources
// Equivalent to Resource.Sub(other)
func VectorSub(dst, src resourceVector) {
    for i := range src {
        dst[i] -= src[i]
    }
}

// Normalization metrics for sorting (used in scenario filtering)
// Normalized sum: sum(resource[i] / totalCapacity[i])
func NormalizedSum(vec, totalCapacity resourceVector) float64 {
    var sum float64
    for i := range vec {
        if totalCapacity[i] > 0 {
            sum += vec[i] / totalCapacity[i]
        }
    }
    return sum
}

// Dominant resource (max ratio): max(resource[i] / totalCapacity[i])
func DominantResource(vec, totalCapacity resourceVector) float64 {
    var max float64
    for i := range vec {
        if totalCapacity[i] > 0 {
            ratio := vec[i] / totalCapacity[i]
            if ratio > max {
                max = ratio
            }
        }
    }
    return max
}
```

### PodGroup and Node Representations

Hierarchical resource structures maintain vector form:

```go
// Pod group represented as hierarchical vector structure
type PodGroupAsVector struct {
    // For leaf pods: direct vector representation
    podVectors []resourceVector  // One vector per pod
    
    // For sub-groups: recursive structure
    subGroups []*PodGroupAsVector
}

// Cluster nodes in vector form
type ClusterNodesVector struct {
    vectorMap     resourceVectorMap
    nodeNames     []string
    nodeResources []resourceVector  // One vector per node
}
```

## Migration Plan

### Phase 1: Type Introduction
- Introduce `resourceVector`, `resourceVectorMap`, and `ResourceVectorContext` types
- Create conversion functions: `ResourceToVector()` and `VectorToResource()`
- Add vector operation helpers: `VectorAdd`, `VectorSub`, `VectorLessEqual`
- Create unit tests for vector operations

### Phase 2: Vector Map Generation
- Extend `ClusterInfoSnapshot` to build `resourceVectorMap` from cluster state
- Create `ResourceVectorContext` during session initialization
- Document vector map lifecycle and cache strategy

### Phase 3: Pod & Node Info Vectorization
- Vectorize pod and node resource representations in `PodInfo`, `NodeInfo`
- Use current Resource structs behind the scenes

### Phase 4: Resource Structs Deprecation and Removal
- Deprecate older Resource structs
- Remove all uses of Resource structs and implement vector resources instead

### Phase 5: Validation & Optimization
- Comprehensive performance testing at scale (100-2000 nodes)
- Final optimization passes
- Document performance improvements and trade-offs

## Baseline Performance

This section establishes baseline metrics for the current struct-based implementation. These metrics will be compared against vector-based implementation in commit 10 to quantify performance improvements.

### Test Environment

- **System**: Intel Core Ultra 7 165H
- **CPU Governor**: performance
- **Go Version**: Latest stable
- **Benchmark Parameters**: `-benchmem -count=10` (10 samples per benchmark)
  - Action benchmarks: `-benchtime=10x` (10 iterations per sample, sufficient for ms-level operations)
  - API micro-benchmarks (PodInfo.Clone, IsTaskAllocatable): default auto-calibrated `b.N` (millions of iterations for stable ns-level timing)

### Benchmark Methodology

Ten benchmark runs were executed (`-count=10`). Results below report the mean across runs.

### Baseline Results Summary

Benchmarking focus areas:
1. **AllocateAction**: Core allocation logic across small (10 nodes), medium (50 nodes), and large (100 nodes) clusters
2. **ReclaimAction**: Reclaim decision-making
3. **PreemptAction**: Preemption scenario validation
4. **ConsolidationAction**: Workload consolidation logic
5. **API Operations**: Direct internal API types operations (PodInfo.Clone() , NodeInfo.IsTaskAllocatable)

### Key Performance Metrics (Average of 10 runs)

| Benchmark | Configuration | Time (ns/op) | Memory (B/op) | Allocations |
|-----------|---------------|-------------|--------------|------------|
| AllocateAction | Small Cluster (10 nodes) | 106.6M | 2.33Mi | 36.9k |
| AllocateAction | Medium Cluster (50 nodes) | 127.1M | 12.53Mi | 327.8k |
| AllocateAction | Large Cluster (100 nodes) | 183.3M | 43.20Mi | 1.401M |
| ReclaimAction | Small Cluster (10 nodes) | 102.7M | 971.5Ki | 8.8k |
| ReclaimAction | Medium Cluster (50 nodes) | 105.0M | 3.15Mi | 28.1k |
| ReclaimLargeJobs | 10 nodes | 104.4M | 1.86Mi | 19.9k |
| ReclaimLargeJobs | 50 nodes | 130.2M | 17.41Mi | 229.8k |
| ReclaimLargeJobs | 100 nodes | 241.2M | 59.23Mi | 856.0k |
| ReclaimLargeJobs | 200 nodes | 816.0M | 234.13Mi | 3.620M |
| ReclaimLargeJobs | 500 nodes | 8.97s | 1.70Gi | 29.050M |
| PreemptAction | Small Cluster (10 nodes) | 104.7M | 1.07Mi | 11.5k |
| PreemptAction | Medium Cluster (50 nodes) | 110.5M | 4.25Mi | 39.9k |
| ConsolidationAction | Small Cluster (10 nodes) | 111.4M | 5.83Mi | 74.6k |
| ConsolidationAction | Medium Cluster (50 nodes) | 187.5M | 48.24Mi | 691.9k |
| PodInfo.Clone | Minimal | 506ns | 976B | 12 |
| PodInfo.Clone | With GPU | 511ns | 976B | 12 |
| PodInfo.Clone | With Multiple GPUs | 617ns | 1184B | 13 |
| IsTaskAllocatable | best-effort-cpu-only | 55ns | 0B | 0 |
| IsTaskAllocatable | regular-gpu | 105ns | 48B | 3 |
| IsTaskAllocatable | fractional-gpu | 107ns | 48B | 3 |
| IsTaskAllocatable | mig-1g-10gb | 201ns | 0B | 0 |
| IsTaskAllocatable | gpu-memory-request | 106ns | 48B | 3 |
| IsTaskAllocatable | custom-resources-1-present | 117ns | 0B | 0 |
| IsTaskAllocatable | custom-resources-2-present | 133ns | 0B | 0 |
| IsTaskAllocatable | custom-resources-5-present | 174ns | 0B | 0 |
| IsTaskAllocatable | custom-resources-10-present | 253ns | 0B | 0 |
| IsTaskAllocatable | custom-resources-1-with-1-missing | 123ns | 48B | 3 |
| IsTaskAllocatable | custom-resources-2-with-1-missing | 132ns | 48B | 3 |
| IsTaskAllocatable | custom-resources-5-with-1-missing | 153ns | 48B | 3 |
| IsTaskAllocatable | custom-resources-10-with-1-missing | 196ns | 48B | 3 |

Notes:
- BenchmarkReclaimLargeJobs_1000Node did not complete within a 40m timeout and is omitted.
- Action benchmarks use `-benchtime=10x` (10 iterations); API micro-benchmarks use default auto-calibration (millions of iterations) for stable timing.


## After Optimization (filled in commit 10)

*Placeholder for final performance metrics and improvements.*

This section will be populated with:
- Vector-based implementation performance metrics
- Side-by-side comparison tables (before/after)
- Performance improvement percentages
- Analysis of optimization effectiveness
- Recommendations for further improvements

## Future Work: Complete Resource Struct Removal

### Task Description

After the core NodeInfo, PodInfo, and PodGroupInfo migrations are complete, additional work is needed to remove `resource_info.Resource` usage from plugins and utilities. This requires a separate design effort due to the breadth of changes and potential API implications.

### Scope

The following areas require further planning:

1. **Proportion Plugin Suite**
   - `plugins/proportion/proportion.go` - Victim resource tracking
   - `plugins/proportion/reclaimable/` - Reclaimer/reclaimee resource calculations
   - `plugins/proportion/utils/utils.go` - `QuantifyResource()` function

2. **Topology Plugin**
   - `plugins/topology/job_filtering.go` - Tree sorting and job ratio calculations
   - `plugins/topology/topology_structs.go` - `DomainInfo.IdleOrReleasingResources`

3. **Error Message Generation**
   - `api/common_info/pod_errors.go`, `job_errors.go`

4. **Predicates**
   - `k8s_internal/predicates/maxNodeResources.go`
