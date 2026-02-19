// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package reflectjoborder

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

type jobOrderCache struct {
	order       *ReflectJobOrder
	fingerprint uint64
}

type JobOrderPlugin struct {
	session         *framework.Session
	ReflectJobOrder *ReflectJobOrder
	cache           *jobOrderCache
}

func (jp *JobOrderPlugin) Name() string {
	return "joborder"
}

func New(_ framework.PluginArguments) framework.Plugin {
	return &JobOrderPlugin{}
}

// NewBuilder returns a PluginBuilder whose closure captures a shared cache,
// allowing successive plugin instances to skip recomputation when session
// state is unchanged.
func NewBuilder() framework.PluginBuilder {
	cache := &jobOrderCache{}
	return func(_ framework.PluginArguments) framework.Plugin {
		return &JobOrderPlugin{cache: cache}
	}
}

func (jp *JobOrderPlugin) OnSessionOpen(ssn *framework.Session) {
	jp.session = ssn
	log.InfraLogger.V(3).Info("Job Order registering get-jobs")
	ssn.AddHttpHandler("/get-job-order", jp.serveJobs)

	if jp.cache != nil {
		fp := computeFingerprint(ssn)
		if fp == jp.cache.fingerprint && jp.cache.order != nil {
			jp.ReflectJobOrder = jp.cache.order
			return
		}
		jp.ReflectJobOrder = buildJobOrder(ssn)
		jp.cache.fingerprint = fp
		jp.cache.order = jp.ReflectJobOrder
		return
	}

	jp.ReflectJobOrder = buildJobOrder(ssn)
}

func (jp *JobOrderPlugin) OnSessionClose(ssn *framework.Session) {}

func buildJobOrder(ssn *framework.Session) *ReflectJobOrder {
	order := &ReflectJobOrder{
		GlobalOrder: make([]JobOrder, 0),
		QueueOrder:  make(map[common_info.QueueID][]JobOrder),
	}

	jobsOrderByQueues := utils.NewJobsOrderByQueues(ssn, utils.JobsOrderInitOptions{
		FilterNonPending:  true,
		FilterUnready:     true,
		MaxJobsQueueDepth: ssn.GetJobsDepth(framework.Allocate),
	})
	jobsOrderByQueues.InitializeWithJobs(ssn.ClusterInfo.PodGroupInfos)

	for !jobsOrderByQueues.IsEmpty() {
		job := jobsOrderByQueues.PopNextJob()
		jobOrder := JobOrder{
			ID:       job.UID,
			Priority: job.Priority,
		}
		order.GlobalOrder = append(order.GlobalOrder, jobOrder)
		order.QueueOrder[job.Queue] = append(order.QueueOrder[job.Queue], jobOrder)
	}

	return order
}

func (jp *JobOrderPlugin) serveJobs(w http.ResponseWriter, r *http.Request) {
	if jp.ReflectJobOrder == nil {
		http.Error(w, "Job order data not ready", http.StatusServiceUnavailable)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	if err := enc.Encode(jp.ReflectJobOrder); err != nil {
		http.Error(w, "Failed to encode job order data", http.StatusInternalServerError)
	}
}
