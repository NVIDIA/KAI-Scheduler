# Cross-Session Caching for reflectjoborder Plugin

## Overview

This document describes the cross-session caching optimization for the `reflectjoborder` plugin. The plugin computes job ordering in every scheduling session via `OnSessionOpen`, even though the `/get-job-order` HTTP endpoint is rarely queried. When cluster state is unchanged between sessions, this computation is redundant.

## Motivation

The `reflectjoborder` plugin eagerly computes job ordering on every scheduling session. Benchmarks confirmed the cost scales significantly with cluster size:

| Cluster Size | Total Session Open | Plugin OnSessionOpen | Plugin % of Total |
|---|---|---|---|
| Small (50 jobs) | 102ms | 0.48ms | ~0.5% |
| Medium (500 jobs) | 105ms | 3.1ms | ~3% |
| Large (5000 jobs) | 148ms | 72.9ms | ~49% |

At large scale, this single plugin dominates session open time.

## Problem Statement

Each scheduling session creates a new `JobOrderPlugin` instance via the `PluginBuilder` closure. The plugin then fully recomputes the job order from scratch by building and draining a `JobsOrderByQueues` priority queue tree. When cluster state (jobs, queues, nodes) hasn't changed between sessions, this recomputation is wasteful.

## Detailed Design

### Fingerprint-Based Change Detection

A 64-bit FNV-1a hash is computed over the session state that affects job ordering:

1. **PodGroupInfos**: UID, Priority, Queue, IsReadyForScheduling, PodStatusIndex counts per status
2. **QueueInfos**: UID, Priority, ParentQueue, ChildQueues, GPU/CPU/Memory quota/limit/weight
3. **QueueResourceUsage**: Queue ID + resource values
4. **Nodes**: Name + GPU capacity (affects DRF calculations)
5. **JobsDepth**: The Allocate action depth config value

All map iterations are sorted by key for deterministic hashing.

### Closure-Captured Cache

The `NewBuilder()` function returns a `PluginBuilder` closure that captures a shared `jobOrderCache` struct:

```go
type jobOrderCache struct {
    order       *ReflectJobOrder
    fingerprint uint64
}

func NewBuilder() framework.PluginBuilder {
    cache := &jobOrderCache{}
    return func(_ framework.PluginArguments) framework.Plugin {
        return &JobOrderPlugin{cache: cache}
    }
}
```

Each scheduling session creates a new `JobOrderPlugin` via the builder, but all instances share the same cache through the closure. The framework calls `pb(pluginOption.Arguments)` to create the plugin each session, and the returned plugin carries the shared cache pointer.

### Cache Logic in OnSessionOpen

```go
func (jp *JobOrderPlugin) OnSessionOpen(ssn *framework.Session) {
    // ... register HTTP handler ...

    if jp.cache != nil {
        fp := computeFingerprint(ssn)
        if fp == jp.cache.fingerprint && jp.cache.order != nil {
            jp.ReflectJobOrder = jp.cache.order  // cache hit
            return
        }
        jp.ReflectJobOrder = buildJobOrder(ssn)   // cache miss
        jp.cache.fingerprint = fp
        jp.cache.order = jp.ReflectJobOrder
        return
    }

    jp.ReflectJobOrder = buildJobOrder(ssn)        // no cache (tests)
}
```

### Backward Compatibility

- `New()` continues to create plugins without caching (nil cache field). Existing tests using `&JobOrderPlugin{}` are unaffected.
- `NewBuilder()` is used in production registration (`factory.go`), providing caching for the real scheduler.

## Implementation Plan

1. Add `jobOrderCache` struct and `NewBuilder()` to `reflect_job_order.go`
2. Implement `computeFingerprint()` with FNV-1a hashing
3. Extract `buildJobOrder()` helper from existing `OnSessionOpen` logic
4. Update `OnSessionOpen` to check cache before computing
5. Change plugin registration in `factory.go` to use `NewBuilder()`
6. Add benchmarks (`BenchmarkReflectJobOrder_RepeatedOnSessionOpen_*`) for repeated sessions
7. Add unit tests for cache hit, cache miss, and nil-cache scenarios

## Testing Strategy

- **Existing tests**: Continue using `&JobOrderPlugin{}` (nil cache) - verified unchanged behavior
- **Cache hit test**: `NewBuilder()` → OnSessionOpen twice with same session → second reuses cached pointer
- **Cache miss test**: `NewBuilder()` → OnSessionOpen, mutate PodGroupInfos → second call recomputes
- **Fingerprint tests**: Determinism and sensitivity to mutations
- **Benchmarks**: Repeated OnSessionOpen at small/medium/large scale

## Benchmark Results

All times below are for the **plugin's `OnSessionOpen`** only, not the total session open.
For context, total session open (all plugins) is ~102ms/105ms/148ms for small/medium/large clusters respectively.

### Before Caching (plugin OnSessionOpen, single-shot)

| Cluster | Plugin Time | Alloc | Allocs/op |
|---|---|---|---|
| Small (50 jobs) | 0.48ms | 318KB | 2708 |
| Medium (500 jobs) | 3.1ms | 2.95MB | 24743 |
| Large (5000 jobs) | 72.9ms | 76.3MB | 610978 |

### After Caching (plugin OnSessionOpen, cache hit path)

| Cluster | Plugin Time | Alloc | Allocs/op | Plugin Speedup |
|---|---|---|---|---|
| Small (50 jobs) | 46µs | 10KB | 497 | 10.5x |
| Medium (500 jobs) | 389µs | 80KB | 3733 | 7.9x |
| Large (5000 jobs) | 5.4ms | 907KB | 37034 | 13.5x |

### End-to-End Impact

| Cluster | Total Session Open | Plugin Before | Plugin After | Session Savings |
|---|---|---|---|---|
| Small (50 jobs) | 102ms | 0.48ms | 46µs | ~0.4ms (~0.4%) |
| Medium (500 jobs) | 105ms | 3.1ms | 389µs | ~2.7ms (~2.6%) |
| Large (5000 jobs) | 148ms | 72.9ms | 5.4ms | ~67.5ms (~46%) |

The cache hit path is dominated by fingerprint computation. At large scale, the plugin drops from ~49% of total session open time to ~3.6%, saving ~67ms per session. Memory allocation drops from 76MB to 907KB.
