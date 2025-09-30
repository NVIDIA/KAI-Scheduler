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

func newSubGroupInfo(name string, topologyConstraint *topology_info.TopologyConstraintInfo) *SubGroupInfo {
	return &SubGroupInfo{
		name:               name,
		topologyConstraint: topologyConstraint,
	}
}

func (sgi *SubGroupInfo) GetName() string {
	return sgi.name
}

func (sgi *SubGroupInfo) GetTopologyConstraint() *topology_info.TopologyConstraintInfo {
	return sgi.topologyConstraint
}
