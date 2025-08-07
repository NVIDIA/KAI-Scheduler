// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package reclaim_test

import (
	"testing"

	. "go.uber.org/mock/gomock"
	"gopkg.in/h2non/gock.v1"
	"k8s.io/utils/pointer"

	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/actions/integration_tests/integration_tests_utils"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/actions/reclaim"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/pod_status"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/podgroup_info"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/constants"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/test_utils"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/test_utils/jobs_fake"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/test_utils/nodes_fake"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/test_utils/tasks_fake"
)

func TestHandleReclaimSubGroups(t *testing.T) {
	test_utils.InitTestingInfrastructure()
	controller := NewController(t)
	defer controller.Finish()
	defer gock.Off()
	testsMetadata := getReclaimSubGroupsTestsMetadata()

	for testNumber, testMetadata := range testsMetadata {
		t.Logf("Running test number: %v, test name: %v,", testNumber, testMetadata.TestTopologyBasic.Name)
		ssn := test_utils.BuildSession(testMetadata.TestTopologyBasic, controller)
		reclaimAction := reclaim.New()
		reclaimAction.Execute(ssn)

		test_utils.MatchExpectedAndRealTasks(t, testNumber, testMetadata.TestTopologyBasic, ssn)
	}
}

func getReclaimSubGroupsTestsMetadata() []integration_tests_utils.TestTopologyMetadata {
	return []integration_tests_utils.TestTopologyMetadata{
		{
			TestTopologyBasic: test_utils.TestTopologyBasic{
				Name: "Reclaim resources for job with sub groups",
				Jobs: []*jobs_fake.TestJobBasic{
					{
						Name:                "running-job",
						RequiredGPUsPerTask: 1,
						Priority:            constants.PriorityTrainNumber,
						QueueName:           "queue0",
						Tasks: []*tasks_fake.TestTaskBasic{
							{
								NodeName: "node0",
								State:    pod_status.Running,
							},
							{
								NodeName: "node0",
								State:    pod_status.Running,
							},
						},
						MinAvailable: pointer.Int32(2),
					},
					{
						Name:                "pending-job",
						RequiredGPUsPerTask: 1,
						Priority:            constants.PriorityTrainNumber,
						QueueName:           "queue1",
						SubGroups: map[string]*podgroup_info.SubGroupInfo{
							"sub-0": podgroup_info.NewSubGroupInfo("sub-0", 1),
							"sub-1": podgroup_info.NewSubGroupInfo("sub-1", 1),
						},
						Tasks: []*tasks_fake.TestTaskBasic{
							{
								NodeName:     "node0",
								State:        pod_status.Pending,
								SubGroupName: "sub-0",
							},
							{
								NodeName:     "node0",
								State:        pod_status.Pending,
								SubGroupName: "sub-1",
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
						DeservedGPUs: 0,
					},
					{
						Name:         "queue1",
						DeservedGPUs: 2,
					},
				},
				TaskExpectedResults: map[string]test_utils.TestExpectedResultBasic{
					"running-job-0": {
						NodeName:             "node0",
						GPUsRequired:         1,
						Status:               pod_status.Releasing,
						DontValidateGPUGroup: true,
					},
					"running-job-1": {
						NodeName:             "node0",
						GPUsRequired:         1,
						Status:               pod_status.Releasing,
						DontValidateGPUGroup: true,
					},

					"pending-job-0": {
						NodeName:             "node0",
						GPUsRequired:         1,
						Status:               pod_status.Pipelined,
						DontValidateGPUGroup: true,
					},
					"pending-job-1": {
						NodeName:             "node0",
						GPUsRequired:         1,
						Status:               pod_status.Pipelined,
						DontValidateGPUGroup: true,
					},
				},
				Mocks: &test_utils.TestMock{
					CacheRequirements: &test_utils.CacheMocking{
						NumberOfCacheEvictions:  2,
						NumberOfPipelineActions: 2,
					},
				},
			},
		},
		{
			TestTopologyBasic: test_utils.TestTopologyBasic{
				Name: "Job with sub groups cannot reclaim resources - cannot satisfy sub group gang",
				Jobs: []*jobs_fake.TestJobBasic{
					{
						Name:                "running-job",
						RequiredGPUsPerTask: 1,
						Priority:            constants.PriorityTrainNumber,
						QueueName:           "queue0",
						Tasks: []*tasks_fake.TestTaskBasic{
							{
								NodeName: "node0",
								State:    pod_status.Running,
							},
							{
								NodeName: "node0",
								State:    pod_status.Running,
							},
						},
						MinAvailable: pointer.Int32(2),
					},
					{
						Name:                "pending-job",
						RequiredGPUsPerTask: 2,
						Priority:            constants.PriorityTrainNumber,
						QueueName:           "queue1",
						SubGroups: map[string]*podgroup_info.SubGroupInfo{
							"sub-0": podgroup_info.NewSubGroupInfo("sub-0", 1),
							"sub-1": podgroup_info.NewSubGroupInfo("sub-1", 1),
						},
						Tasks: []*tasks_fake.TestTaskBasic{
							{
								NodeName:     "node0",
								State:        pod_status.Pending,
								SubGroupName: "sub-0",
							},
							{
								NodeName:     "node0",
								State:        pod_status.Pending,
								SubGroupName: "sub-1",
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
						DeservedGPUs: 0,
					},
					{
						Name:         "queue1",
						DeservedGPUs: 2,
					},
				},
				TaskExpectedResults: map[string]test_utils.TestExpectedResultBasic{
					"running-job-0": {
						NodeName:             "node0",
						GPUsRequired:         1,
						Status:               pod_status.Running,
						DontValidateGPUGroup: true,
					},
					"running-job-1": {
						NodeName:             "node0",
						GPUsRequired:         1,
						Status:               pod_status.Running,
						DontValidateGPUGroup: true,
					},

					"pending-job-0": {
						NodeName:             "node0",
						GPUsRequired:         2,
						Status:               pod_status.Pending,
						DontValidateGPUGroup: true,
					},
					"pending-job-1": {
						NodeName:             "node0",
						GPUsRequired:         2,
						Status:               pod_status.Pending,
						DontValidateGPUGroup: true,
					},
				},
				Mocks: &test_utils.TestMock{
					CacheRequirements: &test_utils.CacheMocking{},
				},
			},
		},
	}
}
