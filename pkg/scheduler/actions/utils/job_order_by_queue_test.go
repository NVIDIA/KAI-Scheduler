// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package utils

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/common_info"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/pod_info"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/pod_status"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/podgroup_info"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/queue_info"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/conf"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/framework"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/plugins/elastic"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/plugins/priority"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/scheduler_util"
)

const (
	testDepartment = "d1"
	testQueue      = "q1"
	testPod        = "p1"
)

func TestNumericalPriorityWithinSameQueue(t *testing.T) {
	ssn := newPrioritySession()

	ssn.Queues = map[common_info.QueueID]*queue_info.QueueInfo{
		testQueue: {
			UID:         testQueue,
			ParentQueue: testDepartment,
		},
		testDepartment: {
			UID:         testDepartment,
			ChildQueues: []common_info.QueueID{testQueue},
		},
	}
	ssn.PodGroupInfos = map[common_info.PodGroupID]*podgroup_info.PodGroupInfo{
		"0": {
			Name:     "p150",
			Priority: 150,
			Queue:    testQueue,
			PodStatusIndex: map[pod_status.PodStatus]pod_info.PodsMap{
				pod_status.Pending: {
					testPod: {},
				},
			},
			PodInfos: pod_info.PodsMap{
				testPod: {},
			},
		},
		"1": {
			Name:     "p255",
			Priority: 255,
			Queue:    testQueue,
			PodStatusIndex: map[pod_status.PodStatus]pod_info.PodsMap{
				pod_status.Pending: {
					testPod: {},
				},
			},
			PodInfos: pod_info.PodsMap{
				testPod: {},
			},
		},
		"2": {
			Name:     "p160",
			Priority: 160,
			Queue:    testQueue,
			PodStatusIndex: map[pod_status.PodStatus]pod_info.PodsMap{
				pod_status.Pending: {
					testPod: {},
				},
			},
			PodInfos: pod_info.PodsMap{
				testPod: {},
			},
		},
		"3": {
			Name:     "p200",
			Priority: 200,
			Queue:    testQueue,
			PodStatusIndex: map[pod_status.PodStatus]pod_info.PodsMap{
				pod_status.Pending: {
					testPod: {},
				},
			},
			PodInfos: pod_info.PodsMap{
				testPod: {},
			},
		},
	}

	jobsOrderByQueues := NewJobsOrderByQueues(ssn, JobsOrderInitOptions{
		FilterNonPending:  true,
		FilterUnready:     true,
		MaxJobsQueueDepth: scheduler_util.QueueCapacityInfinite,
	})
	jobsOrderByQueues.InitializeWithJobs(ssn.PodGroupInfos)

	expectedJobsOrder := []string{"p255", "p200", "p160", "p150"}
	actualJobsOrder := []string{}
	for !jobsOrderByQueues.IsEmpty() {
		job := jobsOrderByQueues.PopNextJob()
		actualJobsOrder = append(actualJobsOrder, job.Name)
	}
	assert.Equal(t, expectedJobsOrder, actualJobsOrder)
}

func TestJobsOrderByQueues_PushJob(t *testing.T) {
	type fields struct {
		jobsOrderInitOptions JobsOrderInitOptions
		Queues               map[common_info.QueueID]*queue_info.QueueInfo
		InsertedJob          map[common_info.PodGroupID]*podgroup_info.PodGroupInfo
	}
	type args struct {
		job *podgroup_info.PodGroupInfo
	}
	type expected struct {
		expectedJobsList []*podgroup_info.PodGroupInfo
	}
	tests := []struct {
		name     string
		fields   fields
		args     args
		expected expected
	}{
		{
			name: "single podgroup insert - empty queue",
			fields: fields{
				jobsOrderInitOptions: JobsOrderInitOptions{
					VictimQueue:       false,
					FilterNonPending:  true,
					FilterUnready:     true,
					MaxJobsQueueDepth: scheduler_util.QueueCapacityInfinite,
				},
				Queues: map[common_info.QueueID]*queue_info.QueueInfo{
					"q1": {ParentQueue: "d1", UID: "q1"},
					"d1": {UID: "d1"},
				},
				InsertedJob: map[common_info.PodGroupID]*podgroup_info.PodGroupInfo{},
			},
			args: args{
				job: &podgroup_info.PodGroupInfo{
					Name:     "p150",
					Priority: 150,
					Queue:    "q1",
					PodStatusIndex: map[pod_status.PodStatus]pod_info.PodsMap{
						pod_status.Pending: {
							"p1": {},
						},
					},
					PodInfos: pod_info.PodsMap{
						"p1": {},
					},
				},
			},
			expected: expected{
				expectedJobsList: []*podgroup_info.PodGroupInfo{
					{
						Name:     "p150",
						Priority: 150,
						Queue:    "q1",
						PodStatusIndex: map[pod_status.PodStatus]pod_info.PodsMap{
							pod_status.Pending: {
								"p1": {},
							},
						},
						PodInfos: pod_info.PodsMap{
							"p1": {},
						},
					},
				},
			},
		},
		{
			name: "single podgroup insert - one in queue. On pop comes second",
			fields: fields{
				jobsOrderInitOptions: JobsOrderInitOptions{
					VictimQueue:       false,
					FilterNonPending:  true,
					FilterUnready:     true,
					MaxJobsQueueDepth: scheduler_util.QueueCapacityInfinite,
				},
				Queues: map[common_info.QueueID]*queue_info.QueueInfo{
					"q1": {ParentQueue: "d1", UID: "q1"},
					"d1": {UID: "d1"},
				},
				InsertedJob: map[common_info.PodGroupID]*podgroup_info.PodGroupInfo{
					"p140": {
						Name:     "p140",
						UID:      "1",
						Priority: 150,
						Queue:    "q1",
						PodStatusIndex: map[pod_status.PodStatus]pod_info.PodsMap{
							pod_status.Pending: {
								"p1": {},
							},
						},
						PodInfos: pod_info.PodsMap{
							"p1": {},
						},
					},
				},
			},
			args: args{
				job: &podgroup_info.PodGroupInfo{
					Name:     "p150",
					UID:      "2",
					Priority: 150,
					Queue:    "q1",
					PodStatusIndex: map[pod_status.PodStatus]pod_info.PodsMap{
						pod_status.Pending: {
							"p1": {},
						},
					},
					PodInfos: pod_info.PodsMap{
						"p1": {},
					},
				},
			},
			expected: expected{
				expectedJobsList: []*podgroup_info.PodGroupInfo{
					{
						Name:     "p140",
						UID:      "1",
						Priority: 150,
						Queue:    "q1",
						PodStatusIndex: map[pod_status.PodStatus]pod_info.PodsMap{
							pod_status.Pending: {
								"p1": {},
							},
						},
						PodInfos: pod_info.PodsMap{
							"p1": {},
						},
					},
					{
						Name:     "p150",
						UID:      "2",
						Priority: 150,
						Queue:    "q1",
						PodStatusIndex: map[pod_status.PodStatus]pod_info.PodsMap{
							pod_status.Pending: {
								"p1": {},
							},
						},
						PodInfos: pod_info.PodsMap{
							"p1": {},
						},
					},
				},
			},
		},
		{
			name: "single podgroup insert - one in queue. On pop comes first",
			fields: fields{
				jobsOrderInitOptions: JobsOrderInitOptions{
					VictimQueue:       false,
					FilterNonPending:  true,
					FilterUnready:     true,
					MaxJobsQueueDepth: scheduler_util.QueueCapacityInfinite,
				},
				Queues: map[common_info.QueueID]*queue_info.QueueInfo{
					"q1": {ParentQueue: "d1", UID: "q1"},
					"d1": {UID: "d1"},
				},
				InsertedJob: map[common_info.PodGroupID]*podgroup_info.PodGroupInfo{
					"p140": {
						Name:     "p140",
						UID:      "1",
						Priority: 150,
						Queue:    "q1",
						PodStatusIndex: map[pod_status.PodStatus]pod_info.PodsMap{
							pod_status.Pending: {
								"p1": {},
							},
						},
						PodInfos: pod_info.PodsMap{
							"p1": {},
						},
					},
				},
			},
			args: args{
				job: &podgroup_info.PodGroupInfo{
					Name:     "p150",
					UID:      "2",
					Priority: 160,
					Queue:    "q1",
					PodStatusIndex: map[pod_status.PodStatus]pod_info.PodsMap{
						pod_status.Pending: {
							"p1": {},
						},
					},
					PodInfos: pod_info.PodsMap{
						"p1": {},
					},
				},
			},
			expected: expected{
				expectedJobsList: []*podgroup_info.PodGroupInfo{
					{
						Name:     "p150",
						UID:      "2",
						Priority: 160,
						Queue:    "q1",
						PodStatusIndex: map[pod_status.PodStatus]pod_info.PodsMap{
							pod_status.Pending: {
								"p1": {},
							},
						},
						PodInfos: pod_info.PodsMap{
							"p1": {},
						},
					},
					{
						Name:     "p140",
						UID:      "1",
						Priority: 150,
						Queue:    "q1",
						PodStatusIndex: map[pod_status.PodStatus]pod_info.PodsMap{
							pod_status.Pending: {
								"p1": {},
							},
						},
						PodInfos: pod_info.PodsMap{
							"p1": {},
						},
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ssn := newPrioritySession()
			ssn.Queues = tt.fields.Queues
			activeDepartments := scheduler_util.NewPriorityQueue(func(l, r interface{}) bool {
				if tt.fields.jobsOrderInitOptions.VictimQueue {
					return !ssn.JobOrderFn(l, r)
				}
				return ssn.JobOrderFn(l, r)
			}, scheduler_util.QueueCapacityInfinite)

			jobsOrder := &JobsOrderByQueues{
				activeDepartments:                activeDepartments,
				queueIdToQueueMetadata:           map[common_info.QueueID]*jobsQueueMetadata{},
				departmentIdToDepartmentMetadata: map[common_info.QueueID]*departmentMetadata{},
				ssn:                              ssn,
				jobsOrderInitOptions:             tt.fields.jobsOrderInitOptions,
			}
			jobsOrder.InitializeWithJobs(tt.fields.InsertedJob)
			jobsOrder.PushJob(tt.args.job)

			for _, expectedJob := range tt.expected.expectedJobsList {
				actualJob := jobsOrder.PopNextJob()
				assert.Equal(t, expectedJob, actualJob)
			}
		})
	}
}

func TestJobsOrderByQueues_RequeueJob(t *testing.T) {
	type fields struct {
		queueIdToQueueMetadata           map[common_info.QueueID]*jobsQueueMetadata
		departmentIdToDepartmentMetadata map[common_info.QueueID]*departmentMetadata
		jobsOrderInitOptions             JobsOrderInitOptions
		Queues                           map[common_info.QueueID]*queue_info.QueueInfo
		InsertedJob                      map[common_info.PodGroupID]*podgroup_info.PodGroupInfo
	}
	type expected struct {
		expectedJobsList []*podgroup_info.PodGroupInfo
	}
	tests := []struct {
		name     string
		fields   fields
		expected expected
	}{
		{
			name: "single job - pop and insert",
			fields: fields{
				queueIdToQueueMetadata:           map[common_info.QueueID]*jobsQueueMetadata{},
				departmentIdToDepartmentMetadata: map[common_info.QueueID]*departmentMetadata{},
				jobsOrderInitOptions: JobsOrderInitOptions{
					VictimQueue:       false,
					FilterNonPending:  true,
					FilterUnready:     true,
					MaxJobsQueueDepth: scheduler_util.QueueCapacityInfinite,
				},
				Queues: map[common_info.QueueID]*queue_info.QueueInfo{
					"q1": {ParentQueue: "d1", UID: "q1"},
					"d1": {UID: "d1"},
				},
				InsertedJob: map[common_info.PodGroupID]*podgroup_info.PodGroupInfo{
					"p140": {
						Name:     "p140",
						UID:      "1",
						Priority: 150,
						Queue:    "q1",
						PodStatusIndex: map[pod_status.PodStatus]pod_info.PodsMap{
							pod_status.Pending: {
								"p1": {},
							},
						},
						PodInfos: pod_info.PodsMap{
							"p1": {},
						},
					},
				},
			},
			expected: expected{
				expectedJobsList: []*podgroup_info.PodGroupInfo{
					{
						Name:     "p140",
						UID:      "1",
						Priority: 150,
						Queue:    "q1",
						PodStatusIndex: map[pod_status.PodStatus]pod_info.PodsMap{
							pod_status.Pending: {
								"p1": {},
							},
						},
						PodInfos: pod_info.PodsMap{
							"p1": {},
						},
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ssn := newPrioritySession()
			ssn.Queues = tt.fields.Queues
			activeDepartments := scheduler_util.NewPriorityQueue(func(l, r interface{}) bool {
				if tt.fields.jobsOrderInitOptions.VictimQueue {
					return !ssn.JobOrderFn(l, r)
				}
				return ssn.JobOrderFn(l, r)
			}, scheduler_util.QueueCapacityInfinite)

			jobsOrder := &JobsOrderByQueues{
				activeDepartments:                activeDepartments,
				queueIdToQueueMetadata:           tt.fields.queueIdToQueueMetadata,
				departmentIdToDepartmentMetadata: tt.fields.departmentIdToDepartmentMetadata,
				ssn:                              ssn,
				jobsOrderInitOptions:             tt.fields.jobsOrderInitOptions,
			}
			jobsOrder.InitializeWithJobs(tt.fields.InsertedJob)

			jobToRequeue := jobsOrder.PopNextJob()
			jobsOrder.PushJob(jobToRequeue)

			for _, expectedJob := range tt.expected.expectedJobsList {
				actualJob := jobsOrder.PopNextJob()
				assert.Equal(t, expectedJob, actualJob)
			}
		})
	}
}

func newPrioritySession() *framework.Session {
	return &framework.Session{
		JobOrderFns: []common_info.CompareFn{
			priority.JobOrderFn,
			elastic.JobOrderFn,
		},
		Config: &conf.SchedulerConfiguration{
			Tiers: []conf.Tier{
				{
					Plugins: []conf.PluginOption{
						{Name: "Priority"},
						{Name: "Elastic"},
						{Name: "Proportion"},
					},
				},
			},
			QueueDepthPerAction: map[string]int{},
		},
	}
}
