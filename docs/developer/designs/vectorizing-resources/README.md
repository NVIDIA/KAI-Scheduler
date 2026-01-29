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

- **Scale test performance**: Full scheduling cycles take 3-4 minutes for 1000 nodes, 20+ minutes for 1000+ nodes for some test cases (observed in the [scale test cluster](https://github.com/NVIDIA/KAI-Scheduler/tree/main/test/e2e/scale))
- **Bottleneck**: Node ordering functions dominate during allocation simulations (filtering scenarios)
- **Root cause**: Resource comparisons iterate over individual nodes and resources in sequence; no bulk operations

With topology-aware scheduling, time-aware scheduling, and large reclaim scenarios becoming more common, the scheduler will face increasingly complex allocation decisions. Vectorizing resources allows:

1. **Vectorized comparisons**: Compare resources for multiple nodes concurrently (the vector-per-node layout enables goroutine-based parallelism; future refactors can adopt a column-major layout for SIMD iterations)
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

## Vector Representation Design

### Core Types

```go
// pkg/scheduler/api/resource_info/resource_vector.go

// ResourceVector represents a single entity's resources as a fixed-length array.
// All vectors use the same index mapping defined by ResourceVectorMap.
type ResourceVector []float64

// ResourceVectorMap maintains the mapping from indices to resource names.
// This is created once during cluster info snapshot and shared across all nodes and pods.
// Resource names are normalized (e.g., "nvidia.com/gpu" → "gpu").
type ResourceVectorMap struct {
    resourceNames []string
    namesToIndex  map[string]int
}

// NewResourceVectorMap creates a new ResourceVectorMap initialized with core resources
// (CPU, Memory, GPU, Pods) to ensure consistent ordering.
func NewResourceVectorMap() *ResourceVectorMap

// NewResourceVector creates a zero-filled vector of the correct length for the given map.
// All vectors should be created through this factory to guarantee length consistency.
func NewResourceVector(indexMap *ResourceVectorMap) ResourceVector
```

### Resource Vector Mapping Example

For a cluster with resources: CPU, Memory, GPUs, EFA, Custom resources:

```text
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

All operations are methods on `ResourceVector`. When vectors have mismatched lengths, operations
handle this gracefully: `Add`/`Sub` extend the shorter vector, and `LessEqual` treats missing
indices as zero. `Sub` can produce negative values (used to track over-subscription).

```go
// LessEqual checks if all resources in v fit within other.
// Mismatched lengths are handled: extra elements in v must be <= 0,
// extra elements in other must be >= 0.
func (v ResourceVector) LessEqual(other ResourceVector) bool

// Add aggregates resource allocations. Extends v if other is longer.
func (v *ResourceVector) Add(other ResourceVector)

// Sub removes allocated resources. Extends v if other is longer.
// Results can be negative (indicating over-subscription).
func (v *ResourceVector) Sub(other ResourceVector)

// SetMax sets each element of v to the maximum of v[i] and other[i].
func (v ResourceVector) SetMax(other ResourceVector)

// Clone returns a deep copy of the vector.
func (v ResourceVector) Clone() ResourceVector

// Get returns the value at the given index, or 0 if index is out of bounds.
func (v ResourceVector) Get(index int) float64

// Set sets the value at the given index (no-op if out of bounds).
func (v ResourceVector) Set(index int, value float64)

// IsZero returns true if all elements are zero.
func (v ResourceVector) IsZero() bool
```

Normalization metrics for sorting (used in scenario filtering):

```go
// Normalized sum: sum(resource[i] / totalCapacity[i])
func NormalizedSum(vec, totalCapacity ResourceVector) float64 {
    var sum float64
    for i := range vec {
        if totalCapacity[i] > 0 {
            sum += vec[i] / totalCapacity[i]
        }
    }
    return sum
}

// Dominant resource (max ratio): max(resource[i] / totalCapacity[i])
func DominantResource(vec, totalCapacity ResourceVector) float64 {
    var maxRatio float64
    for i := range vec {
        if totalCapacity[i] > 0 {
            ratio := vec[i] / totalCapacity[i]
            if ratio > maxRatio {
                maxRatio = ratio
            }
        }
    }
    return maxRatio
}
```

### Conversion Functions

Conversion between existing `Resource`/`ResourceRequirements` structs and vectors:

```go
// ToVector converts a Resource struct to a ResourceVector using the given map.
func (r *Resource) ToVector(indexMap *ResourceVectorMap) ResourceVector

// FromVector populates a Resource struct from a ResourceVector.
func (r *Resource) FromVector(vec ResourceVector, indexMap *ResourceVectorMap)

// ToVector converts ResourceRequirements to a ResourceVector.
func (r *ResourceRequirements) ToVector(indexMap *ResourceVectorMap) ResourceVector

// FromVector populates ResourceRequirements from a ResourceVector.
func (r *ResourceRequirements) FromVector(vec ResourceVector, indexMap *ResourceVectorMap)

// NewResourceVectorFromResourceList creates a vector from a Kubernetes ResourceList.
func NewResourceVectorFromResourceList(resourceList v1.ResourceList, indexMap *ResourceVectorMap) ResourceVector
```

## Migration Plan

### Phase 1: Type Introduction
- Introduce `ResourceVector`, `ResourceVectorMap` types
- Create conversion functions: `Resource.ToVector()`, `Resource.FromVector()`, `NewResourceVectorFromResourceList()`
- Add vector operations as methods: `Add`, `Sub`, `LessEqual`, `Clone`, `SetMax`, `IsZero`
- Create `NewResourceVector` factory to guarantee correct vector length
- Create unit tests for vector operations

### Phase 2: Vector Map Generation
- Extend `ClusterInfoSnapshot` to build `ResourceVectorMap` from cluster state using `BuildResourceVectorMap`
- Pass shared `ResourceVectorMap` to all nodes and pods during session initialization
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

This section establishes baseline metrics for the current struct-based implementation. These metrics will be compared against the vector-based implementation (Phase 5) to quantify performance improvements.

### Test Environment

- **System**: Intel Core Ultra 7 165H
- **CPU Governor**: performance
- **Go Version**: Latest stable
- **Benchmark Parameters**: `-benchmem -benchtime=10x -count=10` (10 iterations, 10 samples)

### Benchmark Methodology

Ten benchmark runs were executed (`-count=10`). Results below report the mean across runs.

### Baseline Results Summary

Benchmarking focus areas:
1. **AllocateAction**: Core allocation logic across small (10 nodes), medium (100 nodes), and large (500 nodes) clusters
2. **ReclaimAction**: Reclaim decision-making
3. **PreemptAction**: Preemption scenario validation
4. **ConsolidationAction**: Workload consolidation logic
5. **API Operations**: Direct internal API types operations (PodInfo.Clone() , NodeInfo.IsTaskAllocatable)

### Key Performance Metrics (Average of 10 runs)

| Benchmark | Configuration | Time (ns/op) | Memory (B/op) | Allocations |
|-----------|---------------|-------------|--------------|------------|
| AllocateAction | Small Cluster (10 nodes) | 107.2M | 2.16Mi | 35.4k |
| AllocateAction | Medium Cluster (100 nodes) | 127.8M | 11.83Mi | 322.0k |
| AllocateAction | Large Cluster (500 nodes) | 184.8M | 41.48Mi | 1.386M |
| ReclaimAction | Small Cluster | 102.7M | 870.9Ki | 7.9k |
| ReclaimAction | Medium Cluster | 104.8M | 2.74Mi | 24.5k |
| ReclaimLargeJobs | 10 nodes | 104.9M | 1.65Mi | 17.9k |
| ReclaimLargeJobs | 50 nodes | 130.4M | 14.75Mi | 205.6k |
| ReclaimLargeJobs | 100 nodes | 222.0M | 49.80Mi | 772.5k |
| ReclaimLargeJobs | 200 nodes | 800.4M | 197.26Mi | 3.304M |
| ReclaimLargeJobs | 500 nodes | 8.35s | 1.44Gi | 26.970M |
| PreemptAction | Small Cluster | 103.2M | 1015.2Ki | 10.8k |
| PreemptAction | Medium Cluster | 110.4M | 3.95Mi | 37.2k |
| ConsolidationAction | Small Cluster | 111.7M | 5.56Mi | 72.4k |
| ConsolidationAction | Medium Cluster | 185.5M | 46.78Mi | 681.0k |
| UnschedulableTime | 100 nodes | 225.2M | 51.0Mi | 784k |
| UnschedulableTime | 500 nodes | 8.25s | 1.5Gi | 27.2M |
| UnschedulableTime | 1000 nodes | 65.5s | 7.9Gi | 153M |
| PodInfo.Clone | Minimal | 476ns | 576B | 7 |
| PodInfo.Clone | With GPU | 474ns | 576B | 7 |
| PodInfo.Clone | With Multiple GPUs | 457ns | 576B | 7 |

Notes:
- UnschedulableTime benchmarks are not present in the current codebase; the prior baseline values are retained in the table above.
- BenchmarkReclaimLargeJobs_1000Node did not complete within a 40m timeout and is omitted.


## After Optimization (filled in Phase 5)

*Placeholder for final performance metrics and improvements.*

This section will be populated with:
- Vector-based implementation performance metrics
- Side-by-side comparison tables (before/after)
- Performance improvement percentages
- Analysis of optimization effectiveness
- Recommendations for further improvements
