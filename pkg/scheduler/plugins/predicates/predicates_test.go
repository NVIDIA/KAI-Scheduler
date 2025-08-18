// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package predicates

import (
	"fmt"
	"reflect"
	"sort"
	"strings"
	"testing"

	"k8s.io/api/core/v1"
	"k8s.io/utils/pointer"

	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/common_info"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/node_info"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/pod_info"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/podgroup_info"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/k8s_internal"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/k8s_internal/predicates"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/test_utils/jobs_fake"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/test_utils/nodes_fake"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/test_utils/plugins_fake/predicates_fake"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/test_utils/tasks_fake"
)

func Test_evaluateTaskOnPrePredicate(t *testing.T) {
	type args struct {
		task          *pod_info.PodInfo
		k8sPredicates k8s_internal.SessionPredicates
	}
	tests := []struct {
		name            string
		args            args
		wantErr         bool
		wantedErrorData []string
	}{
		{
			"All pre-predicates empty",
			args{
				task: &pod_info.PodInfo{
					Name:      "p1",
					Namespace: "ns1",
					Pod:       &v1.Pod{},
				},
				k8sPredicates: k8s_internal.SessionPredicates{
					predicates.PodFitsHostPorts:       predicates_fake.EmptyPredicate(predicates.PodFitsHostPorts),
					predicates.PodToleratesNodeTaints: predicates_fake.EmptyPredicate(predicates.PodToleratesNodeTaints),
					predicates.NodeAffinity:           predicates_fake.EmptyPredicate(predicates.NodeAffinity),
					predicates.PodAffinity:            predicates_fake.EmptyPredicate(predicates.PodAffinity),
					predicates.VolumeBinding:          predicates_fake.EmptyPredicate(predicates.VolumeBinding),
					predicates.NodeScheduler:          predicates_fake.EmptyPredicate(predicates.NodeScheduler),
				},
			},
			false,
			[]string{},
		},
		{
			name: "One failing pre predicate",
			args: args{
				task: &pod_info.PodInfo{
					Name:      "p1",
					Namespace: "ns1",
					Pod:       &v1.Pod{},
				},
				k8sPredicates: k8s_internal.SessionPredicates{
					predicates.PodFitsHostPorts:       predicates_fake.FailingPrePredicate(predicates.PodFitsHostPorts),
					predicates.PodToleratesNodeTaints: predicates_fake.EmptyPredicate(predicates.PodToleratesNodeTaints),
					predicates.NodeAffinity:           predicates_fake.EmptyPredicate(predicates.NodeAffinity),
					predicates.PodAffinity:            predicates_fake.EmptyPredicate(predicates.PodAffinity),
					predicates.VolumeBinding:          predicates_fake.EmptyPredicate(predicates.VolumeBinding),
					predicates.NodeScheduler:          predicates_fake.EmptyPredicate(predicates.NodeScheduler),
				},
			},
			wantErr: true,
			wantedErrorData: []string{"Scheduling conditions were not met for pod ns1/p1:",
				"PodFitsHostPorts: failed pre-predicate PodFitsHostPorts. Reasons: reason1, reason2"},
		},
		{
			name: "One failing pre predicate - error the same s reason",
			args: args{
				task: &pod_info.PodInfo{
					Name:      "p1",
					Namespace: "ns1",
					Pod:       &v1.Pod{},
				},
				k8sPredicates: k8s_internal.SessionPredicates{
					predicates.PodFitsHostPorts: predicates_fake.FailingPrePredicateReasonSameAsError(
						predicates.PodFitsHostPorts),
					predicates.PodToleratesNodeTaints: predicates_fake.EmptyPredicate(predicates.PodToleratesNodeTaints),
					predicates.NodeAffinity:           predicates_fake.EmptyPredicate(predicates.NodeAffinity),
					predicates.PodAffinity:            predicates_fake.EmptyPredicate(predicates.PodAffinity),
					predicates.VolumeBinding:          predicates_fake.EmptyPredicate(predicates.VolumeBinding),
					predicates.NodeScheduler:          predicates_fake.EmptyPredicate(predicates.NodeScheduler),
				},
			},
			wantErr: true,
			wantedErrorData: []string{"Scheduling conditions were not met for pod ns1/p1:",
				"PodFitsHostPorts: failed pre-predicate PodFitsHostPorts."},
		},
		{
			name: "One failing pre predicate - no reasons",
			args: args{
				task: &pod_info.PodInfo{
					Name:      "p1",
					Namespace: "ns1",
					Pod:       &v1.Pod{},
				},
				k8sPredicates: k8s_internal.SessionPredicates{
					predicates.PodFitsHostPorts:       predicates_fake.FailingPrePredicateNoReason(predicates.PodFitsHostPorts),
					predicates.PodToleratesNodeTaints: predicates_fake.EmptyPredicate(predicates.PodToleratesNodeTaints),
					predicates.NodeAffinity:           predicates_fake.EmptyPredicate(predicates.NodeAffinity),
					predicates.PodAffinity:            predicates_fake.EmptyPredicate(predicates.PodAffinity),
					predicates.VolumeBinding:          predicates_fake.EmptyPredicate(predicates.VolumeBinding),
					predicates.NodeScheduler:          predicates_fake.EmptyPredicate(predicates.NodeScheduler),
				},
			},
			wantErr: true,
			wantedErrorData: []string{"Scheduling conditions were not met for pod ns1/p1:",
				"PodFitsHostPorts: failed pre-predicate PodFitsHostPorts."},
		},
		{
			name: "Multiple failing pre predicate",
			args: args{
				task: &pod_info.PodInfo{
					Name:      "p1",
					Namespace: "ns1",
					Pod:       &v1.Pod{},
				},
				k8sPredicates: k8s_internal.SessionPredicates{
					predicates.PodFitsHostPorts:       predicates_fake.FailingPrePredicate(predicates.PodFitsHostPorts),
					predicates.PodToleratesNodeTaints: predicates_fake.EmptyPredicate(predicates.PodToleratesNodeTaints),
					predicates.NodeAffinity:           predicates_fake.EmptyPredicate(predicates.NodeAffinity),
					predicates.PodAffinity:            predicates_fake.FailingPrePredicate(predicates.PodAffinity),
					predicates.VolumeBinding:          predicates_fake.EmptyPredicate(predicates.VolumeBinding),
					predicates.NodeScheduler:          predicates_fake.EmptyPredicate(predicates.NodeScheduler),
				},
			},
			wantErr: true,
			wantedErrorData: []string{
				"Scheduling conditions were not met for pod ns1/p1:",
				"PodAffinity: failed pre-predicate PodAffinity. Reasons: reason1, reason2",
				"PodFitsHostPorts: failed pre-predicate PodFitsHostPorts. Reasons: reason1, reason2",
			},
		},
	}
	for _, tt := range tests {
		t.Logf("Running test: %s", tt.name)
		t.Run(tt.name, func(t *testing.T) {
			skipPredicates := SkipPredicates{}
			got := evaluateTaskOnPrePredicate(tt.args.task, tt.args.k8sPredicates, skipPredicates)

			// I use this weird way to compare the 2 errors because sometimes the line order in
			// the returning error will change
			if (got != nil) != tt.wantErr {
				t.Errorf("evaluateTaskOnPrePredicate() = %v, want %v", got,
					strings.Join(tt.wantedErrorData, "\n"))
			}
			if got != nil {
				if tt.wantErr {
					gotErrorData := strings.Split(got.Error(), "\n")
					gotErrorData = gotErrorData[:len(gotErrorData)-1]
					sort.Strings(gotErrorData)

					wantedErrorData := make([]string, len(tt.wantedErrorData))
					copy(wantedErrorData, tt.wantedErrorData)
					sort.Strings(wantedErrorData)

					if !reflect.DeepEqual(gotErrorData, wantedErrorData) {
						t.Errorf("evaluateTaskOnPrePredicate(), Got:\n%v\nExpected:\n%v", got,
							strings.Join(tt.wantedErrorData, "\n"))
					}
				} else {
					t.Errorf("evaluateTaskOnPrePredicate() = %v, want %v", got, nil)
				}
			}
		})
	}
}

func Test_predicatesPlugin_evaluateTaskOnPredicates(t *testing.T) {
	type args struct {
		taskName                                 common_info.PodID
		jobName                                  common_info.PodGroupID
		nodeName                                 string
		storageSchedulingEnabled                 bool
		isNonPreemptableTaskOnNodeOverCapacityFn api.IsTaskAllocationOverCapacityFn
		k8sPredicates                            k8s_internal.SessionPredicates
	}
	type clusterData struct {
		jobs                            []*jobs_fake.TestJobBasic
		nodes                           map[string]nodes_fake.TestNodeBasic
		isRestrictNodeSchedulingEnabled func() bool
	}
	tests := []struct {
		name        string
		args        args
		clusterData clusterData
		err         error
	}{
		{
			"Empty predicates",
			args{
				taskName:                                 "j1-0",
				jobName:                                  "j1",
				nodeName:                                 "n1",
				storageSchedulingEnabled:                 false,
				isNonPreemptableTaskOnNodeOverCapacityFn: isNonPreemptableTaskOnNodeOverCapacityFnAlwaysSchedulable,
				k8sPredicates: k8s_internal.SessionPredicates{
					predicates.PodFitsHostPorts:       predicates_fake.EmptyPredicate(predicates.PodFitsHostPorts),
					predicates.PodToleratesNodeTaints: predicates_fake.EmptyPredicate(predicates.PodToleratesNodeTaints),
					predicates.NodeAffinity:           predicates_fake.EmptyPredicate(predicates.NodeAffinity),
					predicates.PodAffinity:            predicates_fake.EmptyPredicate(predicates.PodAffinity),
					predicates.VolumeBinding:          predicates_fake.EmptyPredicate(predicates.VolumeBinding),
					predicates.NodeScheduler:          predicates_fake.EmptyPredicate(predicates.NodeScheduler),
				},
			},
			clusterData{
				jobs: []*jobs_fake.TestJobBasic{
					{
						Name: "j1",
						Tasks: []*tasks_fake.TestTaskBasic{
							{
								NodeName: "n1",
							},
						},
					},
				},
				nodes: map[string]nodes_fake.TestNodeBasic{
					"n1": {},
				},
				isRestrictNodeSchedulingEnabled: func() bool {
					return false
				},
			},
			nil,
		},
		{
			"fail on non-preemptible over capacity",
			args{
				taskName:                                 "j1-0",
				jobName:                                  "j1",
				nodeName:                                 "n1",
				storageSchedulingEnabled:                 false,
				isNonPreemptableTaskOnNodeOverCapacityFn: isNonPreemptableTaskOnNodeOverCapacityFnAlwaysUnschedulable,
				k8sPredicates: k8s_internal.SessionPredicates{
					predicates.PodFitsHostPorts:       predicates_fake.EmptyPredicate(predicates.PodFitsHostPorts),
					predicates.PodToleratesNodeTaints: predicates_fake.EmptyPredicate(predicates.PodToleratesNodeTaints),
					predicates.NodeAffinity:           predicates_fake.EmptyPredicate(predicates.NodeAffinity),
					predicates.PodAffinity:            predicates_fake.EmptyPredicate(predicates.PodAffinity),
					predicates.VolumeBinding:          predicates_fake.EmptyPredicate(predicates.VolumeBinding),
					predicates.NodeScheduler:          predicates_fake.EmptyPredicate(predicates.NodeScheduler),
				},
			},
			clusterData{
				jobs: []*jobs_fake.TestJobBasic{
					{
						Name: "j1",
						Tasks: []*tasks_fake.TestTaskBasic{
							{
								NodeName: "n1",
							},
						},
					},
				},
				nodes: map[string]nodes_fake.TestNodeBasic{
					"n1": {},
				},
				isRestrictNodeSchedulingEnabled: func() bool {
					return false
				},
			},
			common_info.NewFitError("j1-0", "", "n1",
				"node does not have enough capacity. Reason: overcapacity, Details: custom error details"),
		},
		{
			"node resources type predicate - Whole GPU task on MIG node with mixed strategy",
			args{
				taskName:                                 "j1-0",
				jobName:                                  "j1",
				nodeName:                                 "n1",
				storageSchedulingEnabled:                 false,
				isNonPreemptableTaskOnNodeOverCapacityFn: isNonPreemptableTaskOnNodeOverCapacityFnAlwaysSchedulable,
				k8sPredicates: k8s_internal.SessionPredicates{
					predicates.PodFitsHostPorts:       predicates_fake.EmptyPredicate(predicates.PodFitsHostPorts),
					predicates.PodToleratesNodeTaints: predicates_fake.EmptyPredicate(predicates.PodToleratesNodeTaints),
					predicates.NodeAffinity:           predicates_fake.EmptyPredicate(predicates.NodeAffinity),
					predicates.PodAffinity:            predicates_fake.EmptyPredicate(predicates.PodAffinity),
					predicates.VolumeBinding:          predicates_fake.EmptyPredicate(predicates.VolumeBinding),
					predicates.NodeScheduler:          predicates_fake.EmptyPredicate(predicates.NodeScheduler),
				},
			},
			clusterData{
				jobs: []*jobs_fake.TestJobBasic{
					{
						Name:                "j1",
						RequiredGPUsPerTask: 1,
						Tasks: []*tasks_fake.TestTaskBasic{
							{
								NodeName: "n1",
							},
						},
					},
				},
				nodes: map[string]nodes_fake.TestNodeBasic{
					"n1": {
						MigInstances: map[v1.ResourceName]int{
							"nvidia.com/mig-1g.5gb": 1,
						},
						MigStrategy: node_info.MigStrategyMixed,
					},
				},
				isRestrictNodeSchedulingEnabled: func() bool {
					return false
				},
			},
			common_info.NewFitError("j1-0", "", "n1",
				"non MIG jobs cant run on a node with MigStrategy='mixed'"),
		},
		{
			"node resources type predicate - Fraction task on MIG node with mixed strategy",
			args{
				taskName:                                 "j1-0",
				jobName:                                  "j1",
				nodeName:                                 "n1",
				storageSchedulingEnabled:                 false,
				isNonPreemptableTaskOnNodeOverCapacityFn: isNonPreemptableTaskOnNodeOverCapacityFnAlwaysSchedulable,
				k8sPredicates: k8s_internal.SessionPredicates{
					predicates.PodFitsHostPorts:       predicates_fake.EmptyPredicate(predicates.PodFitsHostPorts),
					predicates.PodToleratesNodeTaints: predicates_fake.EmptyPredicate(predicates.PodToleratesNodeTaints),
					predicates.NodeAffinity:           predicates_fake.EmptyPredicate(predicates.NodeAffinity),
					predicates.PodAffinity:            predicates_fake.EmptyPredicate(predicates.PodAffinity),
					predicates.VolumeBinding:          predicates_fake.EmptyPredicate(predicates.VolumeBinding),
					predicates.NodeScheduler:          predicates_fake.EmptyPredicate(predicates.NodeScheduler),
				},
			},
			clusterData{
				jobs: []*jobs_fake.TestJobBasic{
					{
						Name:                "j1",
						RequiredGPUsPerTask: 0.5,
						Tasks: []*tasks_fake.TestTaskBasic{
							{
								NodeName: "n1",
							},
						},
					},
				},
				nodes: map[string]nodes_fake.TestNodeBasic{
					"n1": {
						MigInstances: map[v1.ResourceName]int{
							"nvidia.com/mig-1g.5gb": 1,
						},
						MigStrategy: node_info.MigStrategyMixed,
					},
				},
				isRestrictNodeSchedulingEnabled: func() bool {
					return false
				},
			},
			common_info.NewFitError("j1-0", "", "n1",
				"non MIG jobs cant run on a node with MigStrategy='mixed'"),
		},
		{
			"node resources type predicate - GPU memory task on MIG node with mixed strategy",
			args{
				taskName:                                 "j1-0",
				jobName:                                  "j1",
				nodeName:                                 "n1",
				storageSchedulingEnabled:                 false,
				isNonPreemptableTaskOnNodeOverCapacityFn: isNonPreemptableTaskOnNodeOverCapacityFnAlwaysSchedulable,
				k8sPredicates: k8s_internal.SessionPredicates{
					predicates.PodFitsHostPorts:       predicates_fake.EmptyPredicate(predicates.PodFitsHostPorts),
					predicates.PodToleratesNodeTaints: predicates_fake.EmptyPredicate(predicates.PodToleratesNodeTaints),
					predicates.NodeAffinity:           predicates_fake.EmptyPredicate(predicates.NodeAffinity),
					predicates.PodAffinity:            predicates_fake.EmptyPredicate(predicates.PodAffinity),
					predicates.VolumeBinding:          predicates_fake.EmptyPredicate(predicates.VolumeBinding),
					predicates.NodeScheduler:          predicates_fake.EmptyPredicate(predicates.NodeScheduler),
				},
			},
			clusterData{
				jobs: []*jobs_fake.TestJobBasic{
					{
						Name:              "j1",
						RequiredGpuMemory: 500,
						Tasks: []*tasks_fake.TestTaskBasic{
							{
								NodeName: "n1",
							},
						},
					},
				},
				nodes: map[string]nodes_fake.TestNodeBasic{
					"n1": {
						MigInstances: map[v1.ResourceName]int{
							"nvidia.com/mig-1g.5gb": 1,
						},
						MigStrategy: node_info.MigStrategyMixed,
					},
				},
				isRestrictNodeSchedulingEnabled: func() bool {
					return false
				},
			},
			common_info.NewFitError("j1-0", "", "n1",
				"non MIG jobs cant run on a node with MigStrategy='mixed'"),
		},
		{
			"node resources type predicate - MIG task on MIG node with mixed strategy",
			args{
				taskName:                                 "j1-0",
				jobName:                                  "j1",
				nodeName:                                 "n1",
				storageSchedulingEnabled:                 false,
				isNonPreemptableTaskOnNodeOverCapacityFn: isNonPreemptableTaskOnNodeOverCapacityFnAlwaysSchedulable,
				k8sPredicates: k8s_internal.SessionPredicates{
					predicates.PodFitsHostPorts:       predicates_fake.EmptyPredicate(predicates.PodFitsHostPorts),
					predicates.PodToleratesNodeTaints: predicates_fake.EmptyPredicate(predicates.PodToleratesNodeTaints),
					predicates.NodeAffinity:           predicates_fake.EmptyPredicate(predicates.NodeAffinity),
					predicates.PodAffinity:            predicates_fake.EmptyPredicate(predicates.PodAffinity),
					predicates.VolumeBinding:          predicates_fake.EmptyPredicate(predicates.VolumeBinding),
					predicates.NodeScheduler:          predicates_fake.EmptyPredicate(predicates.NodeScheduler),
				},
			},
			clusterData{
				jobs: []*jobs_fake.TestJobBasic{
					{
						Name: "j1",
						Tasks: []*tasks_fake.TestTaskBasic{
							{
								RequiredMigInstances: map[v1.ResourceName]int{
									"nvidia.com/mig-1g.5gb": 1,
								},
								NodeName: "n1",
							},
						},
					},
				},
				nodes: map[string]nodes_fake.TestNodeBasic{
					"n1": {
						MigInstances: map[v1.ResourceName]int{
							"nvidia.com/mig-1g.5gb": 1,
						},
						MigStrategy: node_info.MigStrategyMixed,
					},
				},
				isRestrictNodeSchedulingEnabled: func() bool {
					return false
				},
			},
			nil,
		},
		{
			"node resources type predicate - CPU task on MIG node with mixed strategy",
			args{
				taskName:                                 "j1-0",
				jobName:                                  "j1",
				nodeName:                                 "n1",
				storageSchedulingEnabled:                 false,
				isNonPreemptableTaskOnNodeOverCapacityFn: isNonPreemptableTaskOnNodeOverCapacityFnAlwaysSchedulable,
				k8sPredicates: k8s_internal.SessionPredicates{
					predicates.PodFitsHostPorts:       predicates_fake.EmptyPredicate(predicates.PodFitsHostPorts),
					predicates.PodToleratesNodeTaints: predicates_fake.EmptyPredicate(predicates.PodToleratesNodeTaints),
					predicates.NodeAffinity:           predicates_fake.EmptyPredicate(predicates.NodeAffinity),
					predicates.PodAffinity:            predicates_fake.EmptyPredicate(predicates.PodAffinity),
					predicates.VolumeBinding:          predicates_fake.EmptyPredicate(predicates.VolumeBinding),
					predicates.NodeScheduler:          predicates_fake.EmptyPredicate(predicates.NodeScheduler),
				},
			},
			clusterData{
				jobs: []*jobs_fake.TestJobBasic{
					{
						Name:                "j1",
						RequiredCPUsPerTask: 1000,
						Tasks: []*tasks_fake.TestTaskBasic{
							{
								NodeName: "n1",
							},
						},
					},
				},
				nodes: map[string]nodes_fake.TestNodeBasic{
					"n1": {
						MigInstances: map[v1.ResourceName]int{
							"nvidia.com/mig-1g.5gb": 1,
						},
						MigStrategy: node_info.MigStrategyMixed,
					},
				},
				isRestrictNodeSchedulingEnabled: func() bool {
					return false
				},
			},
			nil,
		},
		{
			"node resources type predicate - Whole GPU task on MIG node with single strategy",
			args{
				taskName:                                 "j1-0",
				jobName:                                  "j1",
				nodeName:                                 "n1",
				storageSchedulingEnabled:                 false,
				isNonPreemptableTaskOnNodeOverCapacityFn: isNonPreemptableTaskOnNodeOverCapacityFnAlwaysSchedulable,
				k8sPredicates: k8s_internal.SessionPredicates{
					predicates.PodFitsHostPorts:       predicates_fake.EmptyPredicate(predicates.PodFitsHostPorts),
					predicates.PodToleratesNodeTaints: predicates_fake.EmptyPredicate(predicates.PodToleratesNodeTaints),
					predicates.NodeAffinity:           predicates_fake.EmptyPredicate(predicates.NodeAffinity),
					predicates.PodAffinity:            predicates_fake.EmptyPredicate(predicates.PodAffinity),
					predicates.VolumeBinding:          predicates_fake.EmptyPredicate(predicates.VolumeBinding),
					predicates.NodeScheduler:          predicates_fake.EmptyPredicate(predicates.NodeScheduler),
				},
			},
			clusterData{
				jobs: []*jobs_fake.TestJobBasic{
					{
						Name:                "j1",
						RequiredGPUsPerTask: 1,
						Tasks: []*tasks_fake.TestTaskBasic{
							{
								NodeName: "n1",
							},
						},
					},
				},
				nodes: map[string]nodes_fake.TestNodeBasic{
					"n1": {
						GPUs:        7,
						MigStrategy: node_info.MigStrategySingle,
					},
				},
				isRestrictNodeSchedulingEnabled: func() bool {
					return false
				},
			},
			nil,
		},
		{
			"node resources type predicate - Fraction task on MIG node with single strategy",
			args{
				taskName:                                 "j1-0",
				jobName:                                  "j1",
				nodeName:                                 "n1",
				storageSchedulingEnabled:                 false,
				isNonPreemptableTaskOnNodeOverCapacityFn: isNonPreemptableTaskOnNodeOverCapacityFnAlwaysSchedulable,
				k8sPredicates: k8s_internal.SessionPredicates{
					predicates.PodFitsHostPorts:       predicates_fake.EmptyPredicate(predicates.PodFitsHostPorts),
					predicates.PodToleratesNodeTaints: predicates_fake.EmptyPredicate(predicates.PodToleratesNodeTaints),
					predicates.NodeAffinity:           predicates_fake.EmptyPredicate(predicates.NodeAffinity),
					predicates.PodAffinity:            predicates_fake.EmptyPredicate(predicates.PodAffinity),
					predicates.VolumeBinding:          predicates_fake.EmptyPredicate(predicates.VolumeBinding),
					predicates.NodeScheduler:          predicates_fake.EmptyPredicate(predicates.NodeScheduler),
				},
			},
			clusterData{
				jobs: []*jobs_fake.TestJobBasic{
					{
						Name:                "j1",
						RequiredGPUsPerTask: 0.5,
						Tasks: []*tasks_fake.TestTaskBasic{
							{
								NodeName: "n1",
							},
						},
					},
				},
				nodes: map[string]nodes_fake.TestNodeBasic{
					"n1": {
						GPUs:        7,
						MigStrategy: node_info.MigStrategySingle,
					},
				},
				isRestrictNodeSchedulingEnabled: func() bool {
					return false
				},
			},
			common_info.NewFitError("j1-0", "", "n1",
				"only whole gpu jobs can run on a node with MigStrategy='single'"),
		},
		{
			"node resources type predicate - GPU memory task on MIG node with single strategy",
			args{
				taskName:                                 "j1-0",
				jobName:                                  "j1",
				nodeName:                                 "n1",
				storageSchedulingEnabled:                 false,
				isNonPreemptableTaskOnNodeOverCapacityFn: isNonPreemptableTaskOnNodeOverCapacityFnAlwaysSchedulable,
				k8sPredicates: k8s_internal.SessionPredicates{
					predicates.PodFitsHostPorts:       predicates_fake.EmptyPredicate(predicates.PodFitsHostPorts),
					predicates.PodToleratesNodeTaints: predicates_fake.EmptyPredicate(predicates.PodToleratesNodeTaints),
					predicates.NodeAffinity:           predicates_fake.EmptyPredicate(predicates.NodeAffinity),
					predicates.PodAffinity:            predicates_fake.EmptyPredicate(predicates.PodAffinity),
					predicates.VolumeBinding:          predicates_fake.EmptyPredicate(predicates.VolumeBinding),
					predicates.NodeScheduler:          predicates_fake.EmptyPredicate(predicates.NodeScheduler),
				},
			},
			clusterData{
				jobs: []*jobs_fake.TestJobBasic{
					{
						Name:              "j1",
						RequiredGpuMemory: 500,
						Tasks: []*tasks_fake.TestTaskBasic{
							{
								NodeName: "n1",
							},
						},
					},
				},
				nodes: map[string]nodes_fake.TestNodeBasic{
					"n1": {
						GPUs:        7,
						MigStrategy: node_info.MigStrategySingle,
					},
				},
				isRestrictNodeSchedulingEnabled: func() bool {
					return false
				},
			},
			common_info.NewFitError("j1-0", "", "n1",
				"only whole gpu jobs can run on a node with MigStrategy='single'"),
		},
		{
			"node resources type predicate - MIG task on MIG node with single strategy",
			args{
				taskName:                                 "j1-0",
				jobName:                                  "j1",
				nodeName:                                 "n1",
				storageSchedulingEnabled:                 false,
				isNonPreemptableTaskOnNodeOverCapacityFn: isNonPreemptableTaskOnNodeOverCapacityFnAlwaysSchedulable,
				k8sPredicates: k8s_internal.SessionPredicates{
					predicates.PodFitsHostPorts:       predicates_fake.EmptyPredicate(predicates.PodFitsHostPorts),
					predicates.PodToleratesNodeTaints: predicates_fake.EmptyPredicate(predicates.PodToleratesNodeTaints),
					predicates.NodeAffinity:           predicates_fake.EmptyPredicate(predicates.NodeAffinity),
					predicates.PodAffinity:            predicates_fake.EmptyPredicate(predicates.PodAffinity),
					predicates.VolumeBinding:          predicates_fake.EmptyPredicate(predicates.VolumeBinding),
					predicates.NodeScheduler:          predicates_fake.EmptyPredicate(predicates.NodeScheduler),
				},
			},
			clusterData{
				jobs: []*jobs_fake.TestJobBasic{
					{
						Name: "j1",
						Tasks: []*tasks_fake.TestTaskBasic{
							{
								RequiredMigInstances: map[v1.ResourceName]int{
									"nvidia.com/mig-1g.5gb": 1,
								},
								NodeName: "n1",
							},
						},
					},
				},
				nodes: map[string]nodes_fake.TestNodeBasic{
					"n1": {
						GPUs:        7,
						MigStrategy: node_info.MigStrategySingle,
					},
				},
				isRestrictNodeSchedulingEnabled: func() bool {
					return false
				},
			},
			common_info.NewFitError("j1-0", "", "n1",
				"only whole gpu jobs can run on a node with MigStrategy='single'"),
		},
		{
			"node resources type predicate - CPU task on MIG node with single strategy",
			args{
				taskName:                                 "j1-0",
				jobName:                                  "j1",
				nodeName:                                 "n1",
				storageSchedulingEnabled:                 false,
				isNonPreemptableTaskOnNodeOverCapacityFn: isNonPreemptableTaskOnNodeOverCapacityFnAlwaysSchedulable,
				k8sPredicates: k8s_internal.SessionPredicates{
					predicates.PodFitsHostPorts:       predicates_fake.EmptyPredicate(predicates.PodFitsHostPorts),
					predicates.PodToleratesNodeTaints: predicates_fake.EmptyPredicate(predicates.PodToleratesNodeTaints),
					predicates.NodeAffinity:           predicates_fake.EmptyPredicate(predicates.NodeAffinity),
					predicates.PodAffinity:            predicates_fake.EmptyPredicate(predicates.PodAffinity),
					predicates.VolumeBinding:          predicates_fake.EmptyPredicate(predicates.VolumeBinding),
					predicates.NodeScheduler:          predicates_fake.EmptyPredicate(predicates.NodeScheduler),
				},
			},
			clusterData{
				jobs: []*jobs_fake.TestJobBasic{
					{
						Name:                "j1",
						RequiredCPUsPerTask: 1000,
						Tasks: []*tasks_fake.TestTaskBasic{
							{
								NodeName: "n1",
							},
						},
					},
				},
				nodes: map[string]nodes_fake.TestNodeBasic{
					"n1": {
						GPUs:        7,
						MigStrategy: node_info.MigStrategySingle,
					},
				},
				isRestrictNodeSchedulingEnabled: func() bool {
					return false
				},
			},
			nil,
		},
		{
			"node resources type predicate - Whole GPU task on non MIG node",
			args{
				taskName:                                 "j1-0",
				jobName:                                  "j1",
				nodeName:                                 "n1",
				storageSchedulingEnabled:                 false,
				isNonPreemptableTaskOnNodeOverCapacityFn: isNonPreemptableTaskOnNodeOverCapacityFnAlwaysSchedulable,
				k8sPredicates: k8s_internal.SessionPredicates{
					predicates.PodFitsHostPorts:       predicates_fake.EmptyPredicate(predicates.PodFitsHostPorts),
					predicates.PodToleratesNodeTaints: predicates_fake.EmptyPredicate(predicates.PodToleratesNodeTaints),
					predicates.NodeAffinity:           predicates_fake.EmptyPredicate(predicates.NodeAffinity),
					predicates.PodAffinity:            predicates_fake.EmptyPredicate(predicates.PodAffinity),
					predicates.VolumeBinding:          predicates_fake.EmptyPredicate(predicates.VolumeBinding),
					predicates.NodeScheduler:          predicates_fake.EmptyPredicate(predicates.NodeScheduler),
				},
			},
			clusterData{
				jobs: []*jobs_fake.TestJobBasic{
					{
						Name:                "j1",
						RequiredGPUsPerTask: 1,
						Tasks: []*tasks_fake.TestTaskBasic{
							{
								NodeName: "n1",
							},
						},
					},
				},
				nodes: map[string]nodes_fake.TestNodeBasic{
					"n1": {
						GPUs: 8,
					},
				},
				isRestrictNodeSchedulingEnabled: func() bool {
					return false
				},
			},
			nil,
		},
		{
			"node resources type predicate - Fraction task on non MIG node",
			args{
				taskName:                                 "j1-0",
				jobName:                                  "j1",
				nodeName:                                 "n1",
				storageSchedulingEnabled:                 false,
				isNonPreemptableTaskOnNodeOverCapacityFn: isNonPreemptableTaskOnNodeOverCapacityFnAlwaysSchedulable,
				k8sPredicates: k8s_internal.SessionPredicates{
					predicates.PodFitsHostPorts:       predicates_fake.EmptyPredicate(predicates.PodFitsHostPorts),
					predicates.PodToleratesNodeTaints: predicates_fake.EmptyPredicate(predicates.PodToleratesNodeTaints),
					predicates.NodeAffinity:           predicates_fake.EmptyPredicate(predicates.NodeAffinity),
					predicates.PodAffinity:            predicates_fake.EmptyPredicate(predicates.PodAffinity),
					predicates.VolumeBinding:          predicates_fake.EmptyPredicate(predicates.VolumeBinding),
					predicates.NodeScheduler:          predicates_fake.EmptyPredicate(predicates.NodeScheduler),
				},
			},
			clusterData{
				jobs: []*jobs_fake.TestJobBasic{
					{
						Name:                "j1",
						RequiredGPUsPerTask: 0.5,
						Tasks: []*tasks_fake.TestTaskBasic{
							{
								NodeName: "n1",
							},
						},
					},
				},
				nodes: map[string]nodes_fake.TestNodeBasic{
					"n1": {
						GPUs: 8,
					},
				},
				isRestrictNodeSchedulingEnabled: func() bool {
					return false
				},
			},
			nil,
		},
		{
			"node resources type predicate - GPU memory task on non MIG node",
			args{
				taskName:                                 "j1-0",
				jobName:                                  "j1",
				nodeName:                                 "n1",
				storageSchedulingEnabled:                 false,
				isNonPreemptableTaskOnNodeOverCapacityFn: isNonPreemptableTaskOnNodeOverCapacityFnAlwaysSchedulable,
				k8sPredicates: k8s_internal.SessionPredicates{
					predicates.PodFitsHostPorts:       predicates_fake.EmptyPredicate(predicates.PodFitsHostPorts),
					predicates.PodToleratesNodeTaints: predicates_fake.EmptyPredicate(predicates.PodToleratesNodeTaints),
					predicates.NodeAffinity:           predicates_fake.EmptyPredicate(predicates.NodeAffinity),
					predicates.PodAffinity:            predicates_fake.EmptyPredicate(predicates.PodAffinity),
					predicates.VolumeBinding:          predicates_fake.EmptyPredicate(predicates.VolumeBinding),
					predicates.NodeScheduler:          predicates_fake.EmptyPredicate(predicates.NodeScheduler),
				},
			},
			clusterData{
				jobs: []*jobs_fake.TestJobBasic{
					{
						Name:              "j1",
						RequiredGpuMemory: 500,
						Tasks: []*tasks_fake.TestTaskBasic{
							{
								NodeName: "n1",
							},
						},
					},
				},
				nodes: map[string]nodes_fake.TestNodeBasic{
					"n1": {
						GPUs: 8,
					},
				},
				isRestrictNodeSchedulingEnabled: func() bool {
					return false
				},
			},
			nil,
		},
		{
			"node resources type predicate - MIG task on non MIG node",
			args{
				taskName:                                 "j1-0",
				jobName:                                  "j1",
				nodeName:                                 "n1",
				storageSchedulingEnabled:                 false,
				isNonPreemptableTaskOnNodeOverCapacityFn: isNonPreemptableTaskOnNodeOverCapacityFnAlwaysSchedulable,
				k8sPredicates: k8s_internal.SessionPredicates{
					predicates.PodFitsHostPorts:       predicates_fake.EmptyPredicate(predicates.PodFitsHostPorts),
					predicates.PodToleratesNodeTaints: predicates_fake.EmptyPredicate(predicates.PodToleratesNodeTaints),
					predicates.NodeAffinity:           predicates_fake.EmptyPredicate(predicates.NodeAffinity),
					predicates.PodAffinity:            predicates_fake.EmptyPredicate(predicates.PodAffinity),
					predicates.VolumeBinding:          predicates_fake.EmptyPredicate(predicates.VolumeBinding),
					predicates.NodeScheduler:          predicates_fake.EmptyPredicate(predicates.NodeScheduler),
				},
			},
			clusterData{
				jobs: []*jobs_fake.TestJobBasic{
					{
						Name: "j1",
						Tasks: []*tasks_fake.TestTaskBasic{
							{
								RequiredMigInstances: map[v1.ResourceName]int{
									"nvidia.com/mig-1g.5gb": 1,
								},
								NodeName: "n1",
							},
						},
					},
				},
				nodes: map[string]nodes_fake.TestNodeBasic{
					"n1": {
						GPUs: 8,
					},
				},
				isRestrictNodeSchedulingEnabled: func() bool {
					return false
				},
			},
			common_info.NewFitError("j1-0", "", "n1",
				"MIG job cannot be scheduled on non-MIG node"),
		},
		{
			"node resources type predicate - CPU task on non MIG node",
			args{
				taskName:                                 "j1-0",
				jobName:                                  "j1",
				nodeName:                                 "n1",
				storageSchedulingEnabled:                 false,
				isNonPreemptableTaskOnNodeOverCapacityFn: isNonPreemptableTaskOnNodeOverCapacityFnAlwaysSchedulable,
				k8sPredicates: k8s_internal.SessionPredicates{
					predicates.PodFitsHostPorts:       predicates_fake.EmptyPredicate(predicates.PodFitsHostPorts),
					predicates.PodToleratesNodeTaints: predicates_fake.EmptyPredicate(predicates.PodToleratesNodeTaints),
					predicates.NodeAffinity:           predicates_fake.EmptyPredicate(predicates.NodeAffinity),
					predicates.PodAffinity:            predicates_fake.EmptyPredicate(predicates.PodAffinity),
					predicates.VolumeBinding:          predicates_fake.EmptyPredicate(predicates.VolumeBinding),
					predicates.NodeScheduler:          predicates_fake.EmptyPredicate(predicates.NodeScheduler),
				},
			},
			clusterData{
				jobs: []*jobs_fake.TestJobBasic{
					{
						Name:                "j1",
						RequiredCPUsPerTask: 1000,
						Tasks: []*tasks_fake.TestTaskBasic{
							{
								NodeName: "n1",
							},
						},
					},
				},
				nodes: map[string]nodes_fake.TestNodeBasic{
					"n1": {
						GPUs: 8,
					},
				},
				isRestrictNodeSchedulingEnabled: func() bool {
					return false
				},
			},
			nil,
		},
		{
			"fail on gpu memory not synced",
			args{
				taskName:                                 "j1-0",
				jobName:                                  "j1",
				nodeName:                                 "n1",
				storageSchedulingEnabled:                 false,
				isNonPreemptableTaskOnNodeOverCapacityFn: isNonPreemptableTaskOnNodeOverCapacityFnAlwaysSchedulable,
				k8sPredicates: k8s_internal.SessionPredicates{
					predicates.PodFitsHostPorts:       predicates_fake.EmptyPredicate(predicates.PodFitsHostPorts),
					predicates.PodToleratesNodeTaints: predicates_fake.EmptyPredicate(predicates.PodToleratesNodeTaints),
					predicates.NodeAffinity:           predicates_fake.EmptyPredicate(predicates.NodeAffinity),
					predicates.PodAffinity:            predicates_fake.EmptyPredicate(predicates.PodAffinity),
					predicates.VolumeBinding:          predicates_fake.EmptyPredicate(predicates.VolumeBinding),
					predicates.NodeScheduler:          predicates_fake.EmptyPredicate(predicates.NodeScheduler),
				},
			},
			clusterData{
				jobs: []*jobs_fake.TestJobBasic{
					{
						Name:              "j1",
						RequiredGpuMemory: 1024,
						Tasks: []*tasks_fake.TestTaskBasic{
							{
								NodeName: "n1",
							},
						},
					},
				},
				nodes: map[string]nodes_fake.TestNodeBasic{
					"n1": {
						GpuMemorySynced: pointer.Bool(false),
					},
				},
				isRestrictNodeSchedulingEnabled: func() bool {
					return false
				},
			},
			common_info.NewFitError("j1-0", "", "n1",
				fmt.Sprintf("node is not a gpu node or the gpu memory count on the node was not synced yet,"+
					" task: <%v/%v>, node: <%v>", "", "j1-0", "n1")),
		},
		{
			"fail on max pods - cpu only task",
			args{
				taskName:                                 "j1-0",
				jobName:                                  "j1",
				nodeName:                                 "n1",
				storageSchedulingEnabled:                 false,
				isNonPreemptableTaskOnNodeOverCapacityFn: isNonPreemptableTaskOnNodeOverCapacityFnAlwaysSchedulable,
				k8sPredicates: k8s_internal.SessionPredicates{
					predicates.PodFitsHostPorts:       predicates_fake.EmptyPredicate(predicates.PodFitsHostPorts),
					predicates.PodToleratesNodeTaints: predicates_fake.EmptyPredicate(predicates.PodToleratesNodeTaints),
					predicates.NodeAffinity:           predicates_fake.EmptyPredicate(predicates.NodeAffinity),
					predicates.PodAffinity:            predicates_fake.EmptyPredicate(predicates.PodAffinity),
					predicates.VolumeBinding:          predicates_fake.EmptyPredicate(predicates.VolumeBinding),
					predicates.NodeScheduler:          predicates_fake.EmptyPredicate(predicates.NodeScheduler),
				},
			},
			clusterData{
				jobs: []*jobs_fake.TestJobBasic{
					{
						Name: "j1",
						Tasks: []*tasks_fake.TestTaskBasic{
							{
								NodeName: "n1",
							},
						},
					},
				},
				nodes: map[string]nodes_fake.TestNodeBasic{
					"n1": {
						MaxTaskNum: pointer.Int(0),
					},
				},
				isRestrictNodeSchedulingEnabled: func() bool {
					return false
				},
			},
			common_info.NewFitError("j1-0", "", "n1", api.NodePodNumberExceeded),
		},
		{
			"fail on max pods - fraction task",
			args{
				taskName:                                 "j1-0",
				jobName:                                  "j1",
				nodeName:                                 "n1",
				storageSchedulingEnabled:                 false,
				isNonPreemptableTaskOnNodeOverCapacityFn: isNonPreemptableTaskOnNodeOverCapacityFnAlwaysSchedulable,
				k8sPredicates: k8s_internal.SessionPredicates{
					predicates.PodFitsHostPorts:       predicates_fake.EmptyPredicate(predicates.PodFitsHostPorts),
					predicates.PodToleratesNodeTaints: predicates_fake.EmptyPredicate(predicates.PodToleratesNodeTaints),
					predicates.NodeAffinity:           predicates_fake.EmptyPredicate(predicates.NodeAffinity),
					predicates.PodAffinity:            predicates_fake.EmptyPredicate(predicates.PodAffinity),
					predicates.VolumeBinding:          predicates_fake.EmptyPredicate(predicates.VolumeBinding),
					predicates.NodeScheduler:          predicates_fake.EmptyPredicate(predicates.NodeScheduler),
				},
			},
			clusterData{
				jobs: []*jobs_fake.TestJobBasic{
					{
						Name:                "j1",
						RequiredGPUsPerTask: 0.5,
						Tasks: []*tasks_fake.TestTaskBasic{
							{
								NodeName: "n1",
							},
						},
					},
				},
				nodes: map[string]nodes_fake.TestNodeBasic{
					"n1": {
						MaxTaskNum: pointer.Int(1),
					},
				},
				isRestrictNodeSchedulingEnabled: func() bool {
					return false
				},
			},
			common_info.NewFitError("j1-0", "", "n1", api.NodePodNumberExceeded),
		},
		{
			"fail on predicate HostPorts with error",
			args{
				taskName:                                 "j1-0",
				jobName:                                  "j1",
				nodeName:                                 "n1",
				storageSchedulingEnabled:                 false,
				isNonPreemptableTaskOnNodeOverCapacityFn: isNonPreemptableTaskOnNodeOverCapacityFnAlwaysSchedulable,
				k8sPredicates: k8s_internal.SessionPredicates{
					predicates.PodFitsHostPorts:       predicates_fake.FailingPredicateWithError(predicates.PodFitsHostPorts),
					predicates.PodToleratesNodeTaints: predicates_fake.EmptyPredicate(predicates.PodToleratesNodeTaints),
					predicates.NodeAffinity:           predicates_fake.EmptyPredicate(predicates.NodeAffinity),
					predicates.PodAffinity:            predicates_fake.EmptyPredicate(predicates.PodAffinity),
					predicates.VolumeBinding:          predicates_fake.EmptyPredicate(predicates.VolumeBinding),
					predicates.NodeScheduler:          predicates_fake.EmptyPredicate(predicates.NodeScheduler),
				},
			},
			clusterData{
				jobs: []*jobs_fake.TestJobBasic{
					{
						Name: "j1",
						Tasks: []*tasks_fake.TestTaskBasic{
							{
								NodeName: "n1",
							},
						},
					},
				},
				nodes: map[string]nodes_fake.TestNodeBasic{
					"n1": {},
				},
				isRestrictNodeSchedulingEnabled: func() bool {
					return false
				},
			},
			fmt.Errorf("failed with error predicate PodFitsHostPorts"),
		},
		{
			"fail on predicate HostPorts with error",
			args{
				taskName:                                 "j1-0",
				jobName:                                  "j1",
				nodeName:                                 "n1",
				storageSchedulingEnabled:                 false,
				isNonPreemptableTaskOnNodeOverCapacityFn: isNonPreemptableTaskOnNodeOverCapacityFnAlwaysSchedulable,
				k8sPredicates: k8s_internal.SessionPredicates{
					predicates.PodFitsHostPorts:       predicates_fake.FailingPredicate(predicates.PodFitsHostPorts),
					predicates.PodToleratesNodeTaints: predicates_fake.EmptyPredicate(predicates.PodToleratesNodeTaints),
					predicates.NodeAffinity:           predicates_fake.EmptyPredicate(predicates.NodeAffinity),
					predicates.PodAffinity:            predicates_fake.EmptyPredicate(predicates.PodAffinity),
					predicates.VolumeBinding:          predicates_fake.EmptyPredicate(predicates.VolumeBinding),
					predicates.NodeScheduler:          predicates_fake.EmptyPredicate(predicates.NodeScheduler),
				},
			},
			clusterData{
				jobs: []*jobs_fake.TestJobBasic{
					{
						Name: "j1",
						Tasks: []*tasks_fake.TestTaskBasic{
							{
								NodeName: "n1",
							},
						},
					},
				},
				nodes: map[string]nodes_fake.TestNodeBasic{
					"n1": {},
				},
				isRestrictNodeSchedulingEnabled: func() bool {
					return false
				},
			},
			common_info.NewFitErrorByReasons("j1-0", "", "n1",
				fmt.Errorf("failed predicate PodFitsHostPorts"), "reason1", "reason2", "reason3"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pp := &predicatesPlugin{
				storageSchedulingEnabled: tt.args.storageSchedulingEnabled,
			}
			skipPredicates := SkipPredicates{}

			jobsMap, tasksMap, _ := jobs_fake.BuildJobsAndTasksMaps(tt.clusterData.jobs)
			nodesMap := nodes_fake.BuildNodesInfoMap(tt.clusterData.nodes, tasksMap)
			task := jobsMap[tt.args.jobName].GetAllPodsMap()[tt.args.taskName]
			job := jobsMap[tt.args.jobName]
			node := nodesMap[tt.args.nodeName]

			if err := pp.evaluateTaskOnPredicates(
				task, job, node, tt.args.k8sPredicates,
				tt.args.isNonPreemptableTaskOnNodeOverCapacityFn,
				tt.clusterData.isRestrictNodeSchedulingEnabled,
				skipPredicates,
			); !reflect.DeepEqual(err, tt.err) {
				t.Errorf("evaluateTaskOnPredicates() error:\n%v\nExpected: %v", err, tt.err)
			}
		})
	}
}

func isNonPreemptableTaskOnNodeOverCapacityFnAlwaysUnschedulable(
	_ *pod_info.PodInfo, _ *podgroup_info.PodGroupInfo, _ *node_info.NodeInfo,
) *api.SchedulableResult {
	return &api.SchedulableResult{
		IsSchedulable: false,
		Reason:        "overcapacity",
		Message:       "custom error details",
		Details:       nil,
	}
}

func isNonPreemptableTaskOnNodeOverCapacityFnAlwaysSchedulable(
	_ *pod_info.PodInfo, _ *podgroup_info.PodGroupInfo, _ *node_info.NodeInfo,
) *api.SchedulableResult {
	return &api.SchedulableResult{
		IsSchedulable: true,
		Reason:        "",
		Message:       "",
		Details:       nil,
	}
}
