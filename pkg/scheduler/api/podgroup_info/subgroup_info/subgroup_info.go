// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package subgroup_info

import (
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/topology_info"
)

type SubGroupInfo struct {
	name               string
	topologyConstraint *topology_info.TopologyConstraintInfo
}

func newSubGroupInfo(name string) *SubGroupInfo {
	return &SubGroupInfo{
		name: name,
	}
}

func (sgi *SubGroupInfo) GetName() string {
	return sgi.name
}

func (sgi *SubGroupInfo) AddTopologyConstraint(tc *topology_info.TopologyConstraintInfo) {
	sgi.topologyConstraint = tc
}

func (sgi *SubGroupInfo) GetTopologyConstraint() *topology_info.TopologyConstraintInfo {
	return sgi.topologyConstraint
}
