// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package consolidation_test

import (
	"fmt"
	"testing"

	. "go.uber.org/mock/gomock"
	"k8s.io/utils/ptr"

	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/actions/consolidation"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/actions/integration_tests/integration_tests_utils"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/pod_status"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/podgroup_info"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/constants"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/test_utils"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/test_utils/jobs_fake"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/test_utils/nodes_fake"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/test_utils/tasks_fake"
)

func TestSubGroupsConsolidation(t *testing.T) {
	test_utils.InitTestingInfrastructure()
	controller := NewController(t)
	defer controller.Finish()
	testsMetadata := getSubGroupsConsolidationTestsMetadata()

	for testNumber, testMetadata := range testsMetadata {
		fmt.Printf("Running test %d/%d: %s\n", testNumber, len(testsMetadata), testMetadata.TestTopologyBasic.Name)
		ssn := test_utils.BuildSession(testMetadata.TestTopologyBasic, controller)
		consolidationAction := consolidation.New()
		consolidationAction.Execute(ssn)
		test_utils.MatchExpectedAndRealTasks(t, testNumber, testMetadata.TestTopologyBasic, ssn)
	}
}

func getSubGroupsConsolidationTestsMetadata() []integration_tests_utils.TestTopologyMetadata {
	return []integration_tests_utils.TestTopologyMetadata{
		{
			TestTopologyBasic: test_utils.TestTopologyBasic{
				Name: "job with sub groups consolidates a running job",
				Jobs: []*jobs_fake.TestJobBasic{
					{
						Name:                "running_job0",
						RequiredGPUsPerTask: 2,
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
						Name:                "running_job1",
						RequiredGPUsPerTask: 2,
						Priority:            constants.PriorityTrainNumber,
						QueueName:           "queue0",
						Tasks: []*tasks_fake.TestTaskBasic{
							{
								State:    pod_status.Running,
								NodeName: "node1",
							},
						},
					},
					{
						Name:      "pending_job0",
						Priority:  constants.PriorityTrainNumber,
						QueueName: "queue0",
						SubGroups: map[string]*podgroup_info.SubGroupInfo{
							"sub-0": podgroup_info.NewSubGroupInfo("sub-0", 1),
							"sub-1": podgroup_info.NewSubGroupInfo("sub-1", 1),
						},
						Tasks: []*tasks_fake.TestTaskBasic{
							{
								State:        pod_status.Pending,
								SubGroupName: "sub-0",
								RequiredGPUs: ptr.To(int64(1)),
							},
							{
								State:        pod_status.Pending,
								SubGroupName: "sub-1",
								RequiredGPUs: ptr.To(int64(3)),
							},
						},
						MinAvailable: ptr.To(int32(2)),
					},
				},
				Nodes: map[string]nodes_fake.TestNodeBasic{
					"node0": {
						GPUs: 4,
					},
					"node1": {
						GPUs: 4,
					},
				},
				Queues: []test_utils.TestQueueBasic{
					{
						Name:         "queue0",
						DeservedGPUs: 2,
					},
				},
				JobExpectedResults: map[string]test_utils.TestExpectedResultBasic{
					"running_job0": {
						NodeName:     "node0",
						GPUsRequired: 2,
						Status:       pod_status.Running,
					},
					"running_job1": {
						NodeName:     "node0",
						GPUsRequired: 2,
						Status:       pod_status.Pipelined,
					},
					"pending_job0": {
						GPUsRequired: 4,
						NodeName:     "node1",
						Status:       pod_status.Pipelined,
					},
				},
				Mocks: &test_utils.TestMock{
					CacheRequirements: &test_utils.CacheMocking{
						NumberOfCacheEvictions:  1,
						NumberOfPipelineActions: 3,
					},
				},
			},
		},
		{
			TestTopologyBasic: test_utils.TestTopologyBasic{
				Name: "job with sub groups cannot consolidate a running job - cannot satisfy sub group gang",
				Jobs: []*jobs_fake.TestJobBasic{
					{
						Name:                "running_job0",
						RequiredGPUsPerTask: 2,
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
						Name:                "running_job1",
						RequiredGPUsPerTask: 2,
						Priority:            constants.PriorityTrainNumber,
						QueueName:           "queue0",
						Tasks: []*tasks_fake.TestTaskBasic{
							{
								State:    pod_status.Running,
								NodeName: "node1",
							},
						},
					},
					{
						Name:      "pending_job0",
						Priority:  constants.PriorityTrainNumber,
						QueueName: "queue0",
						SubGroups: map[string]*podgroup_info.SubGroupInfo{
							"sub-0": podgroup_info.NewSubGroupInfo("sub-0", 1),
							"sub-1": podgroup_info.NewSubGroupInfo("sub-1", 1),
						},
						Tasks: []*tasks_fake.TestTaskBasic{
							{
								State:        pod_status.Pending,
								SubGroupName: "sub-0",
								RequiredGPUs: ptr.To(int64(3)),
							},
							{
								State:        pod_status.Pending,
								SubGroupName: "sub-1",
								RequiredGPUs: ptr.To(int64(3)),
							},
						},
						MinAvailable: ptr.To(int32(2)),
					},
				},
				Nodes: map[string]nodes_fake.TestNodeBasic{
					"node0": {
						GPUs: 4,
					},
					"node1": {
						GPUs: 4,
					},
				},
				Queues: []test_utils.TestQueueBasic{
					{
						Name:         "queue0",
						DeservedGPUs: 2,
					},
				},
				JobExpectedResults: map[string]test_utils.TestExpectedResultBasic{
					"running_job0": {
						NodeName:     "node0",
						GPUsRequired: 2,
						Status:       pod_status.Running,
					},
					"running_job1": {
						NodeName:     "node1",
						GPUsRequired: 2,
						Status:       pod_status.Running,
					},
					"pending_job0": {
						GPUsRequired: 6,
						Status:       pod_status.Pending,
					},
				},
				Mocks: &test_utils.TestMock{
					CacheRequirements: &test_utils.CacheMocking{},
				},
			},
		},
	}
}
