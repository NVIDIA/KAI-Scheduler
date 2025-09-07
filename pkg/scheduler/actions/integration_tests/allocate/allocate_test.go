// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package allocate_test

import (
	"testing"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
	kueuev1alpha1 "sigs.k8s.io/kueue/apis/kueue/v1alpha1"

	enginev2alpha2 "github.com/NVIDIA/KAI-scheduler/pkg/apis/scheduling/v2alpha2"
	commonconstants "github.com/NVIDIA/KAI-scheduler/pkg/common/constants"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/pod_status"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/constants"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/test_utils"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/test_utils/jobs_fake"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/test_utils/nodes_fake"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/test_utils/tasks_fake"

	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/actions/integration_tests/integration_tests_utils"
)

func TestAllocateIntegrationTest(t *testing.T) {
	integration_tests_utils.RunTests(t, getAllocateTestsMetadata())
}

func getAllocateTestsMetadata() []integration_tests_utils.TestTopologyMetadata {
	return []integration_tests_utils.TestTopologyMetadata{
		{
			TestTopologyBasic: test_utils.TestTopologyBasic{
				Name: "allocate job but does not allow to project to go over allowance",
				Jobs: []*jobs_fake.TestJobBasic{
					{
						Name:                "pending_job0",
						RequiredGPUsPerTask: 2,
						Priority:            constants.PriorityTrainNumber,
						QueueName:           "queue0",
						Tasks: []*tasks_fake.TestTaskBasic{
							{
								State: pod_status.Pending,
							},
						},
					},
					{
						Name:                "pending_job1",
						RequiredGPUsPerTask: 2,
						Priority:            constants.PriorityTrainNumber,
						QueueName:           "queue0",
						Tasks: []*tasks_fake.TestTaskBasic{
							{
								State: pod_status.Pending,
							},
						},
					},
				},
				Nodes: map[string]nodes_fake.TestNodeBasic{
					"node0": {
						GPUs: 4,
					},
				},
				Queues: []test_utils.TestQueueBasic{
					{
						Name:               "queue0",
						ParentQueue:        "department-a",
						DeservedGPUs:       2,
						GPUOverQuotaWeight: 2,
						MaxAllowedGPUs:     2,
					},
				},
				Departments: []test_utils.TestDepartmentBasic{
					{
						Name:         "department-a",
						DeservedGPUs: 2,
					},
				},
				JobExpectedResults: map[string]test_utils.TestExpectedResultBasic{
					"pending_job0": {
						NodeName:     "node0",
						GPUsRequired: 2,
						Status:       pod_status.Running,
					},
					"pending_job1": {
						GPUsRequired: 2,
						Status:       pod_status.Pending,
					},
				},
				Mocks: &test_utils.TestMock{
					CacheRequirements: &test_utils.CacheMocking{
						NumberOfCacheBinds: 2,
					},
				},
			},
			RoundsUntilMatch: 1,
		},
		{
			TestTopologyBasic: test_utils.TestTopologyBasic{
				Name: "allocate job but does not allow to department to go over allowance",
				Jobs: []*jobs_fake.TestJobBasic{
					{
						Name:                "pending_job0",
						RequiredGPUsPerTask: 2,
						Priority:            constants.PriorityTrainNumber,
						QueueName:           "queue0",
						Tasks: []*tasks_fake.TestTaskBasic{
							{
								State: pod_status.Pending,
							},
						},
					},
					{
						Name:                "pending_job1",
						RequiredGPUsPerTask: 2,
						Priority:            constants.PriorityTrainNumber,
						QueueName:           "queue0",
						Tasks: []*tasks_fake.TestTaskBasic{
							{
								State: pod_status.Pending,
							},
						},
					},
				},
				Nodes: map[string]nodes_fake.TestNodeBasic{
					"node0": {
						GPUs: 4,
					},
				},
				Queues: []test_utils.TestQueueBasic{
					{
						Name:         "queue0",
						ParentQueue:  "department-a",
						DeservedGPUs: 2,
					},
				},
				Departments: []test_utils.TestDepartmentBasic{
					{
						Name:           "department-a",
						DeservedGPUs:   2,
						MaxAllowedGPUs: 2,
					},
				},
				JobExpectedResults: map[string]test_utils.TestExpectedResultBasic{
					"pending_job0": {
						NodeName:     "node0",
						GPUsRequired: 2,
						Status:       pod_status.Running,
					},
					"pending_job1": {
						GPUsRequired: 2,
						Status:       pod_status.Pending,
					},
				},
				Mocks: &test_utils.TestMock{
					CacheRequirements: &test_utils.CacheMocking{
						NumberOfCacheBinds: 2,
					},
				},
			},
			RoundsUntilMatch: 1,
		},
		{
			TestTopologyBasic: test_utils.TestTopologyBasic{
				Name: "allocate pending jobs, allow over quota for train jobs (with interactive jobs)",
				Jobs: []*jobs_fake.TestJobBasic{
					{
						Name:                "pending_job0",
						RequiredGPUsPerTask: 1,
						Priority:            constants.PriorityBuildNumber,
						QueueName:           "queue0",
						Tasks: []*tasks_fake.TestTaskBasic{
							{
								State: pod_status.Pending,
							},
						},
					}, {
						Name:                "pending_job1",
						RequiredGPUsPerTask: 1,
						Priority:            constants.PriorityTrainNumber,
						QueueName:           "queue0",
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
				Queues: []test_utils.TestQueueBasic{
					{
						Name:         "queue0",
						DeservedGPUs: 1,
					},
					{
						Name:         "queue1",
						DeservedGPUs: 1,
					},
				},
				JobExpectedResults: map[string]test_utils.TestExpectedResultBasic{
					"pending_job0": {
						NodeName:     "node0",
						GPUsRequired: 1,
						Status:       pod_status.Running,
					},
					"pending_job1": {
						NodeName:     "node0",
						GPUsRequired: 1,
						Status:       pod_status.Running,
					},
				},
				Mocks: &test_utils.TestMock{
					CacheRequirements: &test_utils.CacheMocking{
						NumberOfCacheBinds: 2,
					},
				},
			},
			RoundsUntilMatch: 1,
		},
		{
			TestTopologyBasic: test_utils.TestTopologyBasic{
				Name: "don't allocate over quota build jobs",
				Jobs: []*jobs_fake.TestJobBasic{
					{
						Name:                "pending_job0",
						RequiredGPUsPerTask: 2,
						Priority:            constants.PriorityBuildNumber,
						QueueName:           "queue0",
						Tasks: []*tasks_fake.TestTaskBasic{
							{
								State: pod_status.Pending,
							},
						},
					},
				},
				Nodes: map[string]nodes_fake.TestNodeBasic{
					"node0": {
						GPUs: 4,
					},
				},
				Queues: []test_utils.TestQueueBasic{
					{
						Name:         "queue0",
						DeservedGPUs: 1,
					},
					{
						Name:         "queue1",
						DeservedGPUs: 1,
					},
				},
				JobExpectedResults: map[string]test_utils.TestExpectedResultBasic{
					"pending_job0": {
						GPUsRequired: 2,
						Status:       pod_status.Pending,
					},
				},
			},
			RoundsUntilMatch: 1,
		},
		{
			TestTopologyBasic: test_utils.TestTopologyBasic{
				Name: "allocate according to creation time when share is equal",
				Jobs: []*jobs_fake.TestJobBasic{
					{
						Name:                "pending_job0",
						RequiredGPUsPerTask: 1,
						Priority:            constants.PriorityBuildNumber,
						QueueName:           "queue0",
						Tasks: []*tasks_fake.TestTaskBasic{
							{
								State: pod_status.Pending,
							},
						},
					}, {
						Name:                "pending_job1",
						RequiredGPUsPerTask: 1,
						Priority:            constants.PriorityBuildNumber,
						QueueName:           "queue1",
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
					{
						Name:         "queue1",
						DeservedGPUs: 1,
					},
				},
				JobExpectedResults: map[string]test_utils.TestExpectedResultBasic{
					"pending_job0": {
						NodeName:     "node0",
						GPUsRequired: 1,
						Status:       pod_status.Running,
					},
					"pending_job1": {
						GPUsRequired: 1,
						Status:       pod_status.Pending,
					},
				},
				Mocks: &test_utils.TestMock{
					CacheRequirements: &test_utils.CacheMocking{
						NumberOfCacheBinds: 2,
					},
				},
			},
			RoundsUntilMatch: 1,
		},
		{
			TestTopologyBasic: test_utils.TestTopologyBasic{
				Name: "allocate according to priority",
				Jobs: []*jobs_fake.TestJobBasic{
					{
						Name:                "pending_job0",
						RequiredGPUsPerTask: 1,
						Priority:            constants.PriorityBuildNumber,
						QueueName:           "queue0",
						Tasks: []*tasks_fake.TestTaskBasic{
							{
								State: pod_status.Pending,
							},
						},
					}, {
						Name:                "pending_job1",
						RequiredGPUsPerTask: 1,
						Priority:            constants.PriorityBuildNumber,
						QueueName:           "queue1",
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
					{
						Name:         "queue1",
						DeservedGPUs: 1,
						Priority:     pointer.Int(commonconstants.DefaultQueuePriority + 1),
					},
				},
				JobExpectedResults: map[string]test_utils.TestExpectedResultBasic{
					"pending_job0": {
						GPUsRequired: 1,
						Status:       pod_status.Pending,
					},
					"pending_job1": {
						NodeName:     "node0",
						GPUsRequired: 1,
						Status:       pod_status.Running,
					},
				},
				Mocks: &test_utils.TestMock{
					CacheRequirements: &test_utils.CacheMocking{
						NumberOfCacheBinds: 1,
					},
				},
			},
			RoundsUntilMatch: 1,
		},
		{
			TestTopologyBasic: test_utils.TestTopologyBasic{
				Name: "1 build job pending for each queue with different share - allocate the second",
				Jobs: []*jobs_fake.TestJobBasic{
					{
						Name:                "pending_job0",
						RequiredGPUsPerTask: 1,
						Priority:            constants.PriorityBuildNumber,
						QueueName:           "queue0",
						Tasks: []*tasks_fake.TestTaskBasic{
							{
								State: pod_status.Pending,
							},
						},
					}, {
						Name:                "pending_job1",
						RequiredGPUsPerTask: 1,
						Priority:            constants.PriorityBuildNumber,
						QueueName:           "queue1",
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
					{
						Name:         "queue1",
						DeservedGPUs: 2,
					},
				},
				JobExpectedResults: map[string]test_utils.TestExpectedResultBasic{
					"pending_job0": {
						GPUsRequired: 1,
						Status:       pod_status.Pending,
					},
					"pending_job1": {
						NodeName:     "node0",
						GPUsRequired: 1,
						Status:       pod_status.Running,
					},
				},
				Mocks: &test_utils.TestMock{
					CacheRequirements: &test_utils.CacheMocking{
						NumberOfCacheBinds: 1,
					},
				},
			},
			RoundsUntilMatch: 1,
		},
		{
			TestTopologyBasic: test_utils.TestTopologyBasic{
				Name: "6 pending train jobs - allocate the 1st 2 of each queue (verify the share is being updated during the allocation)",
				Jobs: []*jobs_fake.TestJobBasic{
					{
						Name:                "pending_job0",
						RequiredGPUsPerTask: 1,
						Priority:            constants.PriorityTrainNumber,
						QueueName:           "queue0",
						Tasks: []*tasks_fake.TestTaskBasic{
							{
								State: pod_status.Pending,
							},
						},
					}, {
						Name:                "pending_job1",
						RequiredGPUsPerTask: 1,
						Priority:            constants.PriorityTrainNumber,
						QueueName:           "queue0",
						Tasks: []*tasks_fake.TestTaskBasic{
							{
								State: pod_status.Pending,
							},
						},
					}, {
						Name:                "pending_job2",
						RequiredGPUsPerTask: 1,
						Priority:            constants.PriorityTrainNumber,
						QueueName:           "queue0",
						Tasks: []*tasks_fake.TestTaskBasic{
							{
								State: pod_status.Pending,
							},
						},
					}, {
						Name:                "pending_job3",
						RequiredGPUsPerTask: 1,
						Priority:            constants.PriorityTrainNumber,
						QueueName:           "queue1",
						Tasks: []*tasks_fake.TestTaskBasic{
							{
								State: pod_status.Pending,
							},
						},
					}, {
						Name:                "pending_job4",
						RequiredGPUsPerTask: 1,
						Priority:            constants.PriorityTrainNumber,
						QueueName:           "queue1",
						Tasks: []*tasks_fake.TestTaskBasic{
							{
								State: pod_status.Pending,
							},
						},
					}, {
						Name:                "pending_job5",
						RequiredGPUsPerTask: 1,
						Priority:            constants.PriorityTrainNumber,
						QueueName:           "queue1",
						Tasks: []*tasks_fake.TestTaskBasic{
							{
								State: pod_status.Pending,
							},
						},
					},
				},
				Nodes: map[string]nodes_fake.TestNodeBasic{
					"node0": {
						GPUs: 4,
					},
				},
				Queues: []test_utils.TestQueueBasic{
					{
						Name:         "queue0",
						DeservedGPUs: 1,
					},
					{
						Name:         "queue1",
						DeservedGPUs: 1,
					},
				},
				JobExpectedResults: map[string]test_utils.TestExpectedResultBasic{
					"pending_job0": {
						NodeName:     "node0",
						GPUsRequired: 1,
						Status:       pod_status.Running,
					},
					"pending_job1": {
						NodeName:     "node0",
						GPUsRequired: 1,
						Status:       pod_status.Running,
					},
					"pending_job2": {
						GPUsRequired: 1,
						Status:       pod_status.Pending,
					},
					"pending_job3": {
						NodeName:     "node0",
						GPUsRequired: 1,
						Status:       pod_status.Running,
					},
					"pending_job4": {
						NodeName:     "node0",
						GPUsRequired: 1,
						Status:       pod_status.Running,
					},
					"pending_job5": {
						GPUsRequired: 1,
						Status:       pod_status.Pending,
					},
				},
				Mocks: &test_utils.TestMock{
					CacheRequirements: &test_utils.CacheMocking{
						NumberOfCacheBinds: 4,
					},
				},
			},
			RoundsUntilMatch: 1,
		},
		{
			TestTopologyBasic: test_utils.TestTopologyBasic{
				Name: "Over provisioning with over quota, many queues to few GPUs - verify queue share is updated during the same allocation round",
				Jobs: []*jobs_fake.TestJobBasic{
					{
						Name:                "pending_job0",
						RequiredGPUsPerTask: 1,
						Priority:            constants.PriorityTrainNumber,
						QueueName:           "queue0", Tasks: []*tasks_fake.TestTaskBasic{
							{
								State: pod_status.Pending,
							},
						},
					}, {
						Name:                "pending_job1",
						RequiredGPUsPerTask: 1,
						Priority:            constants.PriorityTrainNumber,
						QueueName:           "queue1",
						Tasks: []*tasks_fake.TestTaskBasic{
							{
								State: pod_status.Pending,
							},
						},
					}, {
						Name:                "pending_job2",
						RequiredGPUsPerTask: 1,
						Priority:            constants.PriorityTrainNumber,
						QueueName:           "queue2",
						Tasks: []*tasks_fake.TestTaskBasic{
							{
								State: pod_status.Pending,
							},
						},
					}, {
						Name:                "pending_job3",
						RequiredGPUsPerTask: 1,
						Priority:            constants.PriorityTrainNumber,
						QueueName:           "queue3",
						Tasks: []*tasks_fake.TestTaskBasic{
							{
								State: pod_status.Pending,
							},
						},
					}, {
						Name:                "pending_job4",
						RequiredGPUsPerTask: 1,
						Priority:            constants.PriorityTrainNumber,
						QueueName:           "queue0",
						Tasks: []*tasks_fake.TestTaskBasic{
							{
								State: pod_status.Pending,
							},
						},
					}, {
						Name:                "pending_job5",
						RequiredGPUsPerTask: 1,
						Priority:            constants.PriorityTrainNumber,
						QueueName:           "queue1",
						Tasks: []*tasks_fake.TestTaskBasic{
							{
								State: pod_status.Pending,
							},
						},
					},
				}, Nodes: map[string]nodes_fake.TestNodeBasic{
					"node0": {
						GPUs: 2,
					},
				},
				Queues: []test_utils.TestQueueBasic{
					{
						Name:         "queue0",
						DeservedGPUs: 1,
					}, {
						Name:         "queue1",
						DeservedGPUs: 1,
					}, {
						Name:         "queue2",
						DeservedGPUs: 1,
					}, {
						Name:         "queue3",
						DeservedGPUs: 1,
					},
				},
				JobExpectedResults: map[string]test_utils.TestExpectedResultBasic{
					"pending_job0": {
						NodeName:     "node0",
						GPUsRequired: 1,
						Status:       pod_status.Running,
					},
					"pending_job1": {
						NodeName:     "node0",
						GPUsRequired: 1,
						Status:       pod_status.Running,
					},
					"pending_job2": {
						GPUsRequired: 1,
						Status:       pod_status.Pending,
					},
					"pending_job3": {
						GPUsRequired: 1,
						Status:       pod_status.Pending,
					},
					"pending_job4": {
						GPUsRequired: 1,
						Status:       pod_status.Pending,
					},
					"pending_job5": {
						GPUsRequired: 1,
						Status:       pod_status.Pending,
					},
				},
				Mocks: &test_utils.TestMock{
					CacheRequirements: &test_utils.CacheMocking{
						NumberOfCacheBinds: 2,
					},
				},
			},
			RoundsUntilMatch: 1,
		},
		{
			TestTopologyBasic: test_utils.TestTopologyBasic{
				Name: "Allocate Departments with smaller ratio 1st",
				Jobs: []*jobs_fake.TestJobBasic{
					{
						Name:                "pending_job0",
						RequiredGPUsPerTask: 1,
						Priority:            constants.PriorityTrainNumber,
						QueueName:           "queue0",
						Tasks: []*tasks_fake.TestTaskBasic{
							{
								State: pod_status.Pending,
							},
						},
					}, {
						Name:                "pending_job1",
						RequiredGPUsPerTask: 1,
						Priority:            constants.PriorityTrainNumber,
						QueueName:           "queue1",
						Tasks: []*tasks_fake.TestTaskBasic{
							{
								State: pod_status.Pending,
							},
						},
					}, {
						Name:                "pending_job2",
						RequiredGPUsPerTask: 1,
						Priority:            constants.PriorityTrainNumber,
						QueueName:           "queue2",
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
						DeservedGPUs: 3,
						ParentQueue:  "d1",
					},
					{
						Name:         "queue1",
						DeservedGPUs: 2,
						ParentQueue:  "d1",
					},
					{
						Name:         "queue2",
						DeservedGPUs: 1,
						ParentQueue:  "d2",
					},
				},
				Departments: []test_utils.TestDepartmentBasic{
					{
						Name:         "d1",
						DeservedGPUs: 1,
					},
					{
						Name:         "d2",
						DeservedGPUs: 2,
					},
				},
				JobExpectedResults: map[string]test_utils.TestExpectedResultBasic{
					"pending_job0": {
						GPUsRequired: 1,
						Status:       pod_status.Pending,
					},
					"pending_job1": {
						GPUsRequired: 1,
						Status:       pod_status.Pending,
					},
					"pending_job2": {
						NodeName:     "node0",
						GPUsRequired: 1,
						Status:       pod_status.Running,
					},
				},
				Mocks: &test_utils.TestMock{
					CacheRequirements: &test_utils.CacheMocking{
						NumberOfCacheBinds: 1,
					},
				},
			},
		},
		{
			TestTopologyBasic: test_utils.TestTopologyBasic{
				Name: "Don't allow allocation of interactive jobs above the department's deserved GPUs",
				Jobs: []*jobs_fake.TestJobBasic{
					{
						Name:                "pending_job0",
						RequiredGPUsPerTask: 2,
						Priority:            constants.PriorityBuildNumber,
						QueueName:           "queue0",
						Tasks: []*tasks_fake.TestTaskBasic{
							{
								State: pod_status.Pending,
							},
						},
					},
				},
				Nodes: map[string]nodes_fake.TestNodeBasic{
					"node0": {
						GPUs: 4,
					},
				},
				Queues: []test_utils.TestQueueBasic{
					{
						Name:         "queue0",
						DeservedGPUs: 2,
						ParentQueue:  "d1",
					},
					{
						Name:         "queue1",
						DeservedGPUs: 2,
						ParentQueue:  "d2",
					},
				},
				Departments: []test_utils.TestDepartmentBasic{
					{
						Name:         "d1",
						DeservedGPUs: 1,
					},
					{
						Name:         "d2",
						DeservedGPUs: 1,
					},
				},
				JobExpectedResults: map[string]test_utils.TestExpectedResultBasic{
					"pending_job0": {
						GPUsRequired: 2,
						Status:       pod_status.Pending,
					},
				},
			},
		},
		{
			TestTopologyBasic: test_utils.TestTopologyBasic{
				Name: "try to allocate interactive after train when over-quota - train should be preempted for interactive to run",
				Jobs: []*jobs_fake.TestJobBasic{
					{
						Name:                "running_job_train",
						RequiredGPUsPerTask: 1,
						Priority:            constants.PriorityTrainNumber,
						QueueName:           "queue0",
						Tasks: []*tasks_fake.TestTaskBasic{
							{
								NodeName: "node0",
								State:    pod_status.Running,
							},
						},
					},
					{
						Name:                "pending_job_interactive",
						RequiredGPUsPerTask: 1,
						Priority:            constants.PriorityBuildNumber,
						QueueName:           "queue0",
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
				Queues: []test_utils.TestQueueBasic{
					{
						Name:               "queue0",
						DeservedGPUs:       1,
						GPUOverQuotaWeight: 1,
						MaxAllowedGPUs:     1,
					},
				},
				JobExpectedResults: map[string]test_utils.TestExpectedResultBasic{
					"running_job_train": {
						GPUsRequired: 1,
						Status:       pod_status.Pending,
					},
					"pending_job_interactive": {
						NodeName:     "node0",
						GPUsRequired: 1,
						Status:       pod_status.Running,
					},
				},
				Mocks: &test_utils.TestMock{
					CacheRequirements: &test_utils.CacheMocking{
						NumberOfCacheBinds:      2,
						NumberOfCacheEvictions:  2,
						NumberOfPipelineActions: 1,
					},
				},
			},
			RoundsUntilMatch: 2,
		},
		{
			TestTopologyBasic: test_utils.TestTopologyBasic{
				Name: "try to allocate train after interactive when over-quota - train should not run",
				Jobs: []*jobs_fake.TestJobBasic{
					{
						Name:                "pending_job_interactive0",
						RequiredGPUsPerTask: 1,
						Priority:            constants.PriorityBuildNumber,
						QueueName:           "queue0",
						Tasks: []*tasks_fake.TestTaskBasic{
							{
								NodeName: "node0",
								State:    pod_status.Running,
							},
						},
					},
					{
						Name:                "pending_job_train1",
						RequiredGPUsPerTask: 1,
						Priority:            constants.PriorityTrainNumber,
						QueueName:           "queue0",
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
				Queues: []test_utils.TestQueueBasic{
					{
						Name:               "queue0",
						DeservedGPUs:       1,
						GPUOverQuotaWeight: 1,
						MaxAllowedGPUs:     1,
					},
				},
				JobExpectedResults: map[string]test_utils.TestExpectedResultBasic{
					"pending_job_interactive0": {
						GPUsRequired: 1,
						NodeName:     "node0",
						Status:       pod_status.Running,
					},
					"pending_job_train1": {
						GPUsRequired: 1,
						Status:       pod_status.Pending,
					},
				},
				Mocks: &test_utils.TestMock{
					CacheRequirements: &test_utils.CacheMocking{
						NumberOfCacheBinds: 2,
					},
				},
			},
			RoundsUntilMatch: 1,
		},
		{
			TestTopologyBasic: test_utils.TestTopologyBasic{
				Name: "don't allow job over QUEUE deserved CPU",
				Jobs: []*jobs_fake.TestJobBasic{
					{
						Name:                "pending_job0",
						RequiredGPUsPerTask: 1,
						RequiredCPUsPerTask: 2000,
						Priority:            constants.PriorityTrainNumber,
						QueueName:           "queue0",
						Tasks: []*tasks_fake.TestTaskBasic{
							{
								State: pod_status.Pending,
							},
						},
					},
					{
						Name:                "pending_job1",
						RequiredGPUsPerTask: 1,
						RequiredCPUsPerTask: 2000,
						Priority:            constants.PriorityTrainNumber,
						QueueName:           "queue0",
						Tasks: []*tasks_fake.TestTaskBasic{
							{
								State: pod_status.Pending,
							},
						},
					},
				},
				Nodes: map[string]nodes_fake.TestNodeBasic{
					"node0": {
						GPUs:      4,
						CPUMillis: 5000,
					},
				},
				Queues: []test_utils.TestQueueBasic{
					{
						Name:               "queue0",
						ParentQueue:        "department-a",
						DeservedGPUs:       2,
						GPUOverQuotaWeight: 2,
						MaxAllowedGPUs:     2,
						MaxAllowedCPUs:     test_utils.CreateFloat64Pointer(2500 * 1000),
					},
				},
				Departments: []test_utils.TestDepartmentBasic{
					{
						Name:         "department-a",
						DeservedGPUs: 2,
					},
				},
				JobExpectedResults: map[string]test_utils.TestExpectedResultBasic{
					"pending_job0": {
						NodeName:     "node0",
						GPUsRequired: 1,
						Status:       pod_status.Running,
					},
					"pending_job1": {
						GPUsRequired: 1,
						Status:       pod_status.Pending,
					},
				},
				Mocks: &test_utils.TestMock{
					CacheRequirements: &test_utils.CacheMocking{
						NumberOfCacheBinds: 2,
					},
				},
			},
			RoundsUntilMatch: 1,
		},
		{
			TestTopologyBasic: test_utils.TestTopologyBasic{
				Name: "don't allow job over DEPARTMENT deserved CPU",
				Jobs: []*jobs_fake.TestJobBasic{
					{
						Name:                "pending_job0",
						RequiredGPUsPerTask: 1,
						RequiredCPUsPerTask: 2000,
						Priority:            constants.PriorityTrainNumber,
						QueueName:           "queue0",
						Tasks: []*tasks_fake.TestTaskBasic{
							{
								State: pod_status.Pending,
							},
						},
					},
					{
						Name:                "pending_job1",
						RequiredGPUsPerTask: 1,
						RequiredCPUsPerTask: 2000,
						Priority:            constants.PriorityTrainNumber,
						QueueName:           "queue0",
						Tasks: []*tasks_fake.TestTaskBasic{
							{
								State: pod_status.Pending,
							},
						},
					},
				},
				Nodes: map[string]nodes_fake.TestNodeBasic{
					"node0": {
						GPUs:      4,
						CPUMillis: 5000,
					},
				},
				Queues: []test_utils.TestQueueBasic{
					{
						Name:               "queue0",
						ParentQueue:        "department-a",
						DeservedGPUs:       2,
						GPUOverQuotaWeight: 2,
						MaxAllowedGPUs:     2,
					},
				},
				Departments: []test_utils.TestDepartmentBasic{
					{
						Name:           "department-a",
						DeservedGPUs:   2,
						MaxAllowedCPUs: test_utils.CreateFloat64Pointer(2500 * 1000),
					},
				},
				JobExpectedResults: map[string]test_utils.TestExpectedResultBasic{
					"pending_job0": {
						NodeName:     "node0",
						GPUsRequired: 1,
						Status:       pod_status.Running,
					},
					"pending_job1": {
						GPUsRequired: 1,
						Status:       pod_status.Pending,
					},
				},
				Mocks: &test_utils.TestMock{
					CacheRequirements: &test_utils.CacheMocking{
						NumberOfCacheBinds: 2,
					},
				},
			},
			RoundsUntilMatch: 1,
		},
		{
			TestTopologyBasic: test_utils.TestTopologyBasic{
				Name: "Topology basic test",
				Topologies: []*kueuev1alpha1.Topology{
					{
						ObjectMeta: v1.ObjectMeta{
							Name: "cluster-topology",
						},
						Spec: kueuev1alpha1.TopologySpec{
							Levels: []kueuev1alpha1.TopologyLevel{
								{
									NodeLabel: "k8s.io/rack",
								},
							},
						},
					},
				},
				Jobs: []*jobs_fake.TestJobBasic{
					{
						Name:                "running_job0",
						RequiredGPUsPerTask: 1,
						Priority:            constants.PriorityTrainNumber,
						QueueName:           "queue0",
						Tasks: []*tasks_fake.TestTaskBasic{
							{
								State:    pod_status.Running,
								NodeName: "node0",
							},
						},
					},
					{
						Name:                "pending_job0",
						RequiredGPUsPerTask: 1,
						Priority:            constants.PriorityTrainNumber,
						QueueName:           "queue0",
						Topology: &enginev2alpha2.TopologyConstraint{
							Topology:              "cluster-topology",
							RequiredTopologyLevel: "k8s.io/rack",
						},
						Tasks: []*tasks_fake.TestTaskBasic{
							{
								State: pod_status.Pending,
							},
							{
								State: pod_status.Pending,
							},
						},
					},
				},
				Nodes: map[string]nodes_fake.TestNodeBasic{
					"node0": {
						GPUs: 1,
						Labels: map[string]string{
							"k8s.io/rack": "rack1",
						},
					},
					"node1": {
						GPUs: 1,
						Labels: map[string]string{
							"k8s.io/rack": "rack1",
						},
					},
					"node2": {
						GPUs: 1,
						Labels: map[string]string{
							"k8s.io/rack": "rack2",
						},
					},
					"node3": {
						GPUs: 1,
						Labels: map[string]string{
							"k8s.io/rack": "rack2",
						},
					},
				},
				Queues: []test_utils.TestQueueBasic{
					{
						Name:               "queue0",
						ParentQueue:        "department-a",
						DeservedGPUs:       4,
						GPUOverQuotaWeight: 1,
						MaxAllowedGPUs:     4,
					},
				},
				Departments: []test_utils.TestDepartmentBasic{
					{
						Name:         "department-a",
						DeservedGPUs: 4,
					},
				},
				TaskExpectedResults: map[string]test_utils.TestExpectedResultBasic{
					"running_job0-0": {
						NodeName:             "node0",
						GPUsRequired:         1,
						Status:               pod_status.Running,
						DontValidateGPUGroup: true,
					},
					"pending_job0-0": {
						NodeName:             "node2",
						GPUsRequired:         1,
						Status:               pod_status.Running,
						DontValidateGPUGroup: true,
					},
					"pending_job0-1": {
						NodeName:             "node3",
						GPUsRequired:         1,
						Status:               pod_status.Running,
						DontValidateGPUGroup: true,
					},
				},
				Mocks: &test_utils.TestMock{
					CacheRequirements: &test_utils.CacheMocking{
						NumberOfCacheBinds: 2,
					},
				},
			},
			RoundsUntilMatch: 1,
		},
	}
}
