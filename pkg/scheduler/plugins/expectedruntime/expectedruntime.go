// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package expectedruntime

import (
	"fmt"
	"time"

	"github.com/NVIDIA/KAI-scheduler/pkg/common/constants"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/podgroup_info"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/framework"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/log"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/metrics"
	"github.com/xhit/go-str2duration/v2"
)

const (
	pluginName = "expectedruntime"
)

type expectedruntimePlugin struct{}

func New(_ framework.PluginArguments) framework.Plugin {
	return &expectedruntimePlugin{}
}

func (er *expectedruntimePlugin) Name() string {
	return pluginName
}

func (er *expectedruntimePlugin) OnSessionOpen(ssn *framework.Session) {
	ssn.AddRequeueCandidateNominationFn(er.nominationFn)
}

func (er *expectedruntimePlugin) OnSessionClose(ssn *framework.Session) {
	// No cleanup needed
}

// nominationFn nominates running jobs as requeue candidates based on expected runtime.
// It applies all eligibility checks and returns a list of eligible jobs.
func (er *expectedruntimePlugin) nominationFn(clusterInfo *api.ClusterInfo) []*podgroup_info.PodGroupInfo {
	// Pre-allocate with small capacity to reduce reallocations in large clusters
	// Nomination rate is typically low, so small initial capacity is sufficient
	candidates := make([]*podgroup_info.PodGroupInfo, 0, 4)
	now := time.Now()

	for _, job := range clusterInfo.PodGroupInfos {
		if er.isEligibleForNomination(job, now) {
			candidates = append(candidates, job)
			metrics.IncRequeueNominationsTotal(pluginName)
			log.InfraLogger.V(5).Infof(
				"Requeue candidate nominated: job=%s/%s, plugin=%s",
				job.Namespace, job.Name, pluginName)
		}
	}

	return candidates
}

// isEligibleForNomination checks if a job meets all eligibility criteria for requeue nomination.
// All checks must pass for a job to be nominated.
func (er *expectedruntimePlugin) isEligibleForNomination(job *podgroup_info.PodGroupInfo, now time.Time) bool {
	// Check 1: Running check - job must have active allocated tasks
	if job.GetActiveAllocatedTasksCount() == 0 {
		log.InfraLogger.V(6).Infof(
			"Requeue nomination skipped: job=%s/%s, reason=not_running",
			job.Namespace, job.Name)
		metrics.IncRequeueNominationSkippedTotal(pluginName, "not_running")
		return false
	}

	// Check 2: Preemptible check - job must be marked as preemptible (Phase 1 requirement)
	if !job.IsPreemptibleJob() {
		log.InfraLogger.V(5).Infof(
			"Requeue nomination skipped: job=%s/%s, reason=not_preemptible",
			job.Namespace, job.Name)
		metrics.IncRequeueNominationSkippedTotal(pluginName, "not_preemptible")
		return false
	}

	// Check 3: Config check - expected-runtime annotation must exist and be valid
	expectedRuntime, err := er.parseExpectedRuntime(job)
	if err != nil {
		log.InfraLogger.V(4).Warnf(
			"Requeue nomination skipped: job=%s/%s, reason=invalid_duration, error=%v",
			job.Namespace, job.Name, err)
		metrics.IncRequeueNominationSkippedTotal(pluginName, "invalid_duration")
		return false
	}
	if expectedRuntime <= 0 {
		log.InfraLogger.V(4).Warnf(
			"Requeue nomination skipped: job=%s/%s, reason=invalid_duration (non-positive)",
			job.Namespace, job.Name)
		metrics.IncRequeueNominationSkippedTotal(pluginName, "invalid_duration")
		return false
	}

	// Check 4: Time check - runtime must be >= expectedRuntime
	if !er.isRuntimeExceeded(job, expectedRuntime, now) {
		// Normal case - not yet at expected runtime, no logging needed
		return false
	}

	// Check 5: Cooldown gate - check if job is in cooldown period
	if er.isInCooldown(job, now) {
		log.InfraLogger.V(5).Infof(
			"Requeue nomination skipped: job=%s/%s, reason=cooldown",
			job.Namespace, job.Name)
		metrics.IncRequeueNominationSkippedTotal(pluginName, "cooldown")
		return false
	}

	// Check 6: MinRuntime interaction (Phase 1: allow nomination, rely on action filters)
	// For Phase 1, we allow nomination and let the Requeue action filters handle minruntime protection
	// This follows the "liveness priority" approach from the design

	return true
}

// parseExpectedRuntime parses the expected-runtime annotation from a job's PodGroup.
// Returns the duration and an error if parsing fails or annotation is missing.
func (er *expectedruntimePlugin) parseExpectedRuntime(job *podgroup_info.PodGroupInfo) (time.Duration, error) {
	if job.PodGroup == nil || job.PodGroup.Annotations == nil {
		return 0, fmt.Errorf("podgroup or annotations not found")
	}

	annotationValue, found := job.PodGroup.Annotations[constants.ExpectedRuntimeAnnotation]
	if !found {
		return 0, fmt.Errorf("expected-runtime annotation not found")
	}

	duration, err := str2duration.ParseDuration(annotationValue)
	if err != nil {
		return 0, fmt.Errorf("failed to parse expected-runtime: %w", err)
	}

	return duration, nil
}

// parseRequeueNotBefore parses the requeue-not-before annotation from a job's PodGroup.
// Returns the timestamp and an error if parsing fails. Returns nil, nil if annotation doesn't exist.
func (er *expectedruntimePlugin) parseRequeueNotBefore(job *podgroup_info.PodGroupInfo) (*time.Time, error) {
	if job.PodGroup == nil || job.PodGroup.Annotations == nil {
		return nil, nil
	}

	annotationValue, found := job.PodGroup.Annotations[constants.RequeueNotBeforeAnnotation]
	if !found {
		return nil, nil
	}

	timestamp, err := time.Parse(time.RFC3339, annotationValue)
	if err != nil {
		return nil, fmt.Errorf("failed to parse requeue-not-before timestamp %q: %w", annotationValue, err)
	}

	return &timestamp, nil
}

// isRuntimeExceeded checks if the job's runtime exceeds the expected runtime.
// Returns true if runtime >= expectedRuntime.
// Handles edge cases like missing LastStartTimestamp and clock skew.
func (er *expectedruntimePlugin) isRuntimeExceeded(job *podgroup_info.PodGroupInfo, expectedRuntime time.Duration, now time.Time) bool {
	// Check if LastStartTimestamp exists
	if job.LastStartTimestamp == nil || job.LastStartTimestamp.IsZero() {
		log.InfraLogger.V(5).Infof(
			"Requeue nomination skipped: job=%s/%s, reason=missing_start",
			job.Namespace, job.Name)
		metrics.IncRequeueNominationSkippedTotal(pluginName, "missing_start")
		return false
	}

	// Check for clock skew (now < LastStartTimestamp)
	if now.Before(*job.LastStartTimestamp) {
		log.InfraLogger.V(4).Warnf(
			"Requeue nomination skipped: job=%s/%s, reason=clock_skew (now=%v, lastStart=%v)",
			job.Namespace, job.Name, now, *job.LastStartTimestamp)
		metrics.IncRequeueNominationSkippedTotal(pluginName, "clock_skew")
		return false
	}

	// Calculate runtime (guaranteed non-negative after clock skew check)
	runtime := now.Sub(*job.LastStartTimestamp)

	// Check if runtime >= expectedRuntime (use >= to avoid boundary value misses)
	return runtime >= expectedRuntime
}

// isInCooldown checks if the job is currently in a cooldown period.
// Returns true if requeue-not-before exists and now < not-before.
// Returns true on parsing errors (conservative approach to prevent invalid nominations).
func (er *expectedruntimePlugin) isInCooldown(job *podgroup_info.PodGroupInfo, now time.Time) bool {
	notBefore, err := er.parseRequeueNotBefore(job)
	if err != nil {
		log.InfraLogger.V(4).Warnf(
			"Requeue nomination skipped: job=%s/%s, reason=invalid_not_before, error=%v",
			job.Namespace, job.Name, err)
		metrics.IncRequeueNominationSkippedTotal(pluginName, "invalid_not_before")
		// Conservative: if we can't parse, skip nomination to prevent potential issues
		return true
	}

	if notBefore == nil {
		// No cooldown gate set, not in cooldown
		return false
	}

	// Check if now < not-before (still in cooldown)
	return now.Before(*notBefore)
}
