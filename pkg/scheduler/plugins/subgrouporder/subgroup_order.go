// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package subgrouporder

import (
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/podgroup_info"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/framework"
)

type subGroupOrderPlugin struct {
	// Arguments given for the plugin
	pluginArguments map[string]string
}

func New(arguments map[string]string) framework.Plugin {
	return &subGroupOrderPlugin{pluginArguments: arguments}
}

func (sgop *subGroupOrderPlugin) Name() string {
	return "subgrouporder"
}

func (sgop *subGroupOrderPlugin) OnSessionOpen(ssn *framework.Session) {
	ssn.AddSubGroupsOrderFn(SubGroupOrderFn)
}

func SubGroupOrderFn(l, r interface{}) int {
	lv := l.(*podgroup_info.SubGroupInfo)
	rv := r.(*podgroup_info.SubGroupInfo)

	lNumActiveTasks := lv.GetNumActiveAllocatedTasks()
	rNumActiveTasks := rv.GetNumActiveAllocatedTasks()

	// Prioritize SubGroup below minAvailable
	lGangSatisfied := lNumActiveTasks >= int(lv.MinAvailable)
	rGangSatisfied := rNumActiveTasks >= int(rv.MinAvailable)
	if !lGangSatisfied && !rGangSatisfied {
		return 0
	}

	if !lGangSatisfied {
		return -1
	}
	if !rGangSatisfied {
		return 1
	}

	// Above minAvailable prioritize SubGroup with lower allocation ratio
	lAllocationRatio := float64(lNumActiveTasks) / float64(lv.MinAvailable)
	rAllocationRatio := float64(rNumActiveTasks) / float64(rv.MinAvailable)
	if lAllocationRatio < rAllocationRatio {
		return -1
	}
	if rAllocationRatio < lAllocationRatio {
		return 1
	}
	return 0
}

func (sgop *subGroupOrderPlugin) OnSessionClose(_ *framework.Session) {}
