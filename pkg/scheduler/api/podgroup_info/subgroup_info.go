// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package podgroup_info

import (
	"github.com/NVIDIA/KAI-scheduler/pkg/apis/scheduling/v2alpha2"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/pod_info"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/pod_status"
)

type SubGroupInfo struct {
	Name         string
	MinAvailable int32
	PodInfos     pod_info.PodsMap
}

func newSubGroupInfo(name string, minAvailable int32) *SubGroupInfo {
	return &SubGroupInfo{
		Name:         name,
		MinAvailable: minAvailable,
		PodInfos:     pod_info.PodsMap{},
	}
}

func fromSubGroup(subGroup *v2alpha2.SubGroup) *SubGroupInfo {
	return newSubGroupInfo(subGroup.Name, subGroup.MinMember)
}

func (sgi *SubGroupInfo) assignTask(ti *pod_info.PodInfo) {
	sgi.PodInfos[ti.UID] = ti
}

func (sgi *SubGroupInfo) IsReadyForScheduling() bool {
	readyTasks := sgi.GetNumAliveTasks() - sgi.GetNumGatedTasks()
	return int32(readyTasks) >= sgi.MinAvailable
}

func (sgi *SubGroupInfo) GetNumActiveAllocatedTasks() int {
	taskCount := 0
	for _, task := range sgi.PodInfos {
		if pod_status.IsActiveAllocatedStatus(task.Status) {
			taskCount++
		}
	}
	return taskCount
}

func (sgi *SubGroupInfo) GetNumAliveTasks() int {
	numTasks := 0
	for _, task := range sgi.PodInfos {
		if pod_status.IsAliveStatus(task.Status) {
			numTasks += 1
		}
	}
	return numTasks
}

func (sgi *SubGroupInfo) GetNumGatedTasks() int {
	numTasks := 0
	for _, podInfo := range sgi.PodInfos {
		if podInfo.Status == pod_status.Gated {
			numTasks += 1
		}
	}
	return numTasks
}
