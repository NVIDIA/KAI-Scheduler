// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package allocate_test

import (
	"testing"

	. "go.uber.org/mock/gomock"
	"k8s.io/utils/pointer"
	"k8s.io/utils/ptr"

	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/actions/allocate"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/actions/integration_tests/integration_tests_utils"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/pod_status"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/podgroup_info"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/constants"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/test_utils"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/test_utils/jobs_fake"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/test_utils/nodes_fake"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/test_utils/tasks_fake"
)

func TestHandleSubGroupsAllocation(t *testing.T) {
	test_utils.InitTestingInfrastructure()
	controller := NewController(t)
	defer controller.Finish()

	for testNumber, testMetadata := range getAllocationSubGroupsTestsMetadata() {
		t.Logf("Running test %d: %s", testNumber, testMetadata.TestTopologyBasic.Name)

		ssn := test_utils.BuildSession(testMetadata.TestTopologyBasic, controller)
		allocateAction := allocate.New()
		allocateAction.Execute(ssn)

		test_utils.MatchExpectedAndRealTasks(t, testNumber, testMetadata.TestTopologyBasic, ssn)
	}
}

func getAllocationSubGroupsTestsMetadata() []integration_tests_utils.TestTopologyMetadata {
	return []integration_tests_utils.TestTopologyMetadata{
		{
			TestTopologyBasic: test_utils.TestTopologyBasic{
				Name: "Allocate job with SubGroups - full allocate",
				Jobs: []*jobs_fake.TestJobBasic{
					{
						Name:                "pending_job0",
						RequiredGPUsPerTask: 1,
						QueueName:           "queue0",
						Priority:            constants.PriorityTrainNumber,
						SubGroups: map[string]*podgroup_info.SubGroupInfo{
							"sub0": podgroup_info.NewSubGroupInfo("sub0", 1),
							"sub1": podgroup_info.NewSubGroupInfo("sub1", 1),
						},
						Tasks: []*tasks_fake.TestTaskBasic{
							{
								State:        pod_status.Pending,
								SubGroupName: "sub0",
							},
							{
								State:        pod_status.Pending,
								SubGroupName: "sub1",
							},
						},
						MinAvailable: pointer.Int32(2),
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
				Mocks: &test_utils.TestMock{
					CacheRequirements: &test_utils.CacheMocking{
						NumberOfCacheBinds: 2,
					},
				},
				TaskExpectedResults: map[string]test_utils.TestExpectedResultBasic{
					"pending_job0-0": {
						NodeName:     "node0",
						GPUsRequired: 1,
						Status:       pod_status.Binding,
					},
					"pending_job0-1": {
						NodeName:     "node0",
						GPUsRequired: 1,
						Status:       pod_status.Binding,
					},
				},
			},
		},
		{
			TestTopologyBasic: test_utils.TestTopologyBasic{
				Name: "Allocate job with SubGroups - partial allocation",
				Jobs: []*jobs_fake.TestJobBasic{
					{
						Name:                "pending_job0",
						RequiredGPUsPerTask: 1,
						QueueName:           "queue0",
						Priority:            constants.PriorityTrainNumber,
						SubGroups: map[string]*podgroup_info.SubGroupInfo{
							"sub0": podgroup_info.NewSubGroupInfo("sub0", 1),
							"sub1": podgroup_info.NewSubGroupInfo("sub1", 1),
						},
						Tasks: []*tasks_fake.TestTaskBasic{
							{
								State:        pod_status.Pending,
								SubGroupName: "sub0",
							},
							{
								State:        pod_status.Pending,
								SubGroupName: "sub0",
							},
							{
								State:        pod_status.Pending,
								SubGroupName: "sub1",
							},
							{
								State:        pod_status.Pending,
								SubGroupName: "sub1",
							},
						},
						MinAvailable: pointer.Int32(2),
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
				Mocks: &test_utils.TestMock{
					CacheRequirements: &test_utils.CacheMocking{
						NumberOfCacheBinds: 2,
					},
				},
				TaskExpectedResults: map[string]test_utils.TestExpectedResultBasic{
					"pending_job0-0": {
						NodeName:     "node0",
						GPUsRequired: 1,
						Status:       pod_status.Binding,
					},
					"pending_job0-1": {
						GPUsRequired: 1,
						Status:       pod_status.Pending,
					},
					"pending_job0-2": {
						NodeName:     "node0",
						GPUsRequired: 1,
						Status:       pod_status.Binding,
					},
					"pending_job0-3": {
						GPUsRequired: 1,
						Status:       pod_status.Pending,
					},
				},
			},
		},
		{
			TestTopologyBasic: test_utils.TestTopologyBasic{
				Name: "Allocate job with SubGroups - cannot satisfy sub group gang",
				Jobs: []*jobs_fake.TestJobBasic{
					{
						Name:      "pending_job0",
						QueueName: "queue0",
						Priority:  constants.PriorityTrainNumber,
						SubGroups: map[string]*podgroup_info.SubGroupInfo{
							"sub0": podgroup_info.NewSubGroupInfo("sub0", 1),
							"sub1": podgroup_info.NewSubGroupInfo("sub1", 2),
						},
						Tasks: []*tasks_fake.TestTaskBasic{
							{
								State:        pod_status.Pending,
								SubGroupName: "sub0",
								RequiredGPUs: ptr.To(int64(3)),
							},
							{
								State:        pod_status.Pending,
								SubGroupName: "sub0",
								RequiredGPUs: ptr.To(int64(3)),
							},
							{
								State:        pod_status.Pending,
								SubGroupName: "sub1",
								RequiredGPUs: ptr.To(int64(1)),
							},
							{
								State:        pod_status.Pending,
								SubGroupName: "sub1",
								RequiredGPUs: ptr.To(int64(1)),
							},
						},
						MinAvailable: pointer.Int32(3),
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
				},
				Mocks: &test_utils.TestMock{
					CacheRequirements: &test_utils.CacheMocking{},
				},
				TaskExpectedResults: map[string]test_utils.TestExpectedResultBasic{
					"pending_job0-0": {
						GPUsRequired: 3,
						Status:       pod_status.Pending,
					},
					"pending_job0-1": {
						GPUsRequired: 3,
						Status:       pod_status.Pending,
					},
					"pending_job0-2": {
						GPUsRequired: 1,
						Status:       pod_status.Pending,
					},
					"pending_job0-3": {
						GPUsRequired: 1,
						Status:       pod_status.Pending,
					},
				},
			},
		},
	}
}
