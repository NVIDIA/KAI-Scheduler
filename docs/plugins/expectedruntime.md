# Expected Runtime Plugin

## Overview

The Expected Runtime plugin for KAI-Scheduler nominates running jobs as requeue candidates once they exceed their configured expected runtime. This plugin works in conjunction with a Requeue action that performs transactional eviction: checkpoint → virtual evict → try schedule higher-priority workloads → commit or rollback.

Unlike strict "max runtime" eviction, this plugin implements soft eviction eligibility: jobs exceeding the expected runtime become eligible to be requeued, but are only actually evicted if doing so allows higher-priority workloads to run. If no higher-priority jobs need resources, the job continues running.

## Key Features

- Soft eviction: Jobs are only evicted when there's real resource contention, not just because time expired
- Opt-in configuration: Only jobs with the `expected-runtime` annotation are considered
- Cooldown mechanism: Prevents thrashing by enforcing a cooldown period after successful requeue
- MinRuntime compatibility: Respects minruntime protection via Requeue action filters
- Observability: Provides metrics and logs for debugging nomination decisions

## Usage

The expectedruntime plugin is configured via PodGroup annotations. The plugin is automatically included in the scheduler configuration when deployed via the operator.

### PodGroup Configuration

Jobs can specify expected runtime using annotations:

```yaml
apiVersion: scheduling.run.ai/v2alpha2
kind: PodGroup
metadata:
  name: my-job
  annotations:
    kai.scheduler/expected-runtime: "2h"
    # Optional: kai.scheduler/requeue-delay (cooldown duration) — if supported by Requeue action design
spec:
  preemptibility: "preemptible"  # Required: job must be preemptible
  # ... other PodGroup spec
```

### Annotation Keys

| Annotation | Type | Required | Description |
|------------|------|----------|-------------|
| `kai.scheduler/expected-runtime` | Duration string | Yes | After this duration, the job becomes eligible for requeue nomination (e.g., `30m`, `2h`, `1d`) |
| `kai.scheduler/requeue-delay` | Duration string | No | If defined in Requeue action design: cooldown duration after a successful requeue commit |
| `kai.scheduler/requeue-not-before` | RFC3339 timestamp | No | System-managed timestamp written after committed requeue; users should not set this manually |

## Eligibility Criteria

A job is nominated for requeue only if all of the following conditions are met:

1. Job has active allocated/running tasks
2. Job is marked as preemptible (`spec.preemptibility: "preemptible"`)
3. `expected-runtime` annotation exists and is valid
4. Runtime exceeds expected duration: `now - LastStartTimestamp >= expectedRuntime`
5. Cooldown period has expired (if `requeue-not-before` exists, `now >= not-before`)

**Cooldown vs expected runtime:** Expected runtime = "after how long do we consider this job for eviction?" (time since start). Cooldown = "after we evicted it once, how long before we can nominate it again?" (avoids thrashing; enforced via `requeue-not-before`).

## Example

```yaml
apiVersion: scheduling.run.ai/v2alpha2
kind: PodGroup
metadata:
  name: training-job
  annotations:
    kai.scheduler/expected-runtime: "4h"
spec:
  preemptibility: "preemptible"
  minMember: 1
  queue: training
```

After 4 hours of runtime, the job becomes eligible for requeue. It will only be evicted if a higher-priority job needs the resources.

## Metrics

The plugin exposes the following Prometheus metrics. The metric prefix is the scheduler's metrics namespace (from `--metrics-namespace`; default is `kai` per `constants.DefaultMetricsNamespace`):

- `kai_requeue_nominations_total{plugin="expectedruntime"}`: Total number of requeue nominations
- `kai_requeue_nomination_skipped_total{plugin="expectedruntime",reason="<reason>"}`: Total number of skipped nominations

Skip reasons: `cooldown`, `missing_start`, `invalid_duration`, `not_running`, `not_preemptible`, `clock_skew`, `invalid_not_before`

If the scheduler is started with `--metrics-namespace=kai_scheduler`, the prefix becomes `kai_scheduler_` (e.g. `kai_scheduler_requeue_nominations_total`).

## Logging

The plugin logs at V(5) for nomination events, V(4) for warnings, and V(6) for debug information.

## Integration with Requeue Action

The Expected Runtime plugin only nominates candidates. The actual eviction is performed by the Requeue action, which collects candidates, deduplicates them, performs transactional virtual eviction, and writes the `requeue-not-before` annotation on successful commit.

## Relationship to MinRuntime Plugin

The Expected Runtime plugin and MinRuntime plugin serve complementary purposes:

| Feature | MinRuntime | ExpectedRuntime |
|---------|-----------|-----------------|
| Semantics | Protect job from eviction | Make job eligible for requeue |
| Trigger | When eviction is attempted | When runtime >= expected |
| Behavior | Blocks eviction (filter) | Nominates candidate (nomination) |

The Expected Runtime plugin allows nomination even if a job is in minruntime protection period. The Requeue action's victim filters (which include minruntime filters) will prevent actual eviction during protection.

## Error Handling

The plugin handles the following error cases:
- Missing or invalid `LastStartTimestamp`: Job is not nominated (reason: `missing_start`)
- Clock skew detected (`now < LastStartTimestamp`): Job is not nominated (reason: `clock_skew`)
- Invalid `expected-runtime` annotation: Job is not nominated (reason: `invalid_duration`)
- Invalid `requeue-not-before` annotation: Job is not nominated (reason: `invalid_not_before`)

## Configuration

The plugin has no configurable parameters. It is automatically included in the scheduler configuration when deployed via the operator. To disable the plugin, remove it from the scheduler configuration.

## Limitations

- Only supports PodGroup annotations (no Queue defaults)
- Requires Requeue action to be implemented separately
- MinRuntime interaction relies on Requeue action filters
- Queue fair share is not considered when nominating; design should define whether nomination or the Requeue action should take fair share into account so jobs that should keep running are not evicted

## Testing

### Verify Plugin Configuration

Check that the plugin appears in the scheduler ConfigMap:

```bash
# For default shard
kubectl get configmap kai-scheduler-default -n kai-scheduler \
  -o jsonpath='{.data.config\.yaml}' | grep expectedruntime

# For custom shard (replace <shard-name> with actual shard name)
kubectl get configmap -n kai-scheduler <scheduler-name>-<shard-name> \
  -o jsonpath='{.data.config\.yaml}' | grep expectedruntime
```

### Verify Metrics

Check Prometheus metrics endpoint:

```bash
# Find scheduler pod (adjust selector if needed)
SCHEDULER_POD=$(kubectl get pods -n kai-scheduler \
  -o jsonpath='{.items[?(@.metadata.name=~"kai-scheduler.*")].metadata.name}' | awk '{print $1}')

# Or use specific pod name if known
# SCHEDULER_POD="kai-scheduler-default-<hash>"

kubectl port-forward -n kai-scheduler $SCHEDULER_POD 8080:8080 &
sleep 2
curl -s http://localhost:8080/metrics | grep requeue
```

### Test Nomination

Create a test PodGroup with `kai.scheduler/expected-runtime: "1m"` annotation and verify nomination occurs after the expected duration by checking metrics and scheduler logs.

### Troubleshooting

- **Plugin not loading**: Verify operator includes expectedruntime in plugin list and check operator logs
- **No nominations**: Verify job has `expected-runtime` annotation, is preemptible, has active tasks, and `last-start-timestamp` is set
- **Metrics not appearing**: Verify metrics are initialized and Prometheus is scraping the scheduler metrics endpoint

## See Also

- [MinRuntime Plugin](./minruntime.md)
