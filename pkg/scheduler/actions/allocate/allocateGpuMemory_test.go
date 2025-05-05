// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package allocate_test

import (
	"testing"

	. "go.uber.org/mock/gomock"
	"gopkg.in/h2non/gock.v1"

	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/actions/allocate"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/actions/integration_tests/integration_tests_utils"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/pod_status"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/constants"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/test_utils"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/test_utils/jobs_fake"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/test_utils/nodes_fake"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/test_utils/tasks_fake"
)

func TestHandleMemoryGPUAllocation(t *testing.T) {
	test_utils.InitTestingInfrastructure()
	controller := NewController(t)
	defer controller.Finish()
	defer gock.Off()

	testsMetadata := getMemoryGPUTestsMetadata()
	for testNumber, testMetadata := range testsMetadata {
		t.Logf("Running test %d: %s", testNumber, testMetadata.TestTopologyBasic.Name)

		ssn := test_utils.BuildSession(testMetadata.TestTopologyBasic, controller)
		allocateAction := allocate.New()
		allocateAction.Execute(ssn)

		test_utils.MatchExpectedAndRealTasks(t, testNumber, testMetadata.TestTopologyBasic, ssn)
	}
}

func getMemoryGPUTestsMetadata() []integration_tests_utils.TestTopologyMetadata {
	return []integration_tests_utils.TestTopologyMetadata{
		{
			TestTopologyBasic: test_utils.TestTopologyBasic{
				Name: "Non preemptible job requests gpu memory over deserved quota",
				Jobs: []*jobs_fake.TestJobBasic{
					{
						Name:              "pending_job-0",
						RequiredGpuMemory: 40,
						Priority:          constants.PriorityBuildNumber,
						QueueName:         "queue0",
						Tasks: []*tasks_fake.TestTaskBasic{
							{
								State:     pod_status.Pending,
								GPUGroups: []string{"0"},
							},
							{
								State:     pod_status.Pending,
								GPUGroups: []string{"0"},
							},
							{
								State:     pod_status.Pending,
								GPUGroups: []string{"0"},
							},
						},
					},
				},
				Nodes: map[string]nodes_fake.TestNodeBasic{
					"node0": {
						GPUs: 2,
					},
				},
				Queues: []test_utils.TestQueueBasic{
					{
						Name:         "queue0",
						DeservedGPUs: 1,
					},
				},
				JobExpectedResults: map[string]test_utils.TestExpectedResultBasic{
					"pending_job-0": {
						Status:       pod_status.Pending,
						GPUsAccepted: 0.8,
						GPUGroups:    []string{"0"},
					},
				},
				Mocks: &test_utils.TestMock{
					CacheRequirements: &test_utils.CacheMocking{
						NumberOfCacheBinds: 5,
					},
				},
			},
		},
		{
			TestTopologyBasic: test_utils.TestTopologyBasic{
				Name: "Preemptible job requests gpu memory over queue limit",
				Jobs: []*jobs_fake.TestJobBasic{
					{
						Name:              "pending_job-0",
						RequiredGpuMemory: 40,
						Priority:          constants.PriorityTrainNumber,
						QueueName:         "queue0",
						Tasks: []*tasks_fake.TestTaskBasic{
							{
								State:     pod_status.Pending,
								GPUGroups: []string{"0"},
							},
							{
								State:     pod_status.Pending,
								GPUGroups: []string{"0"},
							},
							{
								State:     pod_status.Pending,
								GPUGroups: []string{"0"},
							},
						},
					},
				},
				Nodes: map[string]nodes_fake.TestNodeBasic{
					"node0": {
						GPUs: 2,
					},
				},
				Queues: []test_utils.TestQueueBasic{
					{
						Name:           "queue0",
						DeservedGPUs:   1,
						MaxAllowedGPUs: 1,
					},
				},
				JobExpectedResults: map[string]test_utils.TestExpectedResultBasic{
					"pending_job-0": {
						Status:       pod_status.Pending,
						GPUsAccepted: 0.8,
						GPUGroups:    []string{"0"},
					},
				},
				Mocks: &test_utils.TestMock{
					CacheRequirements: &test_utils.CacheMocking{
						NumberOfCacheBinds: 5,
					},
				},
			},
		},
		{
			TestTopologyBasic: test_utils.TestTopologyBasic{
				Name: "Basic request gpu by memory when cluster is empty",
				Jobs: []*jobs_fake.TestJobBasic{
					{
						Name:              "pending_job-0",
						RequiredGpuMemory: 50,
						Priority:          constants.PriorityBuildNumber,
						QueueName:         "queue0",
						Tasks: []*tasks_fake.TestTaskBasic{
							{
								State: pod_status.Pending,
							},
						},
					},
				},
				Nodes: map[string]nodes_fake.TestNodeBasic{
					"node0": {
						GPUs: 1,
					},
				},
				Queues: []test_utils.TestQueueBasic{
					{
						Name:         "queue0",
						DeservedGPUs: 1,
					},
				},
				JobExpectedResults: map[string]test_utils.TestExpectedResultBasic{
					"pending_job-0": {
						NodeName:     "node0",
						GPUsAccepted: 0.5,
						Status:       pod_status.Binding,
						GPUGroups:    []string{"0"},
					},
				},
				Mocks: &test_utils.TestMock{
					CacheRequirements: &test_utils.CacheMocking{
						NumberOfCacheBinds: 5,
					},
				},
			},
		},
	}
}
