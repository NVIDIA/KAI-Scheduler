// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

// Package capacity_policy implements queue capacity and quota checking functionality
// for the KAI scheduler. It ensures that jobs do not exceed their queue's resource
// quotas, both at the direct queue level and parent queue levels.
package capacity_policy

import (
	"fmt"

	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/common_info"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/node_info"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/pod_info"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/pod_status"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/podgroup_info"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/resource_info"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/constants"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/framework"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/log"
	rs "github.com/NVIDIA/KAI-scheduler/pkg/scheduler/plugins/proportion/resource_share"
)

// capacityCheckFn is a function type that checks if a job's requested resources
// exceed capacity limits. It returns a SchedulableResult indicating whether the
// job can be scheduled.
type capacityCheckFn func(requestedShare rs.ResourceQuantities, job *podgroup_info.PodGroupInfo) *api.SchedulableResult

// CapacityPolicy implements queue capacity checking and quota enforcement.
// It tracks queue hierarchies and ensures jobs don't exceed resource quotas
// at any level in the hierarchy.
type CapacityPolicy struct {
	queues                 map[common_info.QueueID]*rs.QueueAttributes
	isInferencePreemptible bool
}

// New creates a new CapacityPolicy instance with the given queue attributes
// and inference preemption configuration.
func New(queues map[common_info.QueueID]*rs.QueueAttributes, isInferencePreemptible bool) *CapacityPolicy {
	return &CapacityPolicy{queues, isInferencePreemptible}
}

// IsJobOverQueueCapacity checks if a job would exceed its queue's capacity
// when considering all tasks that need to be allocated. This includes both
// regular capacity limits and non-preemptible quota checks.
func (cp *CapacityPolicy) IsJobOverQueueCapacity(job *podgroup_info.PodGroupInfo,
	tasksToAllocate []*pod_info.PodInfo) *api.SchedulableResult {
	requiredQuota := getRequiredQuota(tasksToAllocate)
	requestedShareQuantities := rs.NewResourceQuantities(
		requiredQuota.MilliCPU,
		requiredQuota.Memory,
		requiredQuota.GPU)

	checkFns := []capacityCheckFn{cp.resultsOverLimit, cp.resultsWithNonPreemptibleOverQuota}
	return cp.isJobOverCapacity(requestedShareQuantities, job, checkFns)
}

// IsNonPreemptibleJobOverQuota specifically checks if a non-preemptible job
// would exceed its queue's quota. This is a stricter check than regular
// capacity checking as non-preemptible jobs have dedicated resource quotas.
func (cp *CapacityPolicy) IsNonPreemptibleJobOverQuota(job *podgroup_info.PodGroupInfo,
	tasksToAllocate []*pod_info.PodInfo) *api.SchedulableResult {

	requiredQuota := getRequiredQuota(tasksToAllocate)
	requestedShareQuantities := rs.NewResourceQuantities(
		requiredQuota.MilliCPU,
		requiredQuota.Memory,
		requiredQuota.GPU)

	checkFns := []capacityCheckFn{cp.resultsWithNonPreemptibleOverQuota}
	return cp.isJobOverCapacity(requestedShareQuantities, job, checkFns)
}

// IsTaskAllocationOnNodeOverCapacity checks if allocating a specific task
// to a node would exceed capacity limits. This considers both the node's
// resources and the queue's capacity constraints.
func (cp *CapacityPolicy) IsTaskAllocationOnNodeOverCapacity(task *pod_info.PodInfo, job *podgroup_info.PodGroupInfo,
	node *node_info.NodeInfo) *api.SchedulableResult {
	requiredInitQuota := node.GetRequiredInitQuota(task)
	requestedShare := rs.NewResourceQuantities(
		requiredInitQuota.MilliCPU,
		requiredInitQuota.Memory,
		requiredInitQuota.GPU)

	checkFns := []capacityCheckFn{cp.resultsOverLimit, cp.resultsWithNonPreemptibleOverQuota}
	return cp.isJobOverCapacity(requestedShare, job, checkFns)
}

// isJobOverCapacity is an internal helper that runs a series of capacity
// check functions to determine if a job exceeds any resource limits.
func (cp *CapacityPolicy) isJobOverCapacity(requestedShare rs.ResourceQuantities, job *podgroup_info.PodGroupInfo,
	checkFns []capacityCheckFn) *api.SchedulableResult {
	for _, checkFn := range checkFns {
		result := checkFn(requestedShare, job)
		if !result.IsSchedulable {
			log.InfraLogger.V(5).Infof("Job: <%v/%v> is over capacity. Reason: %v", job.Namespace, job.Name, result.Message)
			return result
		}
	}

	return Schedulable()
}

// getRequiredQuota calculates the total resource requirements for a set of tasks.
// This includes CPU, Memory, and GPU resources.
func getRequiredQuota(tasksToAllocate []*pod_info.PodInfo) *podgroup_info.JobRequirement {
	quota := podgroup_info.JobRequirement{}
	for _, pod := range tasksToAllocate {
		quota.GPU += pod.ResReq.GetSumGPUs()
		quota.MilliCPU += pod.ResReq.Cpu()
		quota.Memory += pod.ResReq.Memory()
	}
	return &quota
}

// getFirstPendingPod returns the first pod in a job that is in Pending status.
// This is used to avoid duplicate quota checks for the same job.
func getFirstPendingPod(job *podgroup_info.PodGroupInfo) *pod_info.PodInfo {
	for _, pod := range job.PodInfos {
		if pod.Status == pod_status.Pending {
			return pod
		}
	}
	return nil
}

// isPreemptibleJob checks if a job is preemptible based on its priority.
// Preemptible jobs are allowed to exceed queue limits/quotas.
func (cp *CapacityPolicy) isPreemptibleJob(job *podgroup_info.PodGroupInfo) bool {
	preemptiblePriorities := map[int32]bool{
		constants.PriorityInferenceNumber:              true,
		constants.PriorityInteractivePreemptibleNumber: true,
		constants.PriorityTrainNumber:                  true,
	}
	return preemptiblePriorities[job.Priority]
}

// checkParentQueueLimits verifies that a job's resource requirements don't
// exceed limits or quotas at any level in its queue hierarchy. This includes:
// - GPU limit/quota checks
// - CPU limit/quota checks
// - Memory limit/quota checks
//
// The function traverses up the queue hierarchy starting from the job's
// immediate parent queue. If any limit or quota would be exceeded, it returns an
// error with a detailed message.
//
// For preemptible jobs, the limit/quota checks are skipped as they are allowed
// to exceed queue limits/quotas.
func (cp *CapacityPolicy) checkParentQueueLimits(job *podgroup_info.PodGroupInfo, ssn *framework.Session) error {
	// Skip limit/quota checks for preemptible jobs
	if cp.isPreemptibleJob(job) {
		return nil
	}

	// Get queue info for this job
	queue, found := ssn.Queues[job.Queue]
	if !found {
		return nil // Can't check limits/quotas without queue info
	}

	// Calculate job's minimum required resources
	jobResources := resource_info.EmptyResource()
	for _, pod := range job.PodInfos {
		if pod.Status == pod_status.Pending {
			jobResources.AddResourceRequirements(pod.ResReq)
		}
	}

	// Traverse up the queue hierarchy
	for currentQueueID := queue.ParentQueue; currentQueueID != ""; currentQueueID = ssn.Queues[currentQueueID].ParentQueue {
		parentQueue, found := ssn.Queues[currentQueueID]
		if !found {
			break
		}

		// Check resource limits and quotas
		resourceChecks := []struct {
			resourceType string
			limit        float64
			quota        float64
			used         float64
		}{
			{"GPU", float64(parentQueue.Resources.GPU.Limit), float64(parentQueue.Resources.GPU.Quota), jobResources.GPUs()},
			{"CPU", float64(parentQueue.Resources.CPU.Limit), float64(parentQueue.Resources.CPU.Quota), jobResources.Cpu()},
			{"Memory", float64(parentQueue.Resources.Memory.Limit), float64(parentQueue.Resources.Memory.Quota), jobResources.Memory()},
		}

		for _, check := range resourceChecks {
			// Check if either limit or quota is exceeded
			if (check.limit > 0 && check.used > check.limit) || (check.quota > 0 && check.used > check.quota) {
				// Use the more restrictive value for the error message
				restrictiveValue := check.limit
				if check.quota > 0 && (check.limit == 0 || check.quota < check.limit) {
					restrictiveValue = check.quota
				}

				errorMsg := fmt.Sprintf(
					"parent queue '%s' has reached the %s of %s. "+
						"Value is %.0f, workload requested %.0f",
					parentQueue.Name,
					func() string {
						if check.limit > 0 && check.quota > 0 {
							return "limit/quota"
						} else if check.limit > 0 {
							return "limit"
						} else {
							return "quota"
						}
					}(),
					check.resourceType,
					restrictiveValue,
					check.used)

				log.InfraLogger.Warningf("Queue limit/quota exceeded: %s", errorMsg)
				return fmt.Errorf(errorMsg)
			}
		}
	}

	return nil
}

// OnSessionOpen registers the early limit checking function that prevents jobs
// from being considered for scheduling if they would exceed their parent queues' limits.
func (cp *CapacityPolicy) OnSessionOpen(ssn *framework.Session) {
	ssn.AddPrePredicateFn(func(task *pod_info.PodInfo, job *podgroup_info.PodGroupInfo) error {
		return cp.checkParentQueueLimits(job, ssn)
	})
}
