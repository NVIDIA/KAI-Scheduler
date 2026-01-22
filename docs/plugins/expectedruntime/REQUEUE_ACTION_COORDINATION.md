# Expected Runtime Plugin - Requeue Action Coordination

This document outlines the coordination points between the Expected Runtime plugin and the Requeue action implementation.

## Overview

The Expected Runtime plugin **nominates** candidates for requeue, while the Requeue action **executes** the actual eviction. This separation allows multiple plugins to nominate candidates while the action handles the transactional eviction logic.

## Integration Points

### 1. Candidate Collection

**Expected Runtime Plugin**:
- Registers nomination function via `Session.AddRequeueCandidateNominationFn()`
- Returns list of eligible jobs from `nominationFn(clusterInfo)`

**Requeue Action**:
- Calls `Session.CollectRequeueCandidates()` to gather all nominations
- Receives deduplicated union of candidates from all plugins

**Implementation Status**: ✅ Complete
- `CollectRequeueCandidates()` implemented in `pkg/scheduler/framework/session_plugins.go`
- Proper deduplication by PodGroup UID

### 2. MinRuntime Protection

**Expected Runtime Plugin (Phase 1)**:
- **Current behavior**: Allows nomination even if job is in minruntime protection period
- **Rationale**: "Liveness priority" approach - let Requeue action filters handle protection

**Requeue Action (Required)**:
- **Must implement**: MinRuntime victim filters
- **Implementation**: Use `Session.PreemptVictimFilter()` or `Session.ReclaimVictimFilter()` which include minruntime checks

**Example**:
```go
if !ssn.PreemptVictimFilter(preemptor, candidate) {
    // Job is protected by minruntime, skip
    continue
}
```

**Coordination Needed**:
- [ ] Verify Requeue action calls minruntime victim filters
- [ ] Test that minruntime-protected jobs are not evicted even if nominated

### 3. Cooldown Mechanism

**Expected Runtime Plugin**:
- **Reads**: `kai.scheduler/requeue-not-before` annotation
- **Behavior**: Skips nomination if `now < requeue-not-before`
- **Does NOT write**: Plugin never writes annotations (side-effect free)

**Requeue Action (Required)**:
- **Must check**: Cooldown gate before processing candidate
- **Must write**: On successful commit, write `requeue-not-before` annotation
- **Must NOT write**: On rollback, do not write annotation

**Implementation**:
```go
// Check cooldown before processing
if isInCooldown(candidate, now) {
    metrics.IncRequeueSkippedTotal("expectedruntime", "cooldown")
    continue
}

// After successful commit
requeueDelay := parseRequeueDelay(candidate) // from annotation or default
notBefore := now.Add(requeueDelay)
updatePodGroupAnnotation(candidate, constants.RequeueNotBeforeAnnotation, 
    notBefore.Format(time.RFC3339))
```

**Coordination Needed**:
- [ ] Verify Requeue action checks cooldown before processing
- [ ] Verify Requeue action writes annotation only on commit
- [ ] Verify annotation format is RFC3339 timestamp
- [ ] Test cooldown prevents re-nomination

### 4. Annotation Parsing

**Expected Runtime Plugin**:
- Parses `kai.scheduler/expected-runtime` (duration string)
- Parses `kai.scheduler/requeue-not-before` (RFC3339 timestamp)
- Does NOT parse `kai.scheduler/requeue-delay` (used by action)

**Requeue Action (Required)**:
- **Must parse**: `kai.scheduler/requeue-delay` for cooldown duration
- **Must handle**: Missing annotation (use default)
- **Must validate**: Duration is positive

**Shared Constants**:
All annotation keys defined in `pkg/common/constants/constants.go`:
- `ExpectedRuntimeAnnotation = "kai.scheduler/expected-runtime"`
- `RequeueDelayAnnotation = "kai.scheduler/requeue-delay"`
- `RequeueNotBeforeAnnotation = "kai.scheduler/requeue-not-before"`

### 5. Metrics Coordination

**Expected Runtime Plugin**:
- Records: `requeue_nominations_total{plugin="expectedruntime"}`
- Records: `requeue_nomination_skipped_total{plugin="expectedruntime",reason="..."}`

**Requeue Action (Recommended)**:
- Record: `requeue_commits_total{plugin="expectedruntime"}` (or `nominated_by="expectedruntime"`)
- Record: `requeue_rollbacks_total{plugin="expectedruntime"}`
- Record: `requeue_skipped_total{plugin="expectedruntime",reason="..."}`

**Coordination**:
- Use consistent plugin name: `"expectedruntime"` (lowercase, no dashes)
- Use `nominated_by` label to track which plugins nominated (for multi-plugin scenarios)
- All label values must be finite enums (no job names, timestamps, etc.)

### 6. Error Handling

**Expected Runtime Plugin**:
- Conservative: Skips nomination on any uncertainty
- Logs all skip reasons with appropriate verbosity
- Records metrics for all skip reasons

**Requeue Action**:
- Should handle invalid candidates gracefully
- Should log eviction attempts and outcomes
- Should record metrics for commits/rollbacks

## Testing Coordination

### Unit Tests

**Expected Runtime Plugin** (✅ Complete):
- All eligibility checks tested
- Edge cases covered
- Metrics verified

**Requeue Action** (Required):
- Test candidate collection from multiple plugins
- Test minruntime filter integration
- Test cooldown gate enforcement
- Test annotation writing on commit
- Test no annotation writing on rollback

### Integration Tests

**Required** (when Requeue action is implemented):

1. **No Contention Scenario**:
   - Job exceeds expected runtime
   - No higher-priority jobs pending
   - Expected: Rollback, job continues running, no cooldown set

2. **Contention Scenario**:
   - Job exceeds expected runtime
   - Higher-priority job pending
   - Expected: Commit, job evicted, `requeue-not-before` set

3. **Cooldown Scenario**:
   - Job just requeued (in cooldown)
   - Job exceeds expected runtime again
   - Expected: Not nominated (cooldown gate)

4. **MinRuntime Protection**:
   - Job in minruntime protection period
   - Job exceeds expected runtime
   - Expected: Nominated by plugin, but Requeue action filters prevent eviction

5. **Multi-Plugin Nomination**:
   - Expected Runtime plugin nominates job
   - Proportion plugin nominates same job
   - Expected: Single eviction attempt, metrics record both nominators

## Implementation Checklist for Requeue Action

- [ ] Implement `Requeue` action type in `pkg/scheduler/framework/interface.go`
- [ ] Register action in `pkg/scheduler/actions/factory.go`
- [ ] Call `Session.CollectRequeueCandidates()` to get nominations
- [ ] Check cooldown gate before processing each candidate
- [ ] Use `Session.PreemptVictimFilter()` to respect minruntime
- [ ] Implement virtual eviction with checkpoint/rollback
- [ ] Try to schedule higher-priority pending jobs
- [ ] Commit on success, rollback on failure
- [ ] Write `requeue-not-before` annotation only on commit
- [ ] Record metrics for commits, rollbacks, and cooldown blocks
- [ ] Add unit tests for all scenarios
- [ ] Add integration tests with Expected Runtime plugin

## Notes

- **Phase 1**: Expected Runtime plugin allows nomination during minruntime protection. Requeue action must enforce protection via filters.
- **Future**: Consider adding `IsProtectedByMinRuntime(job)` session hook for cleaner integration.
- **Metrics Cardinality**: All labels use finite enums (no job names, timestamps, etc.).
