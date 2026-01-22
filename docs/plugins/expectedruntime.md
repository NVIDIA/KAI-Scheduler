# Expected Runtime Plugin

## Overview

The Expected Runtime plugin for KAI-Scheduler nominates running jobs as requeue candidates once they exceed their configured expected runtime. This plugin works in conjunction with a **Requeue action** (tracked separately) that performs transactional "virtual eviction": checkpoint → virtual evict → try schedule higher-priority workloads → commit or rollback.

Unlike strict "max runtime" eviction, this plugin implements **soft eviction eligibility**: jobs exceeding the expected runtime become *eligible* to be requeued, but are only *actually* evicted if doing so allows higher-priority workloads to run. If no higher-priority jobs need resources, the job continues running.

## Key Features

- **Soft Eviction**: Jobs are only evicted when there's real resource contention, not just because time expired
- **Opt-in Configuration**: Only jobs with the `expected-runtime` annotation are considered
- **Cooldown Mechanism**: Prevents thrashing by enforcing a cooldown period after successful requeue
- **MinRuntime Compatibility**: Respects minruntime protection (Phase 1: relies on Requeue action filters)
- **Observability**: Provides metrics and logs for debugging nomination decisions

## Usage

The expectedruntime plugin is configured via PodGroup annotations. The plugin is automatically included in the scheduler configuration when deployed via the operator.

### PodGroup Configuration

Jobs can specify expected runtime using annotations:

```yaml
apiVersion: scheduling.volcano.sh/v1beta1
kind: PodGroup
metadata:
  name: my-job
  annotations:
    kai.scheduler/expected-runtime: "2h"
    kai.scheduler/requeue-delay: "10m"  # Optional: cooldown period
spec:
  preemptibility: Preemptible  # Required: job must be preemptible
  # ... other PodGroup spec
```

### Annotation Keys

- **`kai.scheduler/expected-runtime`** (required)
  - Type: Duration string (e.g., `30m`, `2h`, `1d`)
  - Meaning: After this duration, the job becomes eligible for requeue nomination
  - Format: Uses standard Go duration format (e.g., `1h30m`, `2h`, `30m`)

- **`kai.scheduler/requeue-delay`** (optional)
  - Type: Duration string (e.g., `10m`, `1h`)
  - Meaning: Cooldown period after a successful requeue commit
  - Default: Determined by Requeue action (if not specified)

- **`kai.scheduler/requeue-not-before`** (system-managed)
  - Type: RFC3339 timestamp (e.g., `2025-01-15T10:30:00Z`)
  - Meaning: Not-before gate timestamp; written by the system after committed requeue
  - **Note**: Users should not set this annotation manually. It is managed by the Requeue action.

### Expected Behavior

1. **At expected runtime**: The job becomes eligible for requeue nomination
2. **If no higher-priority contender can schedule**: Requeue action rollback, job keeps running
3. **If a contender can schedule**: Requeue action commit, job is evicted, `requeue-not-before` is set by system
4. **During cooldown**: Job is not nominated again until cooldown expires

## Eligibility Criteria

A job is nominated for requeue only if **all** of the following conditions are met:

1. **Running**: Job must have active allocated/running tasks (`GetActiveAllocatedTasksCount() > 0`)
2. **Preemptible**: Job must be marked as `Preemptible` (Phase 1 requirement for consistency)
3. **Configured**: `expected-runtime` annotation must exist and be valid (opt-in feature)
4. **Time Exceeded**: `runtime = now - LastStartTimestamp >= expectedRuntime`
5. **Cooldown Gate**: If `requeue-not-before` exists, must satisfy `now >= not-before`
6. **MinRuntime**: Phase 1 allows nomination; Requeue action filters handle minruntime protection

## Examples

### Basic Usage

```yaml
apiVersion: scheduling.volcano.sh/v1beta1
kind: PodGroup
metadata:
  name: training-job
  annotations:
    kai.scheduler/expected-runtime: "4h"
spec:
  preemptibility: Preemptible
  minMember: 1
  queue: training
```

**Behavior**: After 4 hours of runtime, the job becomes eligible for requeue. It will only be evicted if a higher-priority job needs the resources.

### With Custom Cooldown

```yaml
apiVersion: scheduling.volcano.sh/v1beta1
kind: PodGroup
metadata:
  name: long-running-job
  annotations:
    kai.scheduler/expected-runtime: "8h"
    kai.scheduler/requeue-delay: "30m"
spec:
  preemptibility: Preemptible
  minMember: 1
  queue: production
```

**Behavior**: After 8 hours, the job becomes eligible. If requeued, it won't be considered again for at least 30 minutes.

## Metrics

The plugin exposes the following Prometheus metrics:

- **`kai_scheduler_requeue_nominations_total{plugin="expectedruntime"}`**
  - Counter: Total number of requeue nominations made by the plugin

- **`kai_scheduler_requeue_nomination_skipped_total{plugin="expectedruntime",reason="<reason>"}`**
  - Counter: Total number of nominations skipped, categorized by reason
  - Reason values (finite enum):
    - `cooldown`: Job is in cooldown period
    - `missing_start`: `LastStartTimestamp` is missing
    - `invalid_duration`: `expected-runtime` annotation is invalid
    - `not_running`: Job has no active allocated tasks
    - `not_preemptible`: Job is not marked as preemptible
    - `minruntime_protected`: Job is protected by minruntime (future)
    - `clock_skew`: Clock skew detected (now < LastStartTimestamp)
    - `invalid_not_before`: `requeue-not-before` annotation is invalid

## Logging

The plugin logs at various verbosity levels:

- **V(5)**: Nomination events (job nominated/skipped with reason)
- **V(4)**: Warnings (invalid configurations, clock skew)
- **V(6)**: Debug information (detailed eligibility checks)

Example log messages:

```
Requeue candidate nominated: job=training-job/default, plugin=expectedruntime
Requeue nomination skipped: job=training-job/default, reason=cooldown
Requeue nomination skipped: job=training-job/default, reason=invalid_duration, error=...
```

## Integration with Requeue Action

The Expected Runtime plugin only **nominates** candidates. The actual eviction is performed by the **Requeue action** (tracked separately), which:

1. Collects candidates from all plugins via `Session.CollectRequeueCandidates()`
2. Deduplicates candidates (multiple plugins may nominate the same job)
3. Performs transactional virtual eviction (checkpoint → evict → try schedule → commit/rollback)
4. Writes `requeue-not-before` annotation on successful commit
5. Records union of nominators for debugging (e.g., `nominated_by="expectedruntime,proportion"`)

## Relationship to MinRuntime Plugin

The Expected Runtime plugin and MinRuntime plugin serve complementary purposes:

| Feature | MinRuntime | ExpectedRuntime |
|---------|-----------|-----------------|
| **Semantics** | Protect job from eviction | Make job eligible for requeue |
| **Trigger** | When eviction is attempted | When runtime >= expected |
| **Behavior** | Blocks eviction (filter) | Nominates candidate (nomination) |
| **Time Base** | `LastStartTimestamp` | `LastStartTimestamp` |
| **Interaction** | Protection period blocks eviction | Phase 1: allows nomination, action filters handle protection |

**Phase 1 Behavior**: Expected Runtime plugin allows nomination even if job is in minruntime protection period. The Requeue action's victim filters (which include minruntime filters) will prevent actual eviction during protection.

## Edge Cases and Error Handling

### Missing LastStartTimestamp

If `LastStartTimestamp` is missing or zero, the job is not nominated. This is logged at V(5) with reason `missing_start`.

### Clock Skew

If `now < LastStartTimestamp` (indicating clock skew), the job is not nominated. This is logged at V(4) as a warning with reason `clock_skew`.

### Invalid Duration

If `expected-runtime` annotation is missing, invalid, or non-positive, the job is not nominated. This is logged at V(4) with reason `invalid_duration`.

### Invalid Cooldown Timestamp

If `requeue-not-before` annotation exists but is invalid, the plugin conservatively skips nomination (reason `invalid_not_before`).

## Configuration

The plugin has no configurable parameters in Phase 1. It is automatically included in the scheduler configuration when deployed via the operator.

To disable the plugin, remove it from the scheduler configuration:

```yaml
tiers:
- plugins:
  # ... other plugins
  # - name: expectedruntime  # Comment out or remove
```

## Limitations

- **Phase 1**: Only supports PodGroup annotations (no Queue defaults)
- **Phase 1**: Requires Requeue action to be implemented separately
- **Phase 1**: MinRuntime interaction relies on Requeue action filters (liveness priority approach)

## Future Enhancements

- **Phase 2**: Queue-level defaults for expected-runtime
- **Phase 2**: Structured nomination output with metadata
- **Phase 3**: Promote to PodGroup/Queue spec fields with annotation fallback
- **Phase 3**: Direct minruntime hook integration

## See Also

- [Expected Runtime Plugin Design](../designs/expected-runtime-requeue/expected-runtime-plugin.md)
- [Requeue Flow Design](../designs/expected-runtime-requeue/expected-runtime-requeue-flow.md)
- [MinRuntime Plugin](./minruntime.md)
