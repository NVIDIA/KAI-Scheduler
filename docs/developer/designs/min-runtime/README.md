# [DRAFT] min-runtime

## Overview

This document proposes a new feature called "reclaim-min-runtime" and "preempt-min-runtime" that provides configurable guarantees for job runtime before preemption can occur. This feature enables administrators to define minimum runtime guarantees at various levels of the resource hierarchy: node pool, nth level queue and leaf-queue.

## Motivation

Unrestrained preemption can lead to resource thrashing, where jobs are repeatedly preempted before making meaningful progress. The reclaim-min-runtime feature addresses this issue by providing configurable minimum runtime guarantees that can be set by either cluster operators or sometimes within parts of the queue to provide guarantees about minimal useful work.

## Detailed Design

### Reclaim Min Runtime Configuration

The reclaim-min-runtime parameter can be configured with the following values:

- **0 (default)**: Jobs are always preemptible via reclaims
- **-1**: Jobs are never preemptible via reclaims
- **Positive value (in seconds)**: Minimum guaranteed runtime before preemption via reclaims

### Preempt Min Runtime Configuration

In addition to protecting jobs from being preempted too early with reclaim-min-runtime, we also introduce preempt-min-runtime to ensure that in-queue preemptions are also protected with min-runtime.

The preempt-min-runtime parameter can be configured with the following values:

- **0 (default)**: Jobs can always be preempted by others (subject to reclaim-min-runtime constraints) in the same queue
- **-1**: Jobs can never be preempted by others in the same queue
- **Positive value (in seconds)**: Minimum guaranteed runtime before in-queue preemption

### Configuration Hierarchy

The configuration follows a hierarchical override structure:

1. **Node Pool Level**: Base configuration that applies to all reclaims/preemptions if not overridden by a queue
2. **Queue Level**: Overrides node pool configuration for reclaims/preemptions, can be further overridden by a child queue

### Resolving the applicable min-runtime for reclaims and preemptions

#### Reclaims - agreed (preemptor and preemptee are in different queues)
1. Resolve the lowest common ancestor between the leaf-queues of preemptor and preemptee.
2. From the LCA, take 1 step in the hierarchy towards the preemptee's leaf queue
3. Use the reclaim-min-runtime from this queue, if it is set, otherwise move back up towards root of tree and default to node pool-level configuration value.

##### Example
1. Leaf queues root.A.B.C.leaf1 (preemptor) and root.A.B.D.leaf2 (preemptee) will use the min-runtime resolved for root.A.B.D.
2. Leaf queues root.A.B.C.leaf1 (preemptor) and root.A.B.C.leaf2 (preemptee) will use the min-runtime resolved for root.A.B.C.leaf2.

#### Preemptions (preemptor and preemptee are within the same leaf-queue)
Starting from the leaf-queue, walk the tree until the first defined preempt-min-runtime is set and use that.

##### Example
1. root.A.B has preempt-min-runtime: 600, root.A.B.C.leaf1 has preempt-min-runtime: 300. Job in leaf1 will have preempt-min-runtime: 300.

1. root.A.B has preempt-min-runtime: 600, root.A.B.C.leaf1 has preempt-min-runtime unset. Job in leaf1 will have preempt-min-runtime: 600.


## Development

### Phase 1

Add startTime to PodGroup by mimicking how staleTimestamp is set today:
https://github.com/NVIDIA/KAI-Scheduler/blob/main/pkg/scheduler/cache/status_updater/default_status_updater.go#L149

This will be a readable annotation that is set to current time when the first pod of a podgroup reaches running state.

For scheduling purposes, the readable timestamp is converted to a unix timestamp when pods are snapshotted, using https://github.com/NVIDIA/KAI-Scheduler/blob/main/pkg/scheduler/api/podgroup_info/job_info.go#L77

### Phase 2

Prepare https://github.com/NVIDIA/KAI-Scheduler/blob/main/pkg/scheduler/framework/session_plugins.go to expose Preemptable() extension function (potentially moving existing isPreemptable to there?), and move Reclaimable() to call all registered functions and returning the and() result of them, meaning all extensions must consider the scenario preemptable for all to be.

### Phase 3

Implement configuration options for (preempt|reclaim)-min-runtime in node pool and queue configurations.

### Phase 4

Implement min-runtime plugin for the scheduler that extends Reclaimable and Preemptable(), which will be used to filter out jobs eligible for preemption when scheduler tries to take these actions.

For normal jobs, we will simply provide true/false based on if the resolved min-runtime given preemptor and preemptee has been exceeded based on the start time and the time of evaluation, or false if min-runtime is -1.

For elastic jobs, we will have to look at the resources required by preemptor (ReclaimerInfo), and see if that can be satisfied by preemptee by giving up pods so that MinAvailable is still satisfied. If yes or the resolved min-runtime is -1, we will return true, if false we will fall back to normal job logic.