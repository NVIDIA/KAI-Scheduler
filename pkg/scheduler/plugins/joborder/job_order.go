// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package joborder

import (
	"encoding/json"
	"net/http"

	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/actions/utils"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/common_info"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/framework"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/log"
)

type JobOrder struct {
	ID       common_info.PodGroupID `json:"id"`
	Priority int32                  `json:"priority"`
}

type ReflectJobOrder struct {
	GlobalOrder []JobOrder                         `json:"global_order"`
	QueueOrder  map[common_info.QueueID][]JobOrder `json:"queue_order"`
}

type JobOrderPlugin struct {
	session         *framework.Session
	ReflectJobOrder *ReflectJobOrder
}

func (jp *JobOrderPlugin) Name() string {
	return "joborder"
}

func New(arguments map[string]string) framework.Plugin {
	return &JobOrderPlugin{}
}

func (jp *JobOrderPlugin) OnSessionOpen(ssn *framework.Session) {
	jp.session = ssn
	log.InfraLogger.V(3).Info("Job Order registering get-jobs")

	// Initialize empty structure
	jp.ReflectJobOrder = &ReflectJobOrder{
		GlobalOrder: []JobOrder{},
		QueueOrder:  make(map[common_info.QueueID][]JobOrder),
	}

	jobsOrderByQueues := utils.NewJobsOrderByQueues(ssn, utils.JobsOrderInitOptions{
		FilterNonPending:  true,
		FilterUnready:     true,
		MaxJobsQueueDepth: ssn.GetJobsDepth(framework.Allocate),
	})
	jobsOrderByQueues.InitializeWithJobs(ssn.PodGroupInfos)

	// Extract global order by popping jobs until empty
	for !jobsOrderByQueues.IsEmpty() {
		job := jobsOrderByQueues.PopNextJob()
		jobOrder := JobOrder{
			ID:       job.UID,
			Priority: job.Priority,
		}
		jp.ReflectJobOrder.GlobalOrder = append(jp.ReflectJobOrder.GlobalOrder, jobOrder)

		// Extract per-queue order
		queueID := job.Queue
		jp.ReflectJobOrder.QueueOrder[queueID] = append(jp.ReflectJobOrder.QueueOrder[queueID], jobOrder)
	}

	ssn.AddHttpHandler("/get-jobs", jp.serveJobs)
}

func (jp *JobOrderPlugin) OnSessionClose(ssn *framework.Session) {}

func (jp *JobOrderPlugin) serveJobs(writer http.ResponseWriter, request *http.Request) {
	if jp.ReflectJobOrder == nil {
		http.Error(writer, "Job order data not ready", http.StatusServiceUnavailable)
		return
	}

	// Serve the job order data
	writer.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(writer).Encode(jp.ReflectJobOrder); err != nil {
		http.Error(writer, "Failed to encode job order data", http.StatusInternalServerError)
	}
}
