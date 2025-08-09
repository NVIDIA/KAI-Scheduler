// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package podgroup_info

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/pod_info"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/pod_status"
)

func TestGetTasksToEvict_Table(t *testing.T) {
	tests := []struct {
		name         string
		job          *PodGroupInfo
		tasksToEvict bool
		expectTasks  []string
	}{
		{
			name: "WithoutSubGroups_EvictOne",
			job: &PodGroupInfo{
				PodInfos: pod_info.PodsMap{
					"pod-a": simpleTask("pod-a", "", pod_status.Running),
					"pod-b": simpleTask("pod-a", "", pod_status.Running),
					"pod-c": simpleTask("pod-c", "", pod_status.Running),
				},
				MinAvailable: 1,
			},
			tasksToEvict: true,
			expectTasks:  []string{"pod-a"},
		},
		{
			name: "WithoutSubGroups_EmptyQueue",
			job: &PodGroupInfo{
				PodInfos:     pod_info.PodsMap{},
				MinAvailable: 1,
			},
			tasksToEvict: false,
			expectTasks:  []string{},
		},
		{
			name: "WithoutSubGroups_MultipleEvict",
			job: &PodGroupInfo{
				PodInfos: pod_info.PodsMap{
					"pod-a": simpleTask("pod-a", "", pod_status.Running),
					"pod-b": simpleTask("pod-b", "", pod_status.Running),
				},
				MinAvailable: 2,
			},
			tasksToEvict: false,
			expectTasks:  []string{"pod-a", "pod-b"},
		},
		{
			name: "WithSubGroups_SingleEvict",
			job: func() *PodGroupInfo {
				pg := NewPodGroupInfo("pg1")
				pg.MinAvailable = 2
				pg.SubGroups["sg1"] = NewSubGroupInfo("sg1", 1)
				pg.SubGroups["sg2"] = NewSubGroupInfo("sg2", 1)

				pg.AddTaskInfo(simpleTask("pod-1", "sg1", pod_status.Running))
				pg.AddTaskInfo(simpleTask("pod-2", "sg1", pod_status.Running))
				pg.AddTaskInfo(simpleTask("pod-3", "sg2", pod_status.Running))
				return pg
			}(),
			tasksToEvict: true,
			expectTasks:  []string{"pod-3"},
		},
		{
			name: "WithSubGroups_EvictAll",
			job: func() *PodGroupInfo {
				sub1 := NewSubGroupInfo("sg1", 1)
				sub1.AssignTask(simpleTask("pod-1", "sg1", pod_status.Running))

				sub2 := NewSubGroupInfo("sg2", 1)
				sub2.AssignTask(simpleTask("pod-2", "sg2", pod_status.Running))

				return &PodGroupInfo{
					SubGroups: map[string]*SubGroupInfo{
						"sg1": sub1,
						"sg2": sub2,
					},
					MinAvailable: 2,
				}
			}(),
			tasksToEvict: false,
			expectTasks:  []string{"pod-1", "pod-2"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var tasks []*pod_info.PodInfo
			tasks, tasksToEvict := GetTasksToEvict(tt.job, subGroupOrderFn, tasksOrderFn)
			assert.Equal(t, tt.tasksToEvict, tasksToEvict)
			var actualTaskNames []string
			for _, pod := range tasks {
				actualTaskNames = append(actualTaskNames, pod.Name)
			}
			assert.ElementsMatch(t, tt.expectTasks, actualTaskNames)
		})
	}
}
