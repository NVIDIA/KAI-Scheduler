# KAI Scheduler Scale Tests

## Overview

Scale tests validate KAI scheduler performance and correctness at large cluster sizes (hundreds to thousands of nodes). These tests simulate realistic workloads to ensure the scheduler maintains acceptable performance and correctness under scale.

## What We Test

Scale tests verify:

- **Scheduling performance**: Time to schedule large numbers of pods across many nodes
- **Topology-aware scheduling**: Correct placement of distributed jobs with topology constraints
- **Resource allocation**: Proper GPU allocation and queue quota enforcement at scale
- **Reclaim behavior**: Preemption and resource reclamation with background workloads
- **Distributed job scheduling**: Multi-pod job allocation across nodes
- **System stability**: Scheduler behavior under concurrent job creation and high load

## Test Structure

### Test Framework

Tests use **Ginkgo** for test organization and execution. The test suite (`scale_suite_test.go`) defines test contexts and scenarios.

### Node Simulation

Tests use **KWOK** (Kubernetes WithOut Kubelet) to simulate large clusters without requiring real nodes:

- **KWOK nodes**: Virtual nodes created via KWOK operator NodePools
- **Default scale**: 500 nodes (configurable via `NODE_COUNT` environment variable)
- **GPU simulation**: Fake GPU operator provides GPU resource reporting
- **Pod lifecycle**: KWOK stages simulate pod completion and status transitions

### Test Organization

Tests are organized into contexts:

- **Topology tests**: Validate topology-aware scheduling with hierarchical constraints
- **Big cluster tests**: Performance tests with large node counts
  - Cluster fill scenarios (scheduler enabled/disabled during job creation)
  - Whole GPU allocation tests
  - Distributed job scheduling
  - Reclaim scenarios


## Test Execution

- Tests will use dedicated infrastructure and will run every 24 hours
- Test results will be available for viewing through a dashboard

