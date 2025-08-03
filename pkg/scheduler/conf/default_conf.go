// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package conf

func GetDefaultSchedulerConfiguration() *SchedulerConfiguration {
	return &SchedulerConfiguration{
		Actions: "allocate, consolidation, reclaim, preempt, stalegangeviction",
		Tiers: []Tier{
			{
				Plugins: []PluginOption{
					{Name: "predicates"},
					{Name: "proportion"},
					{Name: "priority"},
					{Name: "elastic"},
					{Name: "kubeflow"},
					{Name: "ray"},
					{Name: "nodeavailability"},
					{Name: "gpusharingorder"},
					{Name: "gpupack"},
					{Name: "resourcetype"},
					{Name: "taskorder"},
					{Name: "nominatednode"},
					{Name: "dynamicresources"},
					{
						Name: "nodeplacement",
						Arguments: map[string]string{
							"cpu": "binpack",
							"gpu": "binpack",
						},
					},
					{Name: "minruntime"},
					{Name: "topology"},
				},
			},
		},
	}
}
