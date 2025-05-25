// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package api

import (
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/pod_info"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/podgroup_info"
)

type ScenarioInfo struct {
	Preemptor *podgroup_info.PodGroupInfo
	Victims   []*VictimInfo
}

type VictimInfo struct {
	Job   *podgroup_info.PodGroupInfo
	Tasks []*pod_info.PodInfo
}
