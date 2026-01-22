# Expected Runtime Plugin Deployment Testing Guide

This guide provides steps to verify that the Expected Runtime plugin is correctly deployed and functioning in a KAI Scheduler cluster.

## Prerequisites

- KAI Scheduler deployed via operator
- Kubernetes cluster access with `kubectl` configured
- Access to scheduler pod logs and metrics endpoint

## Verification Steps

### 1. Verify Plugin Configuration

Check that the `expectedruntime` plugin appears in the scheduler ConfigMap:

```bash
kubectl get configmap -n kai-scheduler <scheduler-name>-<shard-name> \
  -o jsonpath='{.data.config\.yaml}' | grep expectedruntime
```

Expected output should include:
```yaml
tiers:
- plugins:
  - name: expectedruntime
```

### 2. Verify Plugin Initialization

Check scheduler logs for plugin initialization:

```bash
SCHEDULER_POD=$(kubectl get pods -n kai-scheduler \
  -l app=scheduler -o jsonpath='{.items[0].metadata.name}')

kubectl logs -n kai-scheduler $SCHEDULER_POD | \
  grep -i "expectedruntime\|plugin.*expectedruntime"
```

Expected: No errors related to expectedruntime plugin loading.

### 3. Verify Metrics Registration

Check Prometheus metrics endpoint:

```bash
kubectl port-forward -n kai-scheduler $SCHEDULER_POD 8080:8080 &
sleep 2
curl -s http://localhost:8080/metrics | grep requeue
```

Expected metrics:
- `kai_scheduler_requeue_nominations_total{plugin="expectedruntime"}`
- `kai_scheduler_requeue_nomination_skipped_total{plugin="expectedruntime",reason="..."}`

### 4. Test Nomination with Sample PodGroup

Create a test PodGroup with expected-runtime annotation:

```yaml
apiVersion: scheduling.run.ai/v2alpha2
kind: PodGroup
metadata:
  name: test-expected-runtime
  namespace: default
  annotations:
    kai.scheduler/expected-runtime: "1m"
spec:
  preemptibility: preemptible
  minMember: 1
  queue: default-queue
---
apiVersion: v1
kind: Pod
metadata:
  name: test-pod-expected-runtime
  namespace: default
  annotations:
    pod-group-name: test-expected-runtime
  labels:
    kai.scheduler/queue: default-queue
spec:
  schedulerName: kai-scheduler
  containers:
  - name: test
    image: nginx:latest
    resources:
      requests:
        cpu: "100m"
        memory: "128Mi"
```

Apply and wait for pod to be scheduled:

```bash
kubectl apply -f test-expected-runtime.yaml
kubectl wait --for=condition=Ready pod/test-pod-expected-runtime \
  -n default --timeout=120s
```

### 5. Verify Nomination Occurs

After the expected runtime duration (1 minute in this example), check:

**Check metrics:**
```bash
curl -s http://localhost:8080/metrics | \
  grep 'requeue_nominations_total.*expectedruntime'
```

**Check scheduler logs:**
```bash
kubectl logs -n kai-scheduler $SCHEDULER_POD --tail=500 | \
  grep -i "requeue.*nominated.*expectedruntime"
```

Expected log message:
```
Requeue candidate nominated: job=test-expected-runtime/default, plugin=expectedruntime
```

**Note**: The plugin nominates candidates, but actual eviction requires the Requeue action to be implemented.

### 6. Test Cooldown Mechanism

If Requeue action is implemented and successfully evicts the job:

**Check that requeue-not-before annotation is set:**
```bash
kubectl get podgroup test-expected-runtime -n default \
  -o jsonpath='{.metadata.annotations.kai\.scheduler/requeue-not-before}'
```

**Verify cooldown prevents re-nomination:**
- Wait for job to be requeued and rescheduled
- After rescheduling, wait for expected runtime again
- Check metrics: `requeue_nomination_skipped_total{reason="cooldown"}` should increment

### 7. Test Edge Cases

**Non-preemptible job should not be nominated:**
```yaml
spec:
  preemptibility: non-preemptible
```

**Job without annotation should not be nominated:**
- Remove `kai.scheduler/expected-runtime` annotation
- Verify no nominations occur

**Job without LastStartTimestamp:**
- Create job that hasn't started yet
- Verify skip reason is `missing_start`

## Troubleshooting

### Plugin Not Loading

**Symptoms**: Plugin not in ConfigMap or scheduler logs show errors

**Check**:
1. Verify operator includes expectedruntime in plugin list (`pkg/operator/operands/scheduler/resources_for_shard.go`)
2. Check operator logs for errors
3. Verify plugin is registered in `pkg/scheduler/plugins/factory.go`

### No Nominations

**Symptoms**: Metrics show zero nominations even for eligible jobs

**Check**:
1. Verify job has `kai.scheduler/expected-runtime` annotation
2. Verify job is preemptible (`spec.preemptibility: preemptible`)
3. Verify job has active allocated tasks
4. Verify `last-start-timestamp` is set (check PodGroup annotations)
5. Check logs for skip reasons: `kubectl logs -n kai-scheduler $SCHEDULER_POD | grep "Requeue nomination skipped"`

### Metrics Not Appearing

**Symptoms**: Prometheus metrics endpoint doesn't show requeue metrics

**Check**:
1. Verify metrics are initialized in `pkg/scheduler/metrics/metrics.go`
2. Check that plugin calls `metrics.IncRequeueNominationsTotal()`
3. Verify Prometheus is scraping scheduler metrics endpoint

## Integration with Requeue Action

**Note**: The Expected Runtime plugin only nominates candidates. Actual eviction requires the Requeue action to be implemented.

To test full flow (when Requeue action is available):

1. Create a high-priority pending job
2. Create a lower-priority job with `expected-runtime` annotation that exceeds runtime
3. Verify lower-priority job is nominated
4. Verify Requeue action evicts lower-priority job
5. Verify higher-priority job is scheduled
6. Verify `requeue-not-before` annotation is set on evicted job

For detailed coordination requirements, see [REQUEUE_ACTION_COORDINATION.md](./REQUEUE_ACTION_COORDINATION.md).
