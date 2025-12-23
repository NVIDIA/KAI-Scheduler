// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package topology_info

import (
	"crypto/sha256"
	"fmt"

	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/common_info"
)

type TopologyConstraintInfo struct {
	PreferredLevel string
	RequiredLevel  string
	Topology       string

	schedulingConstraintsSignature common_info.SchedulingConstraintsSignature
}

func (tc *TopologyConstraintInfo) GetSchedulingConstraintsSignature() common_info.SchedulingConstraintsSignature {
	if tc == nil {
		return ""
	}

	if tc.schedulingConstraintsSignature == "" {
		tc.schedulingConstraintsSignature = tc.generateSchedulingConstraintsSignature()
	}

	return tc.schedulingConstraintsSignature
}

func (tc *TopologyConstraintInfo) generateSchedulingConstraintsSignature() common_info.SchedulingConstraintsSignature {
	hash := sha256.New()
	hash.Write([]byte(tc.Topology))
	hash.Write([]byte(tc.RequiredLevel))
	hash.Write([]byte(tc.PreferredLevel))

	return common_info.SchedulingConstraintsSignature(fmt.Sprintf("%x", hash.Sum(nil)))
}
