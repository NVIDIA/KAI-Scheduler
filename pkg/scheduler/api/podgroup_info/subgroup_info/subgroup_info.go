// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package subgroup_info

import (
	"crypto/sha256"
	"fmt"

	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/common_info"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/topology_info"
)

type SubGroupInfo struct {
	parent                         *SubGroupSet
	name                           string
	topologyConstraint             *topology_info.TopologyConstraintInfo
	schedulingConstraintsSignature common_info.SchedulingConstraintsSignature
}

func newSubGroupInfo(name string, topologyConstraint *topology_info.TopologyConstraintInfo) *SubGroupInfo {
	return &SubGroupInfo{
		parent:             nil,
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

func (sgi *SubGroupInfo) SetParent(parent *SubGroupSet) {
	sgi.parent = parent
}

func (sgi *SubGroupInfo) GetParent() *SubGroupSet {
	return sgi.parent
}

func (sgi *SubGroupInfo) GetSchedulingConstraintsSignature() common_info.SchedulingConstraintsSignature {
	if sgi.schedulingConstraintsSignature == "" {
		sgi.schedulingConstraintsSignature = sgi.generateSchedulingConstraintsSignature()
	}

	return sgi.schedulingConstraintsSignature
}

func (sgi *SubGroupInfo) generateSchedulingConstraintsSignature() common_info.SchedulingConstraintsSignature {
	hash := sha256.New()

	hash.Write([]byte(sgi.name))

	if sgi.topologyConstraint != nil {
		hash.Write([]byte(sgi.topologyConstraint.PreferredLevel))
		hash.Write([]byte(sgi.topologyConstraint.RequiredLevel))
		hash.Write([]byte(sgi.topologyConstraint.Topology))
	}

	return common_info.SchedulingConstraintsSignature(fmt.Sprintf("%x", hash.Sum(nil)))
}
