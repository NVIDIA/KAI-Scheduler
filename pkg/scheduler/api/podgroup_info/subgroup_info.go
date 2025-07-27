// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package podgroup_info

import (
	"github.com/NVIDIA/KAI-scheduler/pkg/apis/scheduling/v2alpha2"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/pod_info"
)

type SubGroupInfo struct {
	Name         string
	MinAvailable int32
	PodInfos     pod_info.PodsMap
}

func NewSubGroupInfo(name string, minAvailable int32) *SubGroupInfo {
	return &SubGroupInfo{
		Name:         name,
		MinAvailable: minAvailable,
		PodInfos:     pod_info.PodsMap{},
	}
}

func FromSubGroup(subGroup *v2alpha2.SubGroup) *SubGroupInfo {
	return NewSubGroupInfo(subGroup.Name, subGroup.MinMember)
}

func (sgi *SubGroupInfo) AssignTask(ti *pod_info.PodInfo) {
	sgi.PodInfos[ti.UID] = ti
}
