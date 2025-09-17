# Priority and Preemptibility Separation (P0)

*Status: Draft*

## Table of Contents
1. [Background](#background)
2. [Problem Statement](#problem-statement)
3. [Goals / Non-Goals](#goals--non-goals)
4. [Proposal](#proposal)
   1. [User-Visible Changes](#user-visible-changes)
   2. [API Changes](#api-changes)
   3. [Scheduler Logic Changes](#scheduler-logic-changes)
   4. [Backward Compatibility](#backward-compatibility)
5. [Implementation Plan](#implementation-plan)
6. [Examples](#examples)
7. [Testing Strategy](#testing-strategy)
8. [Risks & Mitigations](#risks--mitigations)

---

## Background

Currently, Run:ai users can submit workloads with associated priority classes that apply to all subordinate pods. The scheduler implicitly assumes that all workloads using priority classes lower than 100 are preemptible, while workloads using priority classes higher than or equal to 100 are non-preemptible.

This coupling between priority and preemptibility limits usage flexibility for several important use cases:

1. **High-priority preemptible workloads**: Users may want to submit high-priority workloads that are still preemptible (e.g., high-priority training workloads)
2. **Low-priority non-preemptible workloads**: Users may want to submit low-priority workloads (e.g., data processing) that run to completion when granted resources
3. **Semi-preemptible workloads**: Users may want to submit composite workloads with min-members count where min-members are non-preemptible but additional pods above min-members are preemptible

This document will handle cases 1 and 2. Case 3 will be handled in a separate document.

## Problem Statement

The current implementation creates artificial constraints that prevent users from expressing their true scheduling requirements:

- **Priority** should control the order in which workloads are considered for scheduling in a queue
- **Preemptibility** should control whether a workload can be interrupted once running
- These are orthogonal concerns that should be independently configurable

The current priority-based preemptibility determination (priority >= 100 = non-preemptible) is too simplistic and doesn't accommodate the diverse scheduling requirements of modern AI/ML workloads.

## Goals / Non-Goals

### Goals
- **Separate priority from preemptibility**: Allow independent configuration of workload priority and preemptibility
- **Maintain backward compatibility**: Existing workloads without explicit preemptibility configuration should continue to work using the legacy priority-based determination
- **Support two preemptibility modes**: Preemptible, Non-Preemptible

### Non-Goals
- **P1 features**: Semi-preemptible workloads (addressed separately)


<!-- GuyContinue -->
## Proposal

### User-Visible Changes

#### 1. New Preemptibility Parameter
Add a new `preemptibility` parameter to all workload types with three possible values:
- `Preemptible`: Workload can be preempted by higher-priority workloads
- `Non-Preemptible`: Workload runs to completion once scheduled
- `Semi-Preemptible`: Workload has both preemptible and non-preemptible components (P1 feature)

#### 2. Workload API Updates
All workload types (TrainingWorkload, InteractiveWorkload, InferenceWorkload, etc.) will include:
```yaml
spec:
  preemptibility: "Preemptible"  # or "Non-Preemptible" or "Semi-Preemptible"
  priorityClassName: "train"     # existing field, now independent of preemptibility
```

#### 3. UI/CLI Integration
- Preemptibility parameter will be prominently displayed in workload grids
- CLI commands will support preemptibility specification
- Workload creation wizards will include preemptibility selection

### API Changes

#### 1. Workload Type Updates
All workload CRDs will be updated to include the preemptibility field:

```go
type WorkloadSpec struct {
    // ... existing fields ...
    
    // Preemptibility defines whether this workload can be preempted
    // +kubebuilder:validation:Enum=Preemptible;Non-Preemptible;Semi-Preemptible
    // +kubebuilder:default=Preemptible
    Preemptibility string `json:"preemptibility,omitempty"`
}
```

#### 2. PodGroup Annotation
The PodGrouper will add a preemptibility annotation to PodGroups:

```go
const (
    PreemptibilityAnnotationKey = "kai.scheduler/preemptibility"
)

// Values
const (
    PreemptibilityPreemptible     = "Preemptible"
    PreemptibilityNonPreemptible  = "Non-Preemptible"
    PreemptibilitySemiPreemptible = "Semi-Preemptible"
)
```

#### 3. External Workload Support
External workloads (Kubernetes native resources) can specify preemptibility via annotations:

```yaml
metadata:
  annotations:
    kai.scheduler/preemptibility: "Non-Preemptible"
  labels:
    runai/priority-class: "train"
```

### Scheduler Logic Changes

#### 1. Preemptibility Determination
The scheduler will determine preemptibility using the following precedence:

1. **Explicit preemptibility annotation/label** on PodGroup or workload
2. **Legacy priority-based determination** (priority >= 100 = non-preemptible) for backward compatibility

```go
func (pgi *PodGroupInfo) IsPreemptibleJob() bool {
    // Check for explicit preemptibility annotation
    if preemptibility, exists := pgi.Annotations[PreemptibilityAnnotationKey]; exists {
        return preemptibility == PreemptibilityPreemptible
    }
    
    // Fall back to legacy priority-based determination
    return pgi.Priority < PriorityBuildNumber
}
```

#### 2. Preemption Filter Updates
The preemption filter will be updated to respect the new preemptibility determination:

```go
func buildFilterFuncForPreempt(ssn *framework.Session, preemptor *podgroup_info.PodGroupInfo) func(*podgroup_info.PodGroupInfo) bool {
    return func(job *podgroup_info.PodGroupInfo) bool {
        // Use new preemptibility determination
        if !job.IsPreemptibleJob() {
            return false
        }
        
        // ... rest of existing logic ...
    }
}
```

#### 3. Quota Validation Updates
Non-preemptible workloads will continue to be restricted to in-quota resources, but the determination of what constitutes a non-preemptible workload will use the new logic.

### Backward Compatibility

#### 1. Legacy Workload Support
Workloads without explicit preemptibility configuration will continue to use the legacy priority-based determination:
- Priority < 100 → Preemptible
- Priority >= 100 → Non-Preemptible

#### 2. Migration Path
Users can gradually migrate to explicit preemptibility configuration:
1. **Phase 1**: Deploy with new scheduler version (backward compatible)
2. **Phase 2**: Update workloads to use explicit preemptibility (optional)
3. **Phase 3**: Remove legacy priority-based determination (future version)

#### 3. Configuration Validation
The scheduler will log warnings when it encounters workloads using legacy priority-based preemptibility determination to encourage migration.

## Implementation Plan

### Phase 1: Core Infrastructure (P0)
1. **API Updates**
   - Add preemptibility field to all workload CRDs
   - Update PodGrouper to handle preemptibility annotations
   - Add constants and validation

2. **Scheduler Logic**
   - Update `IsPreemptibleJob()` method
   - Modify preemption filters
   - Update quota validation logic

3. **Testing**
   - Unit tests for new preemptibility logic
   - Integration tests for backward compatibility
   - E2E tests for new functionality

### Phase 2: UI/CLI Integration (P0)
1. **CLI Updates**
   - Add preemptibility parameter to workload creation commands
   - Update workload listing to show preemptibility

2. **UI Updates**
   - Add preemptibility column to workload grids
   - Update workload creation forms
   - Add preemptibility indicators

### Phase 3: Documentation and Migration (P0)
1. **Documentation**
   - Update user guides
   - Create migration documentation
   - Update API documentation

2. **Migration Tools**
   - Create migration scripts for existing workloads
   - Provide validation tools

### Phase 4: Semi-Preemptible Support (P1)
1. **Min-Replicas Implementation**
   - Add min-replicas support to workload types
   - Implement semi-preemptible logic in scheduler
   - Update preemption filters for semi-preemptible workloads

## Examples

### Example 1: High-Priority Preemptible Training
```yaml
apiVersion: run.ai/v1
kind: TrainingWorkload
metadata:
  name: high-priority-training
spec:
  priorityClassName: "inference"  # High priority (125)
  preemptibility: "Preemptible"   # But still preemptible
  # ... rest of spec
```

### Example 2: Low-Priority Non-Preemptible Data Processing
```yaml
apiVersion: run.ai/v1
kind: TrainingWorkload
metadata:
  name: data-processing
spec:
  priorityClassName: "train"      # Low priority (50)
  preemptibility: "Non-Preemptible"  # But runs to completion
  # ... rest of spec
```

### Example 3: External Workload with Explicit Preemptibility
```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: external-workload
  annotations:
    kai.scheduler/preemptibility: "Non-Preemptible"
  labels:
    runai/priority-class: "train"
spec:
  # ... deployment spec
```

### Example 4: Legacy Workload (Backward Compatible)
```yaml
apiVersion: run.ai/v1
kind: TrainingWorkload
metadata:
  name: legacy-workload
spec:
  priorityClassName: "build"  # Priority 100 → Non-Preemptible (legacy behavior)
  # No preemptibility field → uses legacy determination
  # ... rest of spec
```

## Testing Strategy

### 1. Unit Tests
- Test preemptibility determination logic
- Test backward compatibility with legacy workloads
- Test validation of preemptibility values

### 2. Integration Tests
- Test PodGrouper annotation handling
- Test scheduler preemption behavior with new logic
- Test quota validation with explicit preemptibility

### 3. E2E Tests
- Test workload creation with explicit preemptibility
- Test preemption scenarios with mixed preemptibility modes
- Test backward compatibility with existing workloads

### 4. Performance Tests
- Ensure no performance regression in scheduling decisions
- Test scheduler behavior under high load with mixed preemptibility

## Risks & Mitigations

| Risk | Impact | Mitigation |
|------|--------|------------|
| **Backward compatibility breakage** | High | Comprehensive testing of legacy workloads, gradual migration path |
| **User confusion with new parameter** | Medium | Clear documentation, migration guides, UI improvements |
| **Scheduler performance impact** | Low | Minimal logic changes, performance testing |
| **Inconsistent preemptibility determination** | Medium | Clear precedence rules, comprehensive validation |
| **Migration complexity** | Medium | Optional migration, automated tools, clear documentation |

---

## Appendix

### Current Priority Classes
- `train` (50) - Preemptible training workloads
- `build-preemptible` (75) - Preemptible build/interactive workloads  
- `build` (100) - Non-preemptible build/interactive workloads
- `inference` (125) - Non-preemptible inference workloads

### Preemptibility Values
- `Preemptible` - Workload can be preempted by higher-priority workloads
- `Non-Preemptible` - Workload runs to completion once scheduled
- `Semi-Preemptible` - Workload has both preemptible and non-preemptible components (P1)

### Key Files to Modify
- Workload CRD definitions in `sdk/api/`
- PodGrouper logic in `kai-scheduler/pkg/podgrouper/`
- Scheduler preemption logic in `kai-scheduler/pkg/scheduler/actions/preempt/`
- Preemptibility determination in `kai-scheduler/pkg/podgroupcontroller/utilities/pod-group/`

*End of document*
