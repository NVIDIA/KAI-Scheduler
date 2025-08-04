// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package reclaim_test

import (
	"testing"

	. "go.uber.org/mock/gomock"
	"k8s.io/utils/ptr"

	commonconstants "github.com/NVIDIA/KAI-scheduler/pkg/common/constants"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/actions/integration_tests/integration_tests_utils"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/actions/reclaim"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/pod_status"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/conf"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/constants"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/test_utils"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/test_utils/jobs_fake"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/test_utils/nodes_fake"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/test_utils/tasks_fake"
)

var priorityBasedFairShareSchedulerConf = &conf.SchedulerConfiguration{
	Actions: "allocate, consolidation, reclaim, preempt, stalegangeviction",
	Tiers: []conf.Tier{
		{
			Plugins: []conf.PluginOption{
				{Name: "predicates"},
				{Name: "proportion", Arguments: map[string]string{"priorityBasedFairShare": "true"}},
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
				{Name: "nodeplacement", Arguments: map[string]string{
					constants.CPUResource: constants.BinpackStrategy,
					constants.GPUResource: constants.BinpackStrategy,
				}},
				{Name: "minruntime"},
			},
		},
	},
}

func TestPriorityBasedFairShareReclaim(t *testing.T) {
	test_utils.InitTestingInfrastructure()
	controller := NewController(t)
	defer controller.Finish()

	testsMetadata := getPriorityFairShareReclaimTestsMetadata()

	for testNumber, testMetadata := range testsMetadata {
		t.Logf("Running test %d – %s", testNumber, testMetadata.TestTopologyBasic.Name)

		ssn := test_utils.BuildSession(testMetadata.TestTopologyBasic, controller)

		reclaimAction := reclaim.New()
		reclaimAction.Execute(ssn)

		test_utils.MatchExpectedAndRealTasks(t, testNumber, testMetadata.TestTopologyBasic, ssn)
	}
}

// Two scenarios:
// 1. High-priority queue should reclaim from lower-priority queues even with 0 deserved / 0 weight.
// 2. Same-priority queue must NOT reclaim under the same conditions.
func getPriorityFairShareReclaimTestsMetadata() []integration_tests_utils.TestTopologyMetadata {
	return []integration_tests_utils.TestTopologyMetadata{
		highPriorityReclaimScenario(),
		samePriorityNoReclaimScenario(),
	}
}

// --- Scenario helpers ----------------------------------------------------

func highPriorityReclaimScenario() integration_tests_utils.TestTopologyMetadata {
	return integration_tests_utils.TestTopologyMetadata{
		TestTopologyBasic: test_utils.TestTopologyBasic{
			Name: "High priority queue reclaims resources from lower priority queues",
			Jobs: []*jobs_fake.TestJobBasic{
				// Lower-priority queue jobs consuming GPUs.
				{
					Name:                "low_q_job0",
					RequiredGPUsPerTask: 1,
					QueueName:           "low_q",
					Tasks: []*tasks_fake.TestTaskBasic{{
						NodeName: "node0",
						State:    pod_status.Running,
					}},
				},
				{
					Name:                "low_q_job1",
					RequiredGPUsPerTask: 1,
					QueueName:           "low_q",
					Tasks: []*tasks_fake.TestTaskBasic{{
						NodeName: "node0",
						State:    pod_status.Running,
					}},
				},
				{
					Name:                "low_q_job2",
					RequiredGPUsPerTask: 1,
					QueueName:           "low_q",
					Tasks: []*tasks_fake.TestTaskBasic{{
						NodeName: "node0",
						State:    pod_status.Running,
					}},
				},
				// Reclaimer job with higher priority, pending two tasks.
				{
					Name:                "reclaimer_job",
					MinAvailable:        ptr.To(int32(2)),
					RequiredGPUsPerTask: 1,
					QueueName:           "reclaimer_q",
					Tasks: []*tasks_fake.TestTaskBasic{
						{State: pod_status.Pending},
						{State: pod_status.Pending},
					},
				},
			},
			Nodes: map[string]nodes_fake.TestNodeBasic{
				"node0": {GPUs: 4},
			},
			Queues: []test_utils.TestQueueBasic{
				{
					Name:               "low_q",
					DeservedGPUs:       1,
					GPUOverQuotaWeight: 1,
					Priority:           ptr.To(commonconstants.DefaultQueuePriority),
				},
				{
					Name:               "reclaimer_q",
					DeservedGPUs:       0,
					GPUOverQuotaWeight: 0,
					Priority:           ptr.To(commonconstants.DefaultQueuePriority + 1),
				},
			},
			JobExpectedResults: map[string]test_utils.TestExpectedResultBasic{
				// Expect one of low_q jobs to be releasing (any), keep two running.
				"low_q_job0":    {NodeName: "node0", GPUsRequired: 1, Status: pod_status.Running},
				"low_q_job1":    {NodeName: "node0", GPUsRequired: 1, Status: pod_status.Running},
				"low_q_job2":    {NodeName: "node0", GPUsRequired: 1, Status: pod_status.Releasing},
				"reclaimer_job": {NodeName: "node0", GPUsRequired: 2, Status: pod_status.Pipelined},
			},

			Mocks: &test_utils.TestMock{
				CacheRequirements: &test_utils.CacheMocking{
					NumberOfCacheBinds:      1,
					NumberOfCacheEvictions:  1,
					NumberOfPipelineActions: 2,
				},
				SchedulerConf: priorityBasedFairShareSchedulerConf,
			},
		},
	}
}

func samePriorityNoReclaimScenario() integration_tests_utils.TestTopologyMetadata {
	return integration_tests_utils.TestTopologyMetadata{
		TestTopologyBasic: test_utils.TestTopologyBasic{
			Name: "Same priority queue cannot reclaim resources",
			Jobs: []*jobs_fake.TestJobBasic{
				// Lower-priority (same actually) queue jobs consuming GPUs.
				{
					Name:                "low_q_job0",
					RequiredGPUsPerTask: 1,
					QueueName:           "low_q",
					Tasks: []*tasks_fake.TestTaskBasic{{
						NodeName: "node0",
						State:    pod_status.Running,
					}},
				},
				{
					Name:                "low_q_job1",
					RequiredGPUsPerTask: 1,
					QueueName:           "low_q",
					Tasks: []*tasks_fake.TestTaskBasic{{
						NodeName: "node0",
						State:    pod_status.Running,
					}},
				},
				{
					Name:                "low_q_job2",
					RequiredGPUsPerTask: 1,
					QueueName:           "low_q",
					Tasks: []*tasks_fake.TestTaskBasic{{
						NodeName: "node0",
						State:    pod_status.Running,
					}},
				},
				// Reclaimer job with SAME priority, pending two tasks.
				{
					Name:                "reclaimer_job",
					MinAvailable:        ptr.To(int32(2)),
					RequiredGPUsPerTask: 1,
					QueueName:           "reclaimer_q",
					Tasks: []*tasks_fake.TestTaskBasic{
						{State: pod_status.Pending},
						{State: pod_status.Pending},
					},
				},
			},
			Nodes: map[string]nodes_fake.TestNodeBasic{
				"node0": {GPUs: 4},
			},
			Queues: []test_utils.TestQueueBasic{
				{
					Name:               "low_q",
					DeservedGPUs:       0,
					GPUOverQuotaWeight: 1,
				},
				{
					Name:               "reclaimer_q",
					DeservedGPUs:       0,
					GPUOverQuotaWeight: 0,
				},
			},
			JobExpectedResults: map[string]test_utils.TestExpectedResultBasic{
				// All low_q jobs stay running; reclaimer stays pending.
				"low_q_job0":    {NodeName: "node0", GPUsRequired: 1, Status: pod_status.Running},
				"low_q_job1":    {NodeName: "node0", GPUsRequired: 1, Status: pod_status.Running},
				"low_q_job2":    {NodeName: "node0", GPUsRequired: 1, Status: pod_status.Running},
				"reclaimer_job": {GPUsRequired: 2, Status: pod_status.Pending},
			},

			Mocks: &test_utils.TestMock{
				SchedulerConf: priorityBasedFairShareSchedulerConf,
			},
		},
	}
}
