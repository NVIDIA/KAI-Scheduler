// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package framework

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"

	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/common_info"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/eviction_info"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/pod_info"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/pod_status"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/podgroup_info"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/resource_info"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/constants"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/test_utils/jobs_fake"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/test_utils/nodes_fake"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/test_utils/tasks_fake"
)

func TestStatement_Evict_Unevict(t *testing.T) {
	type args struct {
		jobName common_info.PodGroupID
		podName common_info.PodID
	}
	type expected struct {
		jobGpuAllocation float64
		usedGpuOnNode    float64
	}
	tests := []struct {
		name         string
		testMetadata nodes_fake.TestClusterTopology
		args         args
		expected     expected
	}{
		{
			"Running pod evict/unevict",
			nodes_fake.TestClusterTopology{
				Jobs: []*jobs_fake.TestJobBasic{
					{
						Name:                "running_job0",
						RequiredGPUsPerTask: 1,
						QueueName:           "queue0",
						Priority:            constants.PriorityTrainNumber,
						Tasks: []*tasks_fake.TestTaskBasic{
							{
								State:    pod_status.Running,
								NodeName: "node0",
							},
						},
					},
				},
				Nodes: map[string]nodes_fake.TestNodeBasic{
					"node0": {
						GPUs: 2,
					},
				},
			},
			args{
				jobName: "running_job0",
				podName: "running_job0-0",
			},
			expected{
				jobGpuAllocation: 1,
				usedGpuOnNode:    1,
			},
		},
		{
			"Running fractional pod evict/unevict",
			nodes_fake.TestClusterTopology{
				Jobs: []*jobs_fake.TestJobBasic{
					{
						Name:                "running_job0",
						RequiredGPUsPerTask: 0.5,
						QueueName:           "queue0",
						Priority:            constants.PriorityTrainNumber,
						Tasks: []*tasks_fake.TestTaskBasic{
							{
								State:     pod_status.Running,
								NodeName:  "node0",
								GPUGroups: []string{"1"},
							},
						},
					},
				},
				Nodes: map[string]nodes_fake.TestNodeBasic{
					"node0": {
						GPUs: 2,
					},
				},
			},
			args{
				jobName: "running_job0",
				podName: "running_job0-0",
			},
			expected{
				jobGpuAllocation: 0.5,
				usedGpuOnNode:    0,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			jobsInfoMap, tasksToNodeMap, _ := jobs_fake.BuildJobsAndTasksMaps(tt.testMetadata.Jobs)
			nodesInfoMap := nodes_fake.BuildNodesInfoMap(tt.testMetadata.Nodes, tasksToNodeMap)
			ssn := &Session{}
			ssn.PodGroupInfos = jobsInfoMap
			ssn.Nodes = nodesInfoMap

			s := &Statement{
				operations: []Operation{},
				ssn:        ssn,
				sessionUID: "1234",
			}

			originalTask := jobsInfoMap[tt.args.jobName].GetAllPodsMap()[tt.args.podName].Clone()
			originalJob := jobsInfoMap[tt.args.jobName].Clone()
			originalNodeInfo := extractNodeAssertedInfo(nodesInfoMap[originalTask.NodeName])

			if err := s.Evict(originalTask, "eviction message", eviction_info.EvictionMetadata{
				Action:           "action",
				EvictionGangSize: 1,
			}); err != nil {
				t.Errorf("Evict() error = %v", err)
			}
			if err := s.undoOperation(0); err != nil {
				t.Errorf("unevict() error = %v", err)
			}

			actualTask := ssn.PodGroupInfos[tt.args.jobName].GetAllPodsMap()[tt.args.podName]
			assert.Equal(t, actualTask.NodeName, originalTask.NodeName)
			assert.Equal(t, actualTask.Status, originalTask.Status)
			assert.Equal(t, *actualTask.ResReq, *originalTask.ResReq)
			assert.Equal(t, actualTask.GPUGroups, originalTask.GPUGroups)

			actualJob := ssn.PodGroupInfos[tt.args.jobName]
			assert.Equal(t, *originalJob.Allocated, *actualJob.Allocated)
			assert.Equal(t, tt.expected.jobGpuAllocation, actualJob.Allocated.GPUs())

			actualNodeInfo := extractNodeAssertedInfo(nodesInfoMap[actualTask.NodeName])
			originalNodeInfo.assertEqual(t, actualNodeInfo)
			assert.Equal(t, tt.expected.usedGpuOnNode, actualNodeInfo.used.GPUs())
		})
	}
}

func TestStatement_Evict(t *testing.T) {
	type args struct {
		jobName common_info.PodGroupID
		podName common_info.PodID
	}
	type expected struct {
		err           error
		jobAllocation *resource_info.Resource
		usedOnNode    *resource_info.Resource
	}
	type originalState struct {
		task *pod_info.PodInfo
		job  *podgroup_info.PodGroupInfo
		node *nodeAssertedInfo
	}
	tests := []struct {
		name         string
		testMetadata nodes_fake.TestClusterTopology
		args         args
		expected     expected
	}{
		{
			"Pending job eviction",
			nodes_fake.TestClusterTopology{
				Jobs: []*jobs_fake.TestJobBasic{
					{
						Name:                "pending_job0",
						RequiredGPUsPerTask: 1,
						QueueName:           "queue0",
						Priority:            constants.PriorityTrainNumber,
						Tasks: []*tasks_fake.TestTaskBasic{
							{
								State: pod_status.Pending,
							},
						},
					},
				},
				Nodes: map[string]nodes_fake.TestNodeBasic{
					"node0": {
						GPUs: 2,
					},
				},
			},
			args{
				jobName: "pending_job0",
				podName: "pending_job0-0",
			},
			expected{
				err: fmt.Errorf("node doesn't exist in sesssion: <>"),
				jobAllocation: resource_info.ResourceFromResourceList(
					v1.ResourceList{
						"nvidia.com/gpu": resource.MustParse("0"),
					},
				),
				usedOnNode: resource_info.ResourceFromResourceList(
					v1.ResourceList{
						"nvidia.com/gpu": resource.MustParse("0"),
					},
				),
			},
		},
		{
			"Running job eviction",
			nodes_fake.TestClusterTopology{
				Jobs: []*jobs_fake.TestJobBasic{
					{
						Name:                "running_job0",
						RequiredGPUsPerTask: 1,
						QueueName:           "queue0",
						Priority:            constants.PriorityTrainNumber,
						Tasks: []*tasks_fake.TestTaskBasic{
							{
								State:    pod_status.Running,
								NodeName: "node0",
							},
						},
					},
				},
				Nodes: map[string]nodes_fake.TestNodeBasic{
					"node0": {
						GPUs: 2,
					},
				},
			},
			args{
				jobName: "running_job0",
				podName: "running_job0-0",
			},
			expected{
				err: nil,
				jobAllocation: resource_info.ResourceFromResourceList(
					v1.ResourceList{
						"nvidia.com/gpu": resource.MustParse("1"),
					},
				),
				usedOnNode: resource_info.ResourceFromResourceList(
					v1.ResourceList{
						"nvidia.com/gpu": resource.MustParse("1"),
					},
				),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			jobsInfoMap, tasksToNodeMap, _ := jobs_fake.BuildJobsAndTasksMaps(tt.testMetadata.Jobs)
			nodesInfoMap := nodes_fake.BuildNodesInfoMap(tt.testMetadata.Nodes, tasksToNodeMap)
			task := jobsInfoMap[tt.args.jobName].GetAllPodsMap()[tt.args.podName]

			s := &Statement{
				operations: []Operation{},
				ssn: &Session{
					PodGroupInfos: jobsInfoMap,
					Nodes:         nodesInfoMap,
				},
				sessionUID: "1234",
			}

			dataBeforeEvict := originalState{
				task: task.Clone(),
				job:  jobsInfoMap[tt.args.jobName].Clone(),
			}
			if task.NodeName != "" {
				dataBeforeEvict.node = extractNodeAssertedInfo(nodesInfoMap[task.NodeName])
			}

			err := s.Evict(task, "message", eviction_info.EvictionMetadata{
				Action:           "action",
				EvictionGangSize: 1,
			})
			if err != tt.expected.err && tt.expected.err.Error() != err.Error() {
				t.Errorf("Evict() expected error = %v, actual error %v", tt.expected.err, err)
			}
			if err != nil {
				return
			}

			validateEvictedTask(t, s.ssn, tt.args.jobName, tt.args.podName, dataBeforeEvict.task)
			validateEvictedJob(t, s.ssn, tt.args.jobName,
				dataBeforeEvict.task, dataBeforeEvict.job, tt.expected.jobAllocation)
			validateEvictedFromNode(t, nodesInfoMap[task.NodeName], dataBeforeEvict.node,
				dataBeforeEvict.task.ResReq)
		})
	}
}

func TestStatement_Evict_Undo_Undo(t *testing.T) {
	type args struct {
		jobName common_info.PodGroupID
		podName common_info.PodID
	}
	type expected struct {
		err           error
		jobAllocation *resource_info.Resource
		usedOnNode    *resource_info.Resource
	}
	type originalState struct {
		task *pod_info.PodInfo
		job  *podgroup_info.PodGroupInfo
		node *nodeAssertedInfo
	}
	tests := []struct {
		name         string
		testMetadata nodes_fake.TestClusterTopology
		args         args
		expected     expected
	}{
		{
			"Pending job eviction undo undo",
			nodes_fake.TestClusterTopology{
				Jobs: []*jobs_fake.TestJobBasic{
					{
						Name:                "pending_job0",
						RequiredGPUsPerTask: 1,
						QueueName:           "queue0",
						Priority:            constants.PriorityTrainNumber,
						Tasks: []*tasks_fake.TestTaskBasic{
							{
								State: pod_status.Pending,
							},
						},
					},
				},
				Nodes: map[string]nodes_fake.TestNodeBasic{
					"node0": {
						GPUs: 2,
					},
				},
			},
			args{
				jobName: "pending_job0",
				podName: "pending_job0-0",
			},
			expected{
				err: fmt.Errorf("node doesn't exist in sesssion: <>"),
				jobAllocation: resource_info.ResourceFromResourceList(
					v1.ResourceList{
						"nvidia.com/gpu": resource.MustParse("0"),
					},
				),
				usedOnNode: resource_info.ResourceFromResourceList(
					v1.ResourceList{
						"nvidia.com/gpu": resource.MustParse("0"),
					},
				),
			},
		},
		{
			"Running job eviction undo undo",
			nodes_fake.TestClusterTopology{
				Jobs: []*jobs_fake.TestJobBasic{
					{
						Name:                "running_job0",
						RequiredGPUsPerTask: 1,
						QueueName:           "queue0",
						Priority:            constants.PriorityTrainNumber,
						Tasks: []*tasks_fake.TestTaskBasic{
							{
								State:    pod_status.Running,
								NodeName: "node0",
							},
						},
					},
				},
				Nodes: map[string]nodes_fake.TestNodeBasic{
					"node0": {
						GPUs: 2,
					},
				},
			},
			args{
				jobName: "running_job0",
				podName: "running_job0-0",
			},
			expected{
				err: nil,
				jobAllocation: resource_info.ResourceFromResourceList(
					v1.ResourceList{
						"nvidia.com/gpu": resource.MustParse("1"),
					},
				),
				usedOnNode: resource_info.ResourceFromResourceList(
					v1.ResourceList{
						"nvidia.com/gpu": resource.MustParse("1"),
					},
				),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			jobsInfoMap, tasksToNodeMap, _ := jobs_fake.BuildJobsAndTasksMaps(tt.testMetadata.Jobs)
			nodesInfoMap := nodes_fake.BuildNodesInfoMap(tt.testMetadata.Nodes, tasksToNodeMap)
			task := jobsInfoMap[tt.args.jobName].GetAllPodsMap()[tt.args.podName]

			s := &Statement{
				operations: []Operation{},
				ssn: &Session{
					PodGroupInfos: jobsInfoMap,
					Nodes:         nodesInfoMap,
				},
				sessionUID: "1234",
			}

			dataBeforeEvict := originalState{
				task: task.Clone(),
				job:  jobsInfoMap[tt.args.jobName].Clone(),
			}
			if task.NodeName != "" {
				dataBeforeEvict.node = extractNodeAssertedInfo(nodesInfoMap[task.NodeName])
			}

			err := s.Evict(task, "message", eviction_info.EvictionMetadata{
				Action:           "action",
				EvictionGangSize: 1,
			})
			if err != tt.expected.err && tt.expected.err.Error() != err.Error() {
				t.Errorf("Evict() expected error = %v, actual error %v", tt.expected.err, err)
			}
			if err != nil {
				return
			}
			if err = s.undoOperation(0); err != nil {
				t.Errorf("unevict() error = %v", err)
			}
			if err = s.undoOperation(1); err != nil {
				t.Errorf("unevict() error = %v", err)
			}

			validateEvictedTask(t, s.ssn, tt.args.jobName, tt.args.podName, dataBeforeEvict.task)
			validateEvictedJob(t, s.ssn, tt.args.jobName,
				dataBeforeEvict.task, dataBeforeEvict.job, tt.expected.jobAllocation)
			validateEvictedFromNode(t, nodesInfoMap[task.NodeName], dataBeforeEvict.node,
				dataBeforeEvict.task.ResReq)
		})
	}
}

func TestStatement_Pipeline_Unpipeline(t *testing.T) {
	type args struct {
		jobName        common_info.PodGroupID
		podName        common_info.PodID
		nodeToPipeline string
	}
	type expected struct {
		jobGpuAllocated             float64
		usedGpuOnPipelineOriginNode float64
		usedGpuOnPipelinedNode      float64
	}
	tests := []struct {
		name         string
		testMetadata nodes_fake.TestClusterTopology
		args         args
		expected     expected
	}{
		{
			"basic pipeline/unpipeline",
			nodes_fake.TestClusterTopology{
				Jobs: []*jobs_fake.TestJobBasic{
					{
						Name:                "pending_job0",
						RequiredGPUsPerTask: 1,
						QueueName:           "queue0",
						Priority:            constants.PriorityTrainNumber,
						Tasks: []*tasks_fake.TestTaskBasic{
							{
								State: pod_status.Pending,
							},
						},
					},
					{
						Name:                "running_job0",
						RequiredGPUsPerTask: 1,
						QueueName:           "queue0",
						Priority:            constants.PriorityTrainNumber,
						Tasks: []*tasks_fake.TestTaskBasic{
							{
								State:    pod_status.Running,
								NodeName: "node0",
							},
							{
								State:    pod_status.Running,
								NodeName: "node0",
							},
						},
					},
				},
				Nodes: map[string]nodes_fake.TestNodeBasic{
					"node0": {
						GPUs: 2,
					},
				},
			},
			args{
				jobName:        "pending_job0",
				podName:        "pending_job0-0",
				nodeToPipeline: "node0",
			},
			expected{
				jobGpuAllocated:             0,
				usedGpuOnPipelineOriginNode: 0,
				usedGpuOnPipelinedNode:      2,
			},
		},
		{
			"pipeline/unpipeline for running pod",
			nodes_fake.TestClusterTopology{
				Jobs: []*jobs_fake.TestJobBasic{
					{
						Name:                "pending_job0",
						RequiredGPUsPerTask: 1,
						QueueName:           "queue0",
						Priority:            constants.PriorityTrainNumber,
						Tasks: []*tasks_fake.TestTaskBasic{
							{
								State:    pod_status.Running,
								NodeName: "node1",
							},
						},
					},
					{
						Name:                "running_job0",
						RequiredGPUsPerTask: 1,
						QueueName:           "queue0",
						Priority:            constants.PriorityTrainNumber,
						Tasks: []*tasks_fake.TestTaskBasic{
							{
								State:    pod_status.Running,
								NodeName: "node0",
							},
							{
								State:    pod_status.Running,
								NodeName: "node0",
							},
						},
					},
				},
				Nodes: map[string]nodes_fake.TestNodeBasic{
					"node0": {
						GPUs: 2,
					},
					"node1": {
						GPUs: 1,
					},
				},
			},
			args{
				jobName:        "pending_job0",
				podName:        "pending_job0-0",
				nodeToPipeline: "node0",
			},
			expected{
				jobGpuAllocated:             1,
				usedGpuOnPipelineOriginNode: 1,
				usedGpuOnPipelinedNode:      2,
			},
		},
		{
			"pipeline/unpipeline for fractional running pod",
			nodes_fake.TestClusterTopology{
				Jobs: []*jobs_fake.TestJobBasic{
					{
						Name:                "pending_job0",
						RequiredGPUsPerTask: 0.5,
						QueueName:           "queue0",
						Priority:            constants.PriorityTrainNumber,
						Tasks: []*tasks_fake.TestTaskBasic{
							{
								State:     pod_status.Running,
								NodeName:  "node1",
								GPUGroups: []string{"0"},
							},
						},
					},
					{
						Name:                "running_job0",
						RequiredGPUsPerTask: 0.5,
						QueueName:           "queue0",
						Priority:            constants.PriorityTrainNumber,
						Tasks: []*tasks_fake.TestTaskBasic{
							{
								State:     pod_status.Running,
								NodeName:  "node0",
								GPUGroups: []string{"0"},
							},
							{
								State:     pod_status.Running,
								NodeName:  "node0",
								GPUGroups: []string{"1"},
							},
						},
					},
				},
				Nodes: map[string]nodes_fake.TestNodeBasic{
					"node0": {
						GPUs: 2,
					},
					"node1": {
						GPUs: 1,
					},
				},
			},
			args{
				jobName:        "pending_job0",
				podName:        "pending_job0-0",
				nodeToPipeline: "node0",
			},
			expected{
				jobGpuAllocated:             0.5,
				usedGpuOnPipelineOriginNode: 0,
				usedGpuOnPipelinedNode:      0,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			jobsInfoMap, tasksToNodeMap, _ := jobs_fake.BuildJobsAndTasksMaps(tt.testMetadata.Jobs)
			nodesInfoMap := nodes_fake.BuildNodesInfoMap(tt.testMetadata.Nodes, tasksToNodeMap)
			ssn := &Session{}
			ssn.PodGroupInfos = jobsInfoMap
			ssn.Nodes = nodesInfoMap

			s := &Statement{
				operations: []Operation{},
				ssn:        ssn,
				sessionUID: "1234",
			}

			pipelinedTask := jobsInfoMap[tt.args.jobName].GetAllPodsMap()[tt.args.podName]

			originalPipelineJob := jobsInfoMap[tt.args.jobName].Clone()
			originalPipelineTask := pipelinedTask.Clone()

			var originalPipelinedNodeInfo *nodeAssertedInfo
			if pipelinedTask.NodeName != "" {
				originalPipelinedNodeInfo = extractNodeAssertedInfo(nodesInfoMap[pipelinedTask.NodeName])
			}
			originalPipelinedToNodeInfo := extractNodeAssertedInfo(nodesInfoMap[tt.args.nodeToPipeline])

			if err := s.Pipeline(pipelinedTask, tt.args.nodeToPipeline, pipelinedTask.NodeName != ""); err != nil {
				t.Errorf("Pipeline() error = %v", err)
			}
			if err := s.undoOperation(0); err != nil {
				t.Errorf("unpipeline() error = %v", err)
			}

			actualTask := ssn.PodGroupInfos[tt.args.jobName].GetAllPodsMap()[tt.args.podName]
			assert.Equal(t, actualTask.NodeName, originalPipelineTask.NodeName)
			assert.Equal(t, actualTask.Status, originalPipelineTask.Status)
			assert.Equal(t, *actualTask.ResReq, *originalPipelineTask.ResReq)
			assert.Equal(t, actualTask.GPUGroups, originalPipelineTask.GPUGroups)

			actualPipelinedJob := ssn.PodGroupInfos[tt.args.jobName]
			assert.Equal(t, *originalPipelineJob.Allocated, *actualPipelinedJob.Allocated)
			assert.Equal(t, tt.expected.jobGpuAllocated, actualPipelinedJob.Allocated.GPUs())

			if pipelinedTask.NodeName != "" {
				pipelinedFromNodeInfo := extractNodeAssertedInfo(nodesInfoMap[pipelinedTask.NodeName])
				originalPipelinedNodeInfo.assertEqual(t, pipelinedFromNodeInfo)
				assert.Equal(t, tt.expected.usedGpuOnPipelineOriginNode, originalPipelinedNodeInfo.used.GPUs())
			}
			pipelinedToNodeInfo := extractNodeAssertedInfo(nodesInfoMap[tt.args.nodeToPipeline])
			originalPipelinedToNodeInfo.assertEqual(t, pipelinedToNodeInfo)
			assert.Equal(t, tt.expected.usedGpuOnPipelinedNode, originalPipelinedToNodeInfo.used.GPUs())
		})
	}
}

func TestStatement_Pipeline(t *testing.T) {
	type args struct {
		jobName                  common_info.PodGroupID
		podName                  common_info.PodID
		nodeToPipeline           string
		updateTaskIfExistsOnNode bool
	}
	type expected struct {
		err           error
		jobAllocation *resource_info.Resource
		usedOnNode    *resource_info.Resource
	}
	type originalState struct {
		task     *pod_info.PodInfo
		job      *podgroup_info.PodGroupInfo
		fromNode *nodeAssertedInfo
		toNode   *nodeAssertedInfo
	}
	tests := []struct {
		name         string
		testMetadata nodes_fake.TestClusterTopology
		args         args
		expected     expected
	}{
		{
			"Pending job pipeline",
			nodes_fake.TestClusterTopology{
				Jobs: []*jobs_fake.TestJobBasic{
					{
						Name:                "pending_job0",
						RequiredGPUsPerTask: 1,
						QueueName:           "queue0",
						Priority:            constants.PriorityTrainNumber,
						Tasks: []*tasks_fake.TestTaskBasic{
							{
								State: pod_status.Pending,
							},
						},
					},
					{
						Name:                "releasing_job1",
						RequiredGPUsPerTask: 1,
						QueueName:           "queue0",
						Priority:            constants.PriorityTrainNumber,
						Tasks: []*tasks_fake.TestTaskBasic{
							{
								State:    pod_status.Releasing,
								NodeName: "node0",
							},
						},
					},
				},
				Nodes: map[string]nodes_fake.TestNodeBasic{
					"node0": {
						GPUs: 1,
					},
				},
			},
			args{
				jobName:                  "pending_job0",
				podName:                  "pending_job0-0",
				nodeToPipeline:           "node0",
				updateTaskIfExistsOnNode: true,
			},
			expected{
				err: nil,
				jobAllocation: resource_info.ResourceFromResourceList(
					v1.ResourceList{
						"nvidia.com/gpu": resource.MustParse("0"),
					},
				),
				usedOnNode: resource_info.ResourceFromResourceList(
					v1.ResourceList{
						"nvidia.com/gpu": resource.MustParse("0"),
					},
				),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			jobsInfoMap, tasksToNodeMap, _ := jobs_fake.BuildJobsAndTasksMaps(tt.testMetadata.Jobs)
			nodesInfoMap := nodes_fake.BuildNodesInfoMap(tt.testMetadata.Nodes, tasksToNodeMap)
			pipelinedTask := jobsInfoMap[tt.args.jobName].GetAllPodsMap()[tt.args.podName]

			s := &Statement{
				operations: []Operation{},
				ssn: &Session{
					PodGroupInfos: jobsInfoMap,
					Nodes:         nodesInfoMap,
				},
				sessionUID: "1234",
			}

			dataBeforeEvict := originalState{
				task:   pipelinedTask.Clone(),
				job:    jobsInfoMap[tt.args.jobName].Clone(),
				toNode: extractNodeAssertedInfo(nodesInfoMap[tt.args.nodeToPipeline]),
			}
			if pipelinedTask.NodeName != "" {
				dataBeforeEvict.fromNode = extractNodeAssertedInfo(nodesInfoMap[pipelinedTask.NodeName])
			}

			err := s.Pipeline(pipelinedTask, tt.args.nodeToPipeline, tt.args.updateTaskIfExistsOnNode)
			if err != tt.expected.err && tt.expected.err.Error() != err.Error() {
				t.Errorf("Pipeline() expected error = %v, actual error %v", tt.expected.err, err)
			}
			if err != nil {
				return
			}

			validatePipelinedTask(t, s.ssn, tt.args.jobName, tt.args.podName, tt.args.nodeToPipeline,
				dataBeforeEvict.task)
			validatePipelinedJob(t, s.ssn, tt.args.jobName, dataBeforeEvict.task, dataBeforeEvict.job,
				tt.expected.jobAllocation)

			if dataBeforeEvict.task.NodeName != tt.args.nodeToPipeline {
				validateEvictedFromNode(t, nodesInfoMap[pipelinedTask.NodeName], dataBeforeEvict.fromNode,
					dataBeforeEvict.task.ResReq)
				validatePipelinedToNode(t, nodesInfoMap[tt.args.nodeToPipeline], dataBeforeEvict.toNode,
					dataBeforeEvict.task.ResReq)
			}
		})
	}
}

func TestStatement_Pipeline_Undo_Undo(t *testing.T) {
	type args struct {
		jobName                  common_info.PodGroupID
		podName                  common_info.PodID
		nodeToPipeline           string
		updateTaskIfExistsOnNode bool
	}
	type expected struct {
		err           error
		jobAllocation *resource_info.Resource
		usedOnNode    *resource_info.Resource
	}
	type originalState struct {
		task     *pod_info.PodInfo
		job      *podgroup_info.PodGroupInfo
		fromNode *nodeAssertedInfo
		toNode   *nodeAssertedInfo
	}
	tests := []struct {
		name         string
		testMetadata nodes_fake.TestClusterTopology
		args         args
		expected     expected
	}{
		{
			"Pending job pipeline",
			nodes_fake.TestClusterTopology{
				Jobs: []*jobs_fake.TestJobBasic{
					{
						Name:                "pending_job0",
						RequiredGPUsPerTask: 1,
						QueueName:           "queue0",
						Priority:            constants.PriorityTrainNumber,
						Tasks: []*tasks_fake.TestTaskBasic{
							{
								State: pod_status.Pending,
							},
						},
					},
					{
						Name:                "releasing_job1",
						RequiredGPUsPerTask: 1,
						QueueName:           "queue0",
						Priority:            constants.PriorityTrainNumber,
						Tasks: []*tasks_fake.TestTaskBasic{
							{
								State:    pod_status.Releasing,
								NodeName: "node0",
							},
						},
					},
				},
				Nodes: map[string]nodes_fake.TestNodeBasic{
					"node0": {
						GPUs: 1,
					},
				},
			},
			args{
				jobName:                  "pending_job0",
				podName:                  "pending_job0-0",
				nodeToPipeline:           "node0",
				updateTaskIfExistsOnNode: true,
			},
			expected{
				err: nil,
				jobAllocation: resource_info.ResourceFromResourceList(
					v1.ResourceList{
						"nvidia.com/gpu": resource.MustParse("0"),
					},
				),
				usedOnNode: resource_info.ResourceFromResourceList(
					v1.ResourceList{
						"nvidia.com/gpu": resource.MustParse("0"),
					},
				),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			jobsInfoMap, tasksToNodeMap, _ := jobs_fake.BuildJobsAndTasksMaps(tt.testMetadata.Jobs)
			nodesInfoMap := nodes_fake.BuildNodesInfoMap(tt.testMetadata.Nodes, tasksToNodeMap)
			pipelinedTask := jobsInfoMap[tt.args.jobName].GetAllPodsMap()[tt.args.podName]

			s := &Statement{
				operations: []Operation{},
				ssn: &Session{
					PodGroupInfos: jobsInfoMap,
					Nodes:         nodesInfoMap,
				},
				sessionUID: "1234",
			}

			dataBeforeEvict := originalState{
				task:   pipelinedTask.Clone(),
				job:    jobsInfoMap[tt.args.jobName].Clone(),
				toNode: extractNodeAssertedInfo(nodesInfoMap[tt.args.nodeToPipeline]),
			}
			if pipelinedTask.NodeName != "" {
				dataBeforeEvict.fromNode = extractNodeAssertedInfo(nodesInfoMap[pipelinedTask.NodeName])
			}

			err := s.Pipeline(pipelinedTask, tt.args.nodeToPipeline, tt.args.updateTaskIfExistsOnNode)
			if err != tt.expected.err && tt.expected.err.Error() != err.Error() {
				t.Errorf("Pipeline() expected error = %v, actual error %v", tt.expected.err, err)
			}
			if err != nil {
				return
			}

			if err = s.undoOperation(0); err != nil {
				t.Errorf("unpipeline() error = %v", err)
			}
			if err = s.undoOperation(1); err != nil {
				t.Errorf("undo unpipeline() error = %v", err)
			}

			validatePipelinedTask(t, s.ssn, tt.args.jobName, tt.args.podName, tt.args.nodeToPipeline,
				dataBeforeEvict.task)
			validatePipelinedJob(t, s.ssn, tt.args.jobName, dataBeforeEvict.task, dataBeforeEvict.job,
				tt.expected.jobAllocation)

			if dataBeforeEvict.task.NodeName != tt.args.nodeToPipeline {
				validateEvictedFromNode(t, nodesInfoMap[pipelinedTask.NodeName], dataBeforeEvict.fromNode,
					dataBeforeEvict.task.ResReq)
				validatePipelinedToNode(t, nodesInfoMap[tt.args.nodeToPipeline], dataBeforeEvict.toNode,
					dataBeforeEvict.task.ResReq)
			}
		})
	}
}

func TestStatement_Allocate_Unallocate(t *testing.T) {
	type args struct {
		jobName        common_info.PodGroupID
		podName        common_info.PodID
		nodeToPipeline string
	}
	type expected struct {
		jobAllocated *resource_info.Resource
		usedOnNode   *resource_info.Resource
	}
	tests := []struct {
		name         string
		testMetadata nodes_fake.TestClusterTopology
		args         args
		expected     expected
	}{
		{
			"basic allocate/unallocate",
			nodes_fake.TestClusterTopology{
				Jobs: []*jobs_fake.TestJobBasic{
					{
						Name:                "pending_job0",
						RequiredGPUsPerTask: 1,
						QueueName:           "queue0",
						Priority:            constants.PriorityTrainNumber,
						Tasks: []*tasks_fake.TestTaskBasic{
							{
								State: pod_status.Pending,
							},
						},
					},
				},
				Nodes: map[string]nodes_fake.TestNodeBasic{
					"node0": {
						GPUs: 2,
					},
				},
			},
			args{
				jobName:        "pending_job0",
				podName:        "pending_job0-0",
				nodeToPipeline: "node0",
			},
			expected{
				jobAllocated: resource_info.ResourceFromResourceList(
					v1.ResourceList{
						"nvidia.com/gpu": resource.MustParse("0"),
					},
				),
				usedOnNode: resource_info.ResourceFromResourceList(
					v1.ResourceList{
						"nvidia.com/gpu": resource.MustParse("0"),
					},
				),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			jobsInfoMap, tasksToNodeMap, _ := jobs_fake.BuildJobsAndTasksMaps(tt.testMetadata.Jobs)
			nodesInfoMap := nodes_fake.BuildNodesInfoMap(tt.testMetadata.Nodes, tasksToNodeMap)
			ssn := &Session{}
			ssn.PodGroupInfos = jobsInfoMap
			ssn.Nodes = nodesInfoMap

			s := &Statement{
				operations: []Operation{},
				ssn:        ssn,
				sessionUID: "1234",
			}

			allocateTask := jobsInfoMap[tt.args.jobName].GetAllPodsMap()[tt.args.podName]

			originalAllocateJob := jobsInfoMap[tt.args.jobName].Clone()
			originalAllocateTask := allocateTask.Clone()
			originalNodeInfo := extractNodeAssertedInfo(nodesInfoMap[tt.args.nodeToPipeline])

			for i := 0; i < 10; i++ {
				if err := s.Allocate(allocateTask, tt.args.nodeToPipeline); err != nil {
					t.Errorf("Evict() error = %v", err)
				}
				if err := s.undoOperation(len(s.operations) - 1); err != nil {
					t.Errorf("unevict() error = %v", err)
				}
			}

			actualAllocatedTask := ssn.PodGroupInfos[tt.args.jobName].GetAllPodsMap()[tt.args.podName]
			assert.Equal(t, actualAllocatedTask.NodeName, originalAllocateTask.NodeName)
			assert.Equal(t, actualAllocatedTask.Status, originalAllocateTask.Status)
			assert.Equal(t, *actualAllocatedTask.ResReq, *originalAllocateTask.ResReq)
			assert.Equal(t, actualAllocatedTask.GPUGroups, originalAllocateTask.GPUGroups)

			actualAllocatedJob := ssn.PodGroupInfos[tt.args.jobName]
			assert.Equal(t, *originalAllocateJob.Allocated, *actualAllocatedJob.Allocated)
			assert.Equal(t, tt.expected.jobAllocated.GPUs(), actualAllocatedJob.Allocated.GPUs())

			actualNodeInfo := extractNodeAssertedInfo(nodesInfoMap[tt.args.nodeToPipeline])
			originalNodeInfo.assertEqual(t, actualNodeInfo)
			assert.Equal(t, tt.expected.usedOnNode.GPUs(), actualNodeInfo.used.GPUs())
		})
	}
}

func TestStatement_Allocate(t *testing.T) {
	type args struct {
		jobName                  common_info.PodGroupID
		podName                  common_info.PodID
		nodeToAllocate           string
		updateTaskIfExistsOnNode bool
	}
	type expected struct {
		err           error
		jobAllocation *resource_info.Resource
	}
	type originalState struct {
		task   *pod_info.PodInfo
		job    *podgroup_info.PodGroupInfo
		toNode *nodeAssertedInfo
	}
	tests := []struct {
		name         string
		testMetadata nodes_fake.TestClusterTopology
		args         args
		expected     expected
	}{
		{
			"Pending job allocate",
			nodes_fake.TestClusterTopology{
				Jobs: []*jobs_fake.TestJobBasic{
					{
						Name:                "pending_job0",
						RequiredGPUsPerTask: 1,
						QueueName:           "queue0",
						Priority:            constants.PriorityTrainNumber,
						Tasks: []*tasks_fake.TestTaskBasic{
							{
								State: pod_status.Pending,
							},
						},
					},
				},
				Nodes: map[string]nodes_fake.TestNodeBasic{
					"node0": {
						GPUs: 2,
					},
					"node1": {
						GPUs: 2,
					},
				},
			},
			args{
				jobName:                  "pending_job0",
				podName:                  "pending_job0-0",
				nodeToAllocate:           "node1",
				updateTaskIfExistsOnNode: true,
			},
			expected{
				err: nil,
				jobAllocation: resource_info.ResourceFromResourceList(
					v1.ResourceList{
						"nvidia.com/gpu": resource.MustParse("1"),
					},
				),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			jobsInfoMap, tasksToNodeMap, _ := jobs_fake.BuildJobsAndTasksMaps(tt.testMetadata.Jobs)
			nodesInfoMap := nodes_fake.BuildNodesInfoMap(tt.testMetadata.Nodes, tasksToNodeMap)
			allocatedTask := jobsInfoMap[tt.args.jobName].GetAllPodsMap()[tt.args.podName]

			s := &Statement{
				operations: []Operation{},
				ssn: &Session{
					PodGroupInfos: jobsInfoMap,
					Nodes:         nodesInfoMap,
				},
				sessionUID: "1234",
			}

			dataBeforeEvict := originalState{
				task:   allocatedTask.Clone(),
				job:    jobsInfoMap[tt.args.jobName].Clone(),
				toNode: extractNodeAssertedInfo(nodesInfoMap[tt.args.nodeToAllocate]),
			}

			err := s.Allocate(allocatedTask, tt.args.nodeToAllocate)
			if err != tt.expected.err &&
				(err == nil || tt.expected.err == nil || tt.expected.err.Error() != err.Error()) {
				t.Errorf("Allocate() expected error = %v, actual error %v", tt.expected.err, err)
			}
			if err != nil {
				return
			}

			validateAllocatedTask(t, s.ssn, tt.args.jobName, tt.args.podName, tt.args.nodeToAllocate,
				dataBeforeEvict.task)
			validateAllocatedJob(t, s.ssn, tt.args.jobName, dataBeforeEvict.task, dataBeforeEvict.job,
				tt.expected.jobAllocation)
			validateAllocatedToNode(t, nodesInfoMap[tt.args.nodeToAllocate], dataBeforeEvict.toNode,
				dataBeforeEvict.task.ResReq)
		})
	}
}

func TestStatement_Allocate_Undo_Undo(t *testing.T) {
	type args struct {
		jobName                  common_info.PodGroupID
		podName                  common_info.PodID
		nodeToAllocate           string
		updateTaskIfExistsOnNode bool
	}
	type expected struct {
		err           error
		jobAllocation *resource_info.Resource
	}
	type originalState struct {
		task   *pod_info.PodInfo
		job    *podgroup_info.PodGroupInfo
		toNode *nodeAssertedInfo
	}
	tests := []struct {
		name         string
		testMetadata nodes_fake.TestClusterTopology
		args         args
		expected     expected
	}{
		{
			"Pending job allocate",
			nodes_fake.TestClusterTopology{
				Jobs: []*jobs_fake.TestJobBasic{
					{
						Name:                "pending_job0",
						RequiredGPUsPerTask: 1,
						QueueName:           "queue0",
						Priority:            constants.PriorityTrainNumber,
						Tasks: []*tasks_fake.TestTaskBasic{
							{
								State: pod_status.Pending,
							},
						},
					},
				},
				Nodes: map[string]nodes_fake.TestNodeBasic{
					"node0": {
						GPUs: 2,
					},
					"node1": {
						GPUs: 2,
					},
				},
			},
			args{
				jobName:                  "pending_job0",
				podName:                  "pending_job0-0",
				nodeToAllocate:           "node1",
				updateTaskIfExistsOnNode: true,
			},
			expected{
				err: nil,
				jobAllocation: resource_info.ResourceFromResourceList(
					v1.ResourceList{
						"nvidia.com/gpu": resource.MustParse("1"),
					},
				),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			jobsInfoMap, tasksToNodeMap, _ := jobs_fake.BuildJobsAndTasksMaps(tt.testMetadata.Jobs)
			nodesInfoMap := nodes_fake.BuildNodesInfoMap(tt.testMetadata.Nodes, tasksToNodeMap)
			allocatedTask := jobsInfoMap[tt.args.jobName].GetAllPodsMap()[tt.args.podName]

			s := &Statement{
				operations: []Operation{},
				ssn: &Session{
					PodGroupInfos: jobsInfoMap,
					Nodes:         nodesInfoMap,
				},
				sessionUID: "1234",
			}

			dataBeforeEvict := originalState{
				task:   allocatedTask.Clone(),
				job:    jobsInfoMap[tt.args.jobName].Clone(),
				toNode: extractNodeAssertedInfo(nodesInfoMap[tt.args.nodeToAllocate]),
			}

			err := s.Allocate(allocatedTask, tt.args.nodeToAllocate)
			if err != tt.expected.err &&
				(err == nil || tt.expected.err == nil || tt.expected.err.Error() != err.Error()) {
				t.Errorf("Allocate() expected error = %v, actual error %v", tt.expected.err, err)
			}
			if err != nil {
				return
			}
			if err = s.undoOperation(0); err != nil {
				t.Errorf("unallocate() error = %v", err)
			}
			if err = s.undoOperation(1); err != nil {
				t.Errorf("undo unallocate() error = %v", err)
			}

			validateAllocatedTask(t, s.ssn, tt.args.jobName, tt.args.podName, tt.args.nodeToAllocate,
				dataBeforeEvict.task)
			validateAllocatedJob(t, s.ssn, tt.args.jobName, dataBeforeEvict.task, dataBeforeEvict.job,
				tt.expected.jobAllocation)
			validateAllocatedToNode(t, nodesInfoMap[tt.args.nodeToAllocate], dataBeforeEvict.toNode,
				dataBeforeEvict.task.ResReq)
		})
	}
}
