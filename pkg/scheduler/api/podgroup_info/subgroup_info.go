// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package podgroup_info

import (
	"github.com/NVIDIA/KAI-scheduler/pkg/apis/scheduling/v2alpha2"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/pod_info"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/pod_status"
)

type SubGroupInfo struct {
	name         string
	minAvailable int32
	podInfos     pod_info.PodsMap
}

func NewSubGroupInfo(name string, minAvailable int32) *SubGroupInfo {
	return &SubGroupInfo{
		name:         name,
		minAvailable: minAvailable,
		podInfos:     pod_info.PodsMap{},
	}
}

func FromSubGroup(subGroup *v2alpha2.SubGroup) *SubGroupInfo {
	return NewSubGroupInfo(subGroup.Name, subGroup.MinMember)
}

func (sgi *SubGroupInfo) GetName() string {
	return sgi.name
}

func (sgi *SubGroupInfo) GetMinAvailable() int32 {
	return sgi.minAvailable
}

func (sgi *SubGroupInfo) SetMinAvailable(value int32) {
	sgi.minAvailable = value
}

func (sgi *SubGroupInfo) GetPodInfos() pod_info.PodsMap {
	return sgi.podInfos
}

func (sgi *SubGroupInfo) AssignTask(ti *pod_info.PodInfo) {
	sgi.podInfos[ti.UID] = ti
}

func (sgi *SubGroupInfo) WithPodInfos(podInfos pod_info.PodsMap) *SubGroupInfo {
	sgi.podInfos = podInfos
	return sgi
}

func (sgi *SubGroupInfo) IsReadyForScheduling() bool {
	readyTasks := sgi.GetNumAliveTasks() - sgi.GetNumGatedTasks()
	return int32(readyTasks) >= sgi.minAvailable
}

func (sgi *SubGroupInfo) IsGangSatisfied() bool {
	numActiveTasks := sgi.GetNumActiveUsedTasks()
	return numActiveTasks >= int(sgi.minAvailable)
}

func (sgi *SubGroupInfo) GetNumActiveAllocatedTasks() int {
	taskCount := 0
	for _, task := range sgi.podInfos {
		if pod_status.IsActiveAllocatedStatus(task.Status) {
			taskCount++
		}
	}
	return taskCount
}

func (sgi *SubGroupInfo) GetNumActiveUsedTasks() int {
	numTasks := 0
	for _, podInfo := range sgi.podInfos {
		if pod_status.IsActiveUsedStatus(podInfo.Status) {
			numTasks += 1
		}
	}
	return numTasks
}

func (sgi *SubGroupInfo) GetNumAliveTasks() int {
	numTasks := 0
	for _, task := range sgi.podInfos {
		if pod_status.IsAliveStatus(task.Status) {
			numTasks += 1
		}
	}
	return numTasks
}

func (sgi *SubGroupInfo) GetNumGatedTasks() int {
	numTasks := 0
	for _, podInfo := range sgi.podInfos {
		if podInfo.Status == pod_status.Gated {
			numTasks += 1
		}
	}
	return numTasks
}

func (sgi *SubGroupInfo) GetNumPendingTasks() int {
	numTasks := 0
	for _, podInfo := range sgi.podInfos {
		if podInfo.Status == pod_status.Pending {
			numTasks += 1
		}
	}
	return numTasks
}
