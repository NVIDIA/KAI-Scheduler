// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package utils

import (
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/common_info"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/pod_status"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/podgroup_info"
)

type JobsOrderInitOptions struct {
	FilterUnready            bool
	FilterNonPending         bool
	FilterNonPreemptible     bool
	FilterNonActiveAllocated bool
	VictimQueue              bool
	// TODO: Rename to MaxJobsPerQueue as the name is misleading.
	MaxJobsQueueDepth int
}

func (jobsOrder *JobsOrderByQueues) InitializeWithJobs(
	jobsToOrder map[common_info.PodGroupID]*podgroup_info.PodGroupInfo) {
	for _, job := range jobsToOrder {
		if jobsOrder.options.FilterUnready && !job.IsReadyForScheduling() {
			continue
		}

		if jobsOrder.options.FilterNonPending && len(job.PodStatusIndex[pod_status.Pending]) == 0 {
			continue
		}

		if jobsOrder.options.FilterNonPreemptible && !job.IsPreemptibleJob() {
			continue
		}

		isJobActive := false
		for _, task := range job.GetAllPodsMap() {
			if pod_status.IsActiveAllocatedStatus(task.Status) {
				isJobActive = true
				break
			}
		}
		if jobsOrder.options.FilterNonActiveAllocated && !isJobActive {
			continue
		}

		// Skip jobs whose queue doesn't exist
		if _, found := jobsOrder.ssn.Queues[job.Queue]; !found {
			continue
		}

		// Skip jobs whose queue's parent queue doesn't exist
		if _, found := jobsOrder.ssn.Queues[jobsOrder.ssn.Queues[job.Queue].ParentQueue]; !found {
			continue
		}

		// Skip jobs whose queue is not a leaf queue
		if !jobsOrder.ssn.Queues[job.Queue].IsLeafQueue() {
			continue
		}

		jobsOrder.addJobToQueue(job)
	}

	jobsOrder.buildActiveQueues(jobsOrder.options.VictimQueue)
}
