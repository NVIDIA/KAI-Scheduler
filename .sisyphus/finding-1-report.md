# Finding 1: `deleteNonReservedPods` Deletes Running User Pods on Missing Reservation

**Date**: 2026-03-02  
**Branch**: `erez/finding-1-delete-non-reserved-pods-race`  
**File**: `pkg/binder/binding/resourcereservation/resource_reservation.go`  
**Function**: `syncForPods` → `deleteNonReservedPods` (lines 189–200)

---

## Finding

`syncForPods` unconditionally deletes all `Running` (and `Pending`) fraction pods for a GPU group whenever no reservation pod is found for that group. This is overly aggressive: a reservation pod can disappear for reasons unrelated to the fraction pod's validity, causing healthy, actively-running user workloads to be terminated.

---

## Reproduction

**Test file**: `pkg/binder/controllers/integration_tests/delete_non_reserved_race_test.go`

**Test name**: `deleteNonReservedPods race condition / should delete running fraction pod when reservation pod is missing — demonstrating the race`

**Run command**:
```bash
KUBEBUILDER_ASSETS="$(pwd)/bin/k8s/1.34.0-linux-amd64" \
  go run github.com/onsi/ginkgo/v2/ginkgo -v --focus "deleteNonReservedPods" \
  ./pkg/binder/controllers/integration_tests/
```

**Test output** (truncated to relevant lines):
```
Running Suite: Controller Suite
===============================================================
Will run 1 of 4 specs
...
deleteNonReservedPods race condition
  should delete running fraction pod when reservation pod is missing — demonstrating the race

  INFO  Warning: Found pod without reservation, deleting
        {"name": "fraction-pod-delete-race-node",
         "namespace": "delete-race-ns",
         "gpuGroup": "delete-race-node-gpu0-group"}

• [0.022 seconds]

Ran 1 of 4 Specs in 4.401 seconds
SUCCESS! -- 1 Passed | 0 Failed | 0 Pending | 3 Skipped
```

The test **passes**, confirming the bug: a running fraction pod that carries a valid GPU group label (placed there during a prior binding) is deleted by `deleteNonReservedPods` the moment `SyncForGpuGroup` runs and finds no matching reservation pod.

---

## Root Cause

In `syncForPods` (lines 189–196):

```go
for gpuGroup, pods := range fractionPods {
    if _, found := reservationPods[gpuGroup]; !found {
        err := rsc.deleteNonReservedPods(ctx, gpuGroup, pods)
        ...
    }
}
```

The invariant assumed here is: *"a fraction pod with a GPU group label must always have a corresponding reservation pod."* This invariant can be violated in several legitimate scenarios:

1. **External reservation pod deletion**: A node drain, OOM killer, or cluster admin deletes the reservation pod. The fraction pod continues running on the node, but the next sync cycle sees the label without a reservation and deletes the user workload.

2. **Premature sync after reservation cleanup**: If a previous sync (or an external actor) deleted the reservation pod but the fraction pod is still `Running` (before kubelet kills it), the next `SyncForGpuGroup` will also attempt to delete the fraction pod — a double-kill.

3. **Replacement window**: When a reservation pod is being replaced (old deleted, new being created), there is a brief window where fraction pods exist with labels but the new reservation hasn't been created yet. Any sync in this window kills the running pod.

The `deleteNonReservedPods` function itself only targets `PodRunning` pods:
```go
if pod.Status.Phase != v1.PodRunning {
    continue
}
```
But this means the most critical case — an actively-running, healthy user workload — is precisely what gets deleted.

**Contrast with the existing fix**: The sibling code path (reservation pod exists but no fraction pods) already has a guard via `hasActiveBindRequestsForGpuGroup`. The fraction-pod deletion path has no equivalent guard.

---

## Impact

In production clusters this means:

- **Silent workload termination**: A running AI/ML job is killed with no indication that it was a scheduler-side decision. Users see their pod terminate unexpectedly and assume a node failure or OOM event.
- **Cascading failures**: If the reservation pod is deleted by a node drain, the `PodReconciler` or the drain controller will reschedule the fraction pods. But `deleteNonReservedPods` fires first, terminating them before the rescheduling completes.
- **Data loss**: Long-running training jobs (hours to days) may lose all unsaved checkpoints.
- **Severity**: High. Triggered by any external event that removes a reservation pod while its fraction pods are still running (drain, eviction, OOM, manual `kubectl delete`).

---

## Proposed Solutions

### Option 1 — Add a BindRequest guard (symmetric with existing fix)

Mirror the `hasActiveBindRequestsForGpuGroup` guard already used for the reservation-pod deletion path:

```go
for gpuGroup, pods := range fractionPods {
    if _, found := reservationPods[gpuGroup]; !found {
        hasActive, err := rsc.hasActiveBindRequestsForGpuGroup(ctx, gpuGroup)
        if err != nil {
            return err
        }
        if hasActive {
            // Reservation pod hasn't propagated yet — skip deletion
            continue
        }
        err = rsc.deleteNonReservedPods(ctx, gpuGroup, pods)
        ...
    }
}
```

**Tradeoffs**:
- ✅ Consistent with the fix already merged for the reservation-pod path.
- ✅ Protects against the cache-lag variant of the race (reservation pod just created, label not yet propagated).
- ❌ Does NOT protect against the case where the reservation pod was deleted externally AFTER a completed binding (no BindRequest remains). In this scenario the running pod still gets deleted.
- ❌ Still treats "no reservation" as an actionable signal for pod deletion.

### Option 2 — Never delete user pods; log only

`deleteNonReservedPods` should not be in the business of deleting user workloads. Instead, log a warning and let the reservation be recreated on the next sync triggered by the user pod's continued presence:

```go
func (rsc *service) syncForPods(ctx context.Context, pods []*v1.Pod, gpuGroupToSync string) error {
    ...
    for gpuGroup, pods := range fractionPods {
        if _, found := reservationPods[gpuGroup]; !found {
            logger.Info("Warning: fraction pods found without reservation pod; "+
                "reservation will be recreated on next sync", "gpuGroup", gpuGroup)
            // Trigger reservation recreation instead of deleting the user pod
        }
    }
    ...
}
```

**Tradeoffs**:
- ✅ Never kills a running user workload due to a missing reservation pod.
- ✅ Eliminates the entire class of bugs in this code path.
- ❌ Requires the reservation-recreation logic to be robust and idempotent — the system must be able to re-create a reservation pod for an already-running fraction pod.
- ❌ May leave GPU reservation in an inconsistent state if the fraction pod is somehow in a zombie state (labeled but not actually using the GPU). However, such a zombie state should be handled by the pod lifecycle (kubelet termination), not by the scheduler.

### Option 3 — Check whether the fraction pod is actually running on the node

Instead of checking BindRequests, verify that the fraction pod's node is schedulable and the pod is genuinely running (not just has a Running status):

```go
for gpuGroup, pods := range fractionPods {
    if _, found := reservationPods[gpuGroup]; !found {
        // Only delete pods whose node no longer has capacity for this GPU group
        // (i.e., the pod is in an impossible state). Pods on healthy nodes
        // should be left alone.
        logger.Info("Reservation pod missing for running fraction pods; skipping deletion", ...)
        continue
    }
}
```

**Tradeoffs**:
- ✅ Safer default: do nothing when uncertain.
- ✅ Can be combined with Option 1 or Option 2.
- ❌ Requires more complex node state inspection.
- ❌ Still does not recreate the reservation pod.

---

## Recommendation

**Option 2** (log-only, never delete user pods) is the safest and most correct approach. The responsibility of `syncForPods` should be to manage reservation pods, not to kill user workloads. Running pods should only be terminated by the pod lifecycle (eviction, preemption, user request), not by a sync loop that notices a missing reservation.

**Short-term**: Apply Option 1 as a quick guard to prevent the cache-lag variant. This is the minimal change consistent with the existing codebase style.

**Long-term**: Implement Option 2 and add logic to recreate missing reservation pods for running fraction pods.
