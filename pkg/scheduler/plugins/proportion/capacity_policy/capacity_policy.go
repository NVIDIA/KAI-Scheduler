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

// OnSessionOpen is called when a new scheduling session begins. It registers
// the early quota checking function that prevents jobs from being considered
// for scheduling if they would exceed their parent queues' quotas.
func (cp *CapacityPolicy) OnSessionOpen(ssn *framework.Session) {
	// Register early quota checks
	ssn.AddPrePredicateFn(func(task *pod_info.PodInfo, job *podgroup_info.PodGroupInfo) error {
		// Only check for the first pending pod to avoid duplicate checks
		firstPending := getFirstPendingPod(job)
		if firstPending == nil || task != firstPending {
			return nil
		}

		// Check parent queue quotas
		return cp.checkParentQueueQuotas(job, ssn)
	})
}

// checkParentQueueQuotas verifies that a job's resource requirements don't
// exceed quotas at any level in its queue hierarchy. This includes:
// - GPU quota checks
// - CPU quota checks
// - Memory quota checks
//
// The function traverses up the queue hierarchy starting from the job's
// immediate parent queue. If any quota would be exceeded, it returns an
// error with a detailed message.
//
// Note: Preemptible jobs (PriorityTrainNumber) are allowed to exceed parent
// queue quotas, while non-preemptible jobs must strictly adhere to quotas.
func (cp *CapacityPolicy) checkParentQueueQuotas(job *podgroup_info.PodGroupInfo, ssn *framework.Session) error {
	// Skip quota checks for preemptible jobs
	if job.Priority == constants.PriorityTrainNumber {
		log.InfraLogger.V(5).Infof("Job: <%v/%v> is preemptible, skipping parent queue quota checks", job.Namespace, job.Name)
		return nil
	}

	// Get queue info for this job
	queue, found := ssn.Queues[job.Queue]
	if !found {
		return nil // Can't check quota without queue info
	}

	// Only check parent queues, not the job's direct queue
	currentQueueID := queue.ParentQueue

	for currentQueueID != "" {
		parentQueue, found := ssn.Queues[currentQueueID]
		if !found {
			break
		}

		// Calculate job's total resource requirements
		jobResources := resource_info.EmptyResource()
		for _, pod := range job.PodInfos {
			if pod.Status == pod_status.Pending {
				jobResources.AddResourceRequirements(pod.ResReq)
			}
		}

		// Check GPU quota
		if parentQueue.Resources.GPU.Quota > 0 && jobResources.GPUs() > float64(parentQueue.Resources.GPU.Quota) {
			errorMsg := fmt.Sprintf(
				"parent queue '%s' quota has reached the allowable limit of GPUs. "+
					"Limit is %.0f GPUs, workload requested %.0f GPUs",
				parentQueue.Name,
				parentQueue.Resources.GPU.Quota,
				jobResources.GPUs())

			// Record event
			if firstPod := getFirstPendingPod(job); firstPod != nil {
				log.InfraLogger.Warningf("Queue quota exceeded: %s", errorMsg)
			}

			return fmt.Errorf(errorMsg)
		}

		// Check CPU quota
		if parentQueue.Resources.CPU.Quota > 0 && jobResources.Cpu() > float64(parentQueue.Resources.CPU.Quota) {
			errorMsg := fmt.Sprintf(
				"parent queue '%s' quota has reached the allowable limit of CPU. "+
					"Limit is %.0f CPU, workload requested %.0f CPU",
				parentQueue.Name,
				parentQueue.Resources.CPU.Quota,
				jobResources.Cpu())

			// Record event
			if firstPod := getFirstPendingPod(job); firstPod != nil {
				log.InfraLogger.Warningf("Queue quota exceeded: %s", errorMsg)
			}

			return fmt.Errorf(errorMsg)
		}

		// Check Memory quota
		if parentQueue.Resources.Memory.Quota > 0 && jobResources.Memory() > float64(parentQueue.Resources.Memory.Quota) {
			errorMsg := fmt.Sprintf(
				"parent queue '%s' quota has reached the allowable limit of Memory. "+
					"Limit is %.0f Memory, workload requested %.0f Memory",
				parentQueue.Name,
				parentQueue.Resources.Memory.Quota,
				jobResources.Memory())

			// Record event
			if firstPod := getFirstPendingPod(job); firstPod != nil {
				log.InfraLogger.Warningf("Queue quota exceeded: %s", errorMsg)
			}

			return fmt.Errorf(errorMsg)
		}

		// Move up the hierarchy
		currentQueueID = parentQueue.ParentQueue
	}

	return nil
}
