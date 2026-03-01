# Binder Race Condition & Anti-Pattern Analysis

**Date**: 2026-03-01
**Branch**: `erez/fix-binder-reservation-pod-sync-race`
**Scope**: `pkg/binder/` — controllers, binding, plugins

## Concurrency Architecture Summary

The binder has **3 concurrent entry points** that share the `resourceReservation` service:

| Controller | MaxConcurrentReconciles | Triggers sync via |
|---|---|---|
| `BindRequestReconciler.Reconcile` | configurable (default 10) | `binder.Bind()` → `SyncForNode()` |
| `BindRequestReconciler.deleteHandler` | runs in informer goroutine | `SyncForGpuGroup()` directly |
| `PodReconciler` event handlers | runs in informer goroutine | `SyncForGpuGroup()` directly |

The only synchronization is `gpuGroupMutex` — a per-GPU-group lock in `resource_reservation.go`. All cache reads go through controller-runtime's informer cache (not API server).

---

## HIGH Severity Findings

### Finding 1: `deleteNonReservedPods` — Same Class of Race as the Fixed Bug

**File**: `pkg/binder/binding/resourcereservation/resource_reservation.go:384-400`

```go
func (rsc *service) deleteNonReservedPods(ctx context.Context, gpuGroup string, pods []*v1.Pod) error {
    for _, pod := range pods {
        if pod.Status.Phase != v1.PodRunning {
            continue
        }
        // DELETE the user's pod
        err := rsc.kubeClient.Delete(ctx, pod)
```

**The pattern**: `syncForPods` (line 189-196) finds fraction pods that have **no matching reservation pod**, then calls `deleteNonReservedPods` to delete them. This is the inverse of the fixed bug — here it deletes *user pods* instead of reservation pods.

**Race**: If a reservation pod was just created by Thread A (via `ReserveGpuDevice`) but hasn't propagated to the informer cache yet, Thread B's `syncForPods` will see the fraction pod but no reservation pod, and **delete the user's running pod**.

**Impact**: HIGH — deletes a running user workload. The existing BindRequest check doesn't protect this path.

**Mitigation needed**: Same approach — check for active BindRequests before deleting fraction pods that appear unreserved.

---

## MEDIUM Severity Findings

### Finding 2: `deleteHandler` Runs Outside Reconcile Loop

**File**: `pkg/binder/controllers/bindrequest_controller.go:207-224`

```go
func (r *BindRequestReconciler) deleteHandler(ctx context.Context, event event.TypedDeleteEvent[client.Object], ...) {
    bindRequest, ok := event.Object.(*schedulingv1alpha2.BindRequest)
    // ...
    for _, gpuGroup := range bindRequest.Spec.SelectedGPUGroups {
        err := r.resourceReservation.SyncForGpuGroup(ctx, gpuGroup)
```

**Problem**: This handler runs **synchronously in the informer's event handler goroutine**, not in the reconcile workqueue. This means:
1. It blocks the informer's event dispatch for all BindRequest events
2. It runs on a **different goroutine** from reconcilers — meaning it can race with active `Bind()` calls for the same GPU group
3. The `gpuGroupMutex` serializes per-group, but the *overall timing* is problematic: the BindRequest is already deleted from the cache when this runs, so the "active BindRequests" check we just added will no longer see it

**Scenario**: BindRequest for GPU group X succeeds → scheduler deletes it → `deleteHandler` fires → `SyncForGpuGroup` → cache may still be mid-propagation from the binding → could trigger premature deletion

**Practical risk**: Medium — the BindRequest is deleted only after `Phase=Succeeded`, meaning the pod labels should have propagated by then. But there's no guarantee of ordering between the BindRequest delete watch event and the pod label update watch event.

### Finding 5: ConfigMap Read-Modify-Write Without Retry

**File**: `pkg/binder/common/gpu_access.go:125-159`

```go
func UpdateConfigMapEnvironmentVariable(ctx context.Context, kubeclient client.Client, ...) error {
    configMap := &v1.ConfigMap{}
    err := kubeclient.Get(ctx, ..., configMap)     // READ from cache
    // ...
    origConfigMap := configMap.DeepCopy()
    if err = changesFunc(configMap.Data); err != nil { ... }
    err = kubeclient.Patch(ctx, configMap, client.MergeFrom(origConfigMap))  // WRITE
```

**Problem**: Classic read-modify-write without optimistic concurrency retry. If two concurrent bindings for pods sharing the same ConfigMap (same GPU group) run simultaneously:
1. Both read the same ConfigMap version from cache
2. Both modify the Data map
3. Both Patch — `MergeFrom` uses the original's ResourceVersion, so the **second Patch will fail with a conflict** (409)

**Mitigating factor**: The `gpuGroupMutex` serializes operations per GPU group during `ReserveGpuDevice`, but ConfigMap updates happen in the `Bind()` → `PreBind()` → `gpusharing.PreBind()` path, which is **outside** the mutex scope. Two bindings to the same GPU group could race here.

**Impact**: Medium — Patch failure returns error, which triggers rollback. No data corruption, but unnecessary binding failures.

### Finding 8: `syncReservationIfNeeded` Blocks Informer Event Handler

**File**: `pkg/binder/controllers/pod_controller.go:91-98, 85-87`

```go
DeleteFunc: func(ctx context.Context, deleteEvent event.DeleteEvent, ...) {
    r.syncReservationIfNeeded(ctx, deleteEvent.Object)  // blocks informer
    // ...
},
UpdateFunc: func(ctx context.Context, updateEvent event.UpdateEvent, ...) {
    if isCompletionEvent(oldObject, newObject) {
        r.syncReservationIfNeeded(ctx, updateEvent.ObjectNew)  // blocks informer
    }
```

**Problem**: `syncReservationIfNeeded` calls `SyncForGpuGroup` directly from the informer event handler goroutine. This:
1. Blocks informer event processing while `SyncForGpuGroup` runs (includes List calls + potential deletes)
2. Can race with concurrent `Bind()` operations on the same GPU group (mitigated by `gpuGroupMutex`, but adds latency)
3. Creates head-of-line blocking: all pod events queue behind an expensive sync

### Finding 10: `Bind()` Mutates Pod In-Place, Corrupting Informer Cache

**File**: `pkg/binder/plugins/k8s-plugins/k8s_plugins.go:90`

```go
// Inside K8sPlugins.PreBind:
pod.Spec.NodeName = node.Name  // mutates the informer cache copy
```

**File**: `pkg/binder/controllers/bindrequest_controller.go:129-137`

```go
pod = &v1.Pod{...}
if err = r.Client.Get(ctx, client.ObjectKeyFromObject(pod), pod); err != nil { ... }
// pod is now a reference into the informer cache
// ...
err = r.binder.Bind(ctx, pod, node, bindRequest)
// Bind() mutates pod.Spec.NodeName, pod.Labels, pod.Annotations in-place
```

**Problem**: The `pod` pointer fetched via `Client.Get()` is a reference into the informer cache. Mutating it in-place (setting `NodeName`, patching labels) corrupts the cached copy. Another reconciler reading this pod from cache could see:
- `pod.Spec.NodeName` set (making it look already bound) before the actual binding completes
- Partially-modified labels from `updatePodGPUGroup`

The reconciler at line 138 checks `pod.Spec.NodeName != ""` to skip already-bound pods — a corrupted cache could cause it to skip a pod that hasn't actually been bound yet.

---

## LOW Severity Findings (for reference)

### Finding 3: `findGPUIndexByGroup` Uses `context.Background()`

**File**: `resource_reservation.go:344` — List uses `context.Background()` instead of caller's context. Cannot be cancelled during shutdown.

### Finding 4: `createResourceReservationPod` Uses `context.Background()`

**File**: `resource_reservation.go:578` — Create uses `context.Background()` instead of caller's context.

### Finding 6: `UpsertJobConfigMap` TOCTOU on Create

**File**: `config_map.go:33-77` — Get-then-Create without handling `AlreadyExists`. Two concurrent calls could both see "not found" and both Create.

### Finding 7: Missing `ok` Check After Type Assertion on Watch Event

**File**: `resource_reservation.go:504-507` — `pod, ok := event.Object.(*v1.Pod)` but `ok` is never checked. If the cast fails, `pod` is nil → panic.

### Finding 9: `sync.Map` Shared State (Correctly Used)

**File**: `k8s_plugins.go:113, 119` — `sync.Map` for `K8sPlugins.states`. Store in PreBind, LoadAndDelete in PostBind/Rollback. Sequential flow prevents race. FYI only.

---

## Summary by Severity

| # | Severity | Finding | Type |
|---|---|---|---|
| 1 | **HIGH** | `deleteNonReservedPods` — cache-lag race deletes user pods | Cache staleness |
| 2 | **MEDIUM** | `deleteHandler` runs in informer goroutine, races with Bind | Concurrency |
| 5 | **MEDIUM** | ConfigMap read-modify-write without retry | Lost update |
| 8 | **MEDIUM** | `syncReservationIfNeeded` blocks informer | Event handler anti-pattern |
| 10 | **MEDIUM** | `Bind()` mutates pod in-place, corrupting informer cache | In-place mutation |
| 3 | LOW | `findGPUIndexByGroup` uses `context.Background()` | Context misuse |
| 4 | LOW | `createResourceReservationPod` uses `context.Background()` | Context misuse |
| 6 | LOW | `UpsertJobConfigMap` TOCTOU on Create | TOCTOU |
| 7 | LOW | Missing `ok` check after type assertion | Nil panic |
| 9 | LOW | `sync.Map` shared state (correct) | FYI |
