// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package resourcetype

import (
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/node_info"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/pod_info"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/framework"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/log"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/plugins/scores"
)

type resourceType struct {
	pluginArguments map[string]string
}

func New(arguments map[string]string) framework.Plugin {
	return &resourceType{pluginArguments: arguments}
}

func (pp *resourceType) Name() string {
	return "resourcetype"
}

func (pp *resourceType) OnSessionOpen(ssn *framework.Session) {
	ssn.AddNodeOrderFn(pp.nodeOrderFn())
}

func (pp *resourceType) nodeOrderFn() api.NodeOrderFn {
	return func(task *pod_info.PodInfo, node *node_info.NodeInfo) (float64, error) {
		score := 0.0
		isCPUOnlyTask := task.IsCPUOnlyRequest()
		if isCPUOnlyTask && node.IsCPUOnlyNode() {
			score = scores.ResourceType
		}
		log.InfraLogger.V(7).Infof(
			"Task %s requests GPU: %t. On node with %f total allocatable GPU. Score: %f",
			task.Name, !isCPUOnlyTask, node.Allocatable.GPUs, score)
		return score, nil
	}
}

func (pp *resourceType) OnSessionClose(_ *framework.Session) {}
