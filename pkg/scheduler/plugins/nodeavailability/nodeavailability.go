// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package nodeavailability

import (
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/node_info"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/pod_info"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/framework"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/log"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/plugins/scores"
)

type nodeAvailabilityPlugin struct {
	// Arguments given for the plugin
	pluginArguments map[string]string
}

// New function returns nodeAvailabilityPlugin object
func New(arguments map[string]string) framework.Plugin {
	return &nodeAvailabilityPlugin{pluginArguments: arguments}
}

func (pp *nodeAvailabilityPlugin) Name() string {
	return "nodeavailability"
}

func (pp *nodeAvailabilityPlugin) OnSessionOpen(ssn *framework.Session) {
	ssn.AddNodeOrderFn(pp.nodeOrderFn)
}

func (pp *nodeAvailabilityPlugin) nodeOrderFn(task *pod_info.PodInfo, node *node_info.NodeInfo) (float64, error) {
	score := 0.0
	if taskAllocatable := node.IsTaskAllocatable(task); taskAllocatable {
		score = scores.Availability
	}

	log.InfraLogger.V(7).Infof(
		"Estimating Task: <%v/%v> Job: <%v> for node: <%s> that has <%f> idle GPUs and <%f> releasing GPUs and <%f> allocated GPUs. Score: %f",
		task.Namespace, task.Name, task.Job, node.Name, node.Idle.GPUs(), node.Releasing.GPUs(),
		node.Used.GPUs(), score)
	return score, nil
}

func (pp *nodeAvailabilityPlugin) OnSessionClose(_ *framework.Session) {}
