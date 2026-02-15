# Fix Max Pods Preemption with Fractional GPU Pods

## Context

When preempting to schedule a high-priority fractional GPU pod on a node at max capacity, the scheduler doesn't track an expected reservation pod for virtually pipelined fraction pods, allowing other victim pods in the simulation to be re-allocated (pipelined), which cancels their eviction. This results in no actual preemption occurring, leaving the high-priority pod unscheduled.

**Test case**: `test/e2e/suites/preempt/preempt_max_pods_test.go` - "Proper reservation calculation: preempt fraction with fraction that reuses GPU group"

**Expected behavior**: Pod `kai-gmhgqgutdo/qzgbekguul` (priority 50) should either:
- Preempt one of the existing fraction pods (priority 49) and reuse their GPU group's reservation pod (the original intention of the test)
- Correctly simulate preemption of a cpu-only filler pod: during simulation:
   - Filler pod is evicted
   - Pending fraction pod is able to get scheduled, including the max pods predicate
   - Filler pod fails to get re-allocated to the node, since the scheduler tracks the expected reservation pod that will be created

**Actual behavior**: The scheduler thinks it found a valid scenario but never actually evicts anything.

## Root Cause Analysis

### The Problem Flow

1. **Node state**: 109/110 pods (maxPods - 1), including 3 fraction pods sharing 1 reservation pod
2. **Preemption starts**: Scheduler evicts a low-priority CPU filler pod → creates 2 available pod slots
3. **Fraction pod allocation**: Scheduler allocates the new high-priority fraction pod
   - Scheduler detects it needs to create a **new** GPU group
   - This requires 1 task pod + 1 reservation pod = 2 slots total
   - After allocating task pod: 1 slot remains available
4. **Pipelining phase**: Evicted filler pod tries to re-allocate
5. **Predicate passes**: `checkMaxPodsWithGpuGroupReservation` checks available pods
   - Sees 1 available slot
   - Filler pod only needs 1 slot
   - **Problem**: The pending reservation pod hasn't been created yet, so it's not counted!
   - Predicate allows filler pod to re-allocate
6. **Eviction cancelled**: Filler pod gets re-allocated, its eviction statement is removed
7. **Invalid scenario accepted**: Scheduler believes it successfully scheduled the fraction pod with pipelining, but never actually evicted anything

### Why This Happens

The node's resource accounting during preemption simulation doesn't track **pending reservation pods** that will need to be created but don't exist yet. The `checkMaxPodsWithGpuGroupReservation` predicate only sees:
- Currently allocated pods
- Releasing pods (victims being evicted)

It doesn't see:
- Reservation pods that **will** be created as a result of the current allocation

## Solution: Virtual Reservation Pods in Statement

### Approach

Track pending reservation pod creation in the `Statement` object. When the statement commits, create **virtual reservation pods** on the node that:
1. Count towards the node's pod capacity limit
2. Consume no other resources (CPU, GPU, memory)
3. Persist for the rest of the scheduling session
4. Prevent other pods from incorrectly pipelining back

### How It Solves The Issue

**With virtual reservation pods**:
1. Evict filler pod → 2 available slots
2. Allocate fraction pod, detect new GPU group needed
3. **Add pending reservation to statement**
4. **Statement.Commit() → create virtual reservation pod on node** → 0 available slots
5. Try to pipeline filler pod
6. Predicate check: `availablePods=0 < 1` ✗ → filler stays evicted
7. ✅ **Success**: Filler is preempted, fraction pod scheduled, reservation will be created later

## Implementation Plan

### 1. Extend Statement to Track Pending Reservations

**File**: `pkg/scheduler/framework/statement.go`

Add:
- `pendingReservations []*PendingReservation` field to Statement
- `PendingReservation` struct with fields: NodeName, GpuGroup, ForTask
- `AddPendingReservation(nodeName, gpuGroup string, task *pod_info.PodInfo)` method
- Update `Commit()` to create virtual reservation pods
- Update `Checkpoint()` to track pendingReservations count
- Update `Rollback()` to remove virtual pods added after checkpoint

### 2. Add Virtual Reservation Pod Status

**File**: `pkg/scheduler/api/pod_status/status.go`

Add:
- New status constant: `VirtualReservation Status = "VirtualReservation"`
- Update `IsActiveUsedStatus()` to include VirtualReservation

### 3. Create Virtual Pod Helper

**File**: `pkg/scheduler/framework/statement.go` or new helper file

Add:
- `createVirtualReservationPod(pendingRes *PendingReservation) *pod_info.PodInfo`
  - Creates minimal Pod with:
    - Name: `virtual-reservation-{gpuGroup}`
    - Namespace: `kai-resource-reservation`
    - Label: `runai-gpu-group: {gpuGroup}`
    - Label: `virtual-reservation: "true"`
    - Status: `VirtualReservation`
  - Consumes only 1 pod slot, no other resources

### 4. Detect New GPU Group Creation During Allocation

**Files**:
- `pkg/scheduler/actions/allocate/allocate.go`
- `pkg/scheduler/actions/common/action.go` (in `AllocateJob` or similar)

Modify allocation logic to:
1. After allocating a fraction pod, check if it creates a new GPU group
2. Reuse existing `willCreateNewGpuGroup` logic from predicates plugin
3. If new group detected, call `statement.AddPendingReservation(node.Name, gpuGroupID, task)`

**Key integration point**: When calling `ssn.Allocate()` or `statement.Allocate()` for a fraction pod, check:
```go
if task.IsSharedGPURequest() {
    // Determine GPU assignment and check if new group
    gpuInfo := ssn.GetAllocatedGPUForTask(task, node)
    if gpuInfo.IsNewGroup {
        statement.AddPendingReservation(node.Name, gpuInfo.GroupID, task)
    }
}
```

### 5. Handle Preemption Path

**File**: `pkg/scheduler/actions/preempt/preempt.go` or in solver

Ensure the same detection logic applies during preemption allocation. The `AllocateJob` function used in `TryToVirtuallyAllocatePreemptorAndGetVictims` should automatically trigger the pending reservation logic.

### 6. GPU Sharing Integration

**File**: `pkg/scheduler/gpu_sharing/gpuSharing.go`

Potentially need to expose from GPU sharing logic:
- Whether a task allocation created a new GPU group
- The GPU group ID assigned

Current code has `GetNodePreferableGpuForSharing` which returns GPU info. May need to enhance return value to indicate if group is new.

### 7. Verify Virtual Pods Count in Predicates

**File**: `pkg/scheduler/plugins/predicates/predicates.go`

The existing `checkMaxPodsWithGpuGroupReservation` should already work correctly because:
- Virtual pods are added to `node.PodInfos` via `node.AddTask()`
- Available pods calculated as: `node.Idle.Get(v1.ResourcePods) + node.Releasing.Get(v1.ResourcePods)`
- Virtual pods in `VirtualReservation` status should count as allocated (verify in `IsActiveUsedStatus`)

**Verify**: Virtual pods are included in `node.Allocated` resource tracking.

## Critical Files to Modify

1. **pkg/scheduler/framework/statement.go**
   - Add pendingReservations tracking
   - Implement commit/rollback logic for virtual pods

2. **pkg/scheduler/api/pod_status/status.go**
   - Add VirtualReservation status
   - Update status helper functions

3. **pkg/scheduler/actions/common/action.go**
   - Detect new GPU group creation during allocation
   - Add pending reservations to statement

4. **pkg/scheduler/gpu_sharing/gpuSharing.go** (possibly)
   - May need to expose whether GPU assignment creates new group
   - Return GPU group ID from allocation functions

5. **pkg/scheduler/plugins/predicates/predicates.go**
   - Verify virtual pods are counted correctly (may need no changes)

## Edge Cases to Handle

1. **Multiple fraction pods on same new GPU group**: Should only create ONE virtual reservation per GPU group per node
   - Track created virtual reservations by (node, gpuGroup) to avoid duplicates

2. **GPU group reuse**: If fraction pod can reuse existing GPU group, don't create virtual reservation
   - `willCreateNewGpuGroup` should return false
   - No pending reservation added

3. **Rollback during preemption**: If scenario fails and rolls back, virtual pods must be removed
   - Checkpoint/Rollback mechanism handles this

4. **Session cleanup**: Virtual pods exist only during scheduling session
   - They're never persisted to etcd
   - New session starts fresh from real cluster state

## Testing & Verification

### Unit Tests
- Test Statement pending reservation tracking
- Test virtual pod creation
- Test checkpoint/rollback with virtual pods
- Test GPU group detection logic

### E2E Tests
1. **Existing failing test** should now pass:
   - `test/e2e/suites/preempt/preempt_max_pods_test.go`
   - "Proper reservation calculation: preempt fraction with fraction that reuses GPU group"

2. **Verify pod kai-gmhgqgutdo/qzgbekguul**:
   - Should successfully preempt one existing fraction pod
   - Should be scheduled on target node
   - One low-priority fraction pod should be terminated

3. **Additional test scenarios**:
   - Node at maxPods with CPU pods, high-priority fraction pod should preempt CPU pod and add reservation
   - Node at maxPods-1 with fraction pods, new fraction pod creating new GPU group should fail (not enough room for task+reservation)

### Debug Verification
Run test with verbose logging to observe:
- Virtual reservation pod creation in statement commit
- Virtual pods appearing in node.PodInfos
- Predicate checks seeing virtual pods in capacity calculations
- Filler pods correctly staying evicted instead of pipelining

## Success Criteria

1. ✅ Test "Proper reservation calculation: preempt fraction with fraction that reuses GPU group" passes
2. ✅ High-priority fraction pod successfully preempts and schedules
3. ✅ Virtual reservation pods prevent incorrect pipelining
4. ✅ Max pods capacity correctly accounts for pending reservations
5. ✅ No regression in other preemption or max pods tests
