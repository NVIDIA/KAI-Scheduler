// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package capacity_policy

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/NVIDIA/KAI-scheduler/pkg/apis/scheduling/v2alpha2"
	commonconstants "github.com/NVIDIA/KAI-scheduler/pkg/common/constants"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/common_info"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/node_info"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/pod_info"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/pod_status"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/podgroup_info"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/resource_info"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/constants"
	rs "github.com/NVIDIA/KAI-scheduler/pkg/scheduler/plugins/proportion/resource_share"
)

var _ = Describe("Capacity Policy Check", func() {
	Describe("IsJobOverQueueCapacity", func() {
		Context("max allowed", func() {
			tests := map[string]struct {
				queues         map[common_info.QueueID]*rs.QueueAttributes
				job            *podgroup_info.PodGroupInfo
				expectedResult bool
			}{
				"limited queues - results below limit": {
					queues: map[common_info.QueueID]*rs.QueueAttributes{
						"top-queue": {
							UID:               "top-queue",
							Name:              "top-queue",
							ParentQueue:       "",
							ChildQueues:       []common_info.QueueID{"mid-queue"},
							CreationTimestamp: metav1.Time{},
							QueueResourceShare: rs.QueueResourceShare{
								GPU: rs.ResourceShare{
									MaxAllowed: 3,
									Allocated:  2,
								},
							},
						},
						"mid-queue": {
							UID:               "mid-queue",
							Name:              "mid-queue",
							ParentQueue:       "top-queue",
							ChildQueues:       []common_info.QueueID{"leaf-queue"},
							CreationTimestamp: metav1.Time{},
							QueueResourceShare: rs.QueueResourceShare{
								GPU: rs.ResourceShare{
									MaxAllowed: 3,
									Allocated:  2,
								},
							},
						},
						"leaf-queue": {
							UID:               "leaf-queue",
							Name:              "leaf-queue",
							ParentQueue:       "mid-queue",
							ChildQueues:       nil,
							CreationTimestamp: metav1.Time{},
							QueueResourceShare: rs.QueueResourceShare{
								GPU: rs.ResourceShare{
									MaxAllowed: 3,
									Allocated:  2,
								},
							},
						},
					},
					job: &podgroup_info.PodGroupInfo{
						Name:         "job-a",
						Namespace:    "team-a",
						Queue:        "leaf-queue",
						JobFitErrors: make(v2alpha2.UnschedulableExplanations, 0),
						PodInfos: map[common_info.PodID]*pod_info.PodInfo{
							"task-a": {
								UID:       "task-a",
								Job:       "job-a",
								Name:      "task-a",
								Namespace: "team-a",
								Status:    pod_status.Pending,
								ResReq:    resource_info.NewResourceRequirementsWithGpus(1),
							},
						},
					},
					expectedResult: true,
				},
				"limited queues - result over limit": {
					queues: map[common_info.QueueID]*rs.QueueAttributes{
						"top-queue": {
							UID:               "top-queue",
							Name:              "top-queue",
							ParentQueue:       "",
							ChildQueues:       []common_info.QueueID{"mid-queue"},
							CreationTimestamp: metav1.Time{},
							QueueResourceShare: rs.QueueResourceShare{
								GPU: rs.ResourceShare{
									MaxAllowed: 3,
									Allocated:  3,
								},
							},
						},
						"mid-queue": {
							UID:               "mid-queue",
							Name:              "mid-queue",
							ParentQueue:       "top-queue",
							ChildQueues:       []common_info.QueueID{"leaf-queue"},
							CreationTimestamp: metav1.Time{},
							QueueResourceShare: rs.QueueResourceShare{
								GPU: rs.ResourceShare{
									MaxAllowed: 3,
									Allocated:  2,
								},
							},
						},
						"leaf-queue": {
							UID:               "leaf-queue",
							Name:              "leaf-queue",
							ParentQueue:       "mid-queue",
							ChildQueues:       nil,
							CreationTimestamp: metav1.Time{},
							QueueResourceShare: rs.QueueResourceShare{
								GPU: rs.ResourceShare{
									MaxAllowed: 3,
									Allocated:  2,
								},
							},
						},
					},
					job: &podgroup_info.PodGroupInfo{
						Name:         "job-a",
						Namespace:    "team-a",
						Queue:        "leaf-queue",
						JobFitErrors: make(v2alpha2.UnschedulableExplanations, 0),
						PodInfos: map[common_info.PodID]*pod_info.PodInfo{
							"task-a": {
								UID:       "task-a",
								Job:       "job-a",
								Name:      "task-a",
								Namespace: "team-a",
								Status:    pod_status.Pending,
								ResReq:    resource_info.NewResourceRequirementsWithGpus(1),
							},
						},
					},
					expectedResult: false,
				},
			}

			for name, data := range tests {
				testName := name
				testData := data
				It(testName, func() {
					capacityPolicy := New(testData.queues, true)
					tasksToAllocate := podgroup_info.GetTasksToAllocate(testData.job, dummyTasksLessThen,
						true)
					result := capacityPolicy.IsJobOverQueueCapacity(testData.job, tasksToAllocate)
					Expect(result.IsSchedulable).To(Equal(testData.expectedResult))
				})
			}

		})
		Context("allocated non preemptible over quota", func() {
			tests := map[string]struct {
				queues         map[common_info.QueueID]*rs.QueueAttributes
				job            *podgroup_info.PodGroupInfo
				expectedResult bool
			}{
				"unlimited queues - preemptible job - allocated non preemptible below quota": {
					queues: map[common_info.QueueID]*rs.QueueAttributes{
						"top-queue": {
							UID:               "top-queue",
							Name:              "top-queue",
							ParentQueue:       "",
							ChildQueues:       []common_info.QueueID{"mid-queue"},
							CreationTimestamp: metav1.Time{},
							QueueResourceShare: rs.QueueResourceShare{
								GPU: rs.ResourceShare{
									MaxAllowed:              commonconstants.UnlimitedResourceQuantity,
									AllocatedNotPreemptible: 2,
									Deserved:                3,
								},
							},
						},
						"mid-queue": {
							UID:               "mid-queue",
							Name:              "mid-queue",
							ParentQueue:       "top-queue",
							ChildQueues:       []common_info.QueueID{"leaf-queue"},
							CreationTimestamp: metav1.Time{},
							QueueResourceShare: rs.QueueResourceShare{
								GPU: rs.ResourceShare{
									MaxAllowed:              commonconstants.UnlimitedResourceQuantity,
									AllocatedNotPreemptible: 2,
									Deserved:                3,
								},
							},
						},
						"leaf-queue": {
							UID:               "leaf-queue",
							Name:              "leaf-queue",
							ParentQueue:       "mid-queue",
							ChildQueues:       nil,
							CreationTimestamp: metav1.Time{},
							QueueResourceShare: rs.QueueResourceShare{
								GPU: rs.ResourceShare{
									MaxAllowed:              commonconstants.UnlimitedResourceQuantity,
									AllocatedNotPreemptible: 2,
									Deserved:                3,
								},
							},
						},
					},
					job: &podgroup_info.PodGroupInfo{
						Name:         "job-a",
						Namespace:    "team-a",
						Queue:        "leaf-queue",
						Priority:     constants.PriorityBuildNumber,
						JobFitErrors: make(v2alpha2.UnschedulableExplanations, 0),
						PodInfos: map[common_info.PodID]*pod_info.PodInfo{
							"task-a": {
								UID:       "task-a",
								Job:       "job-a",
								Name:      "task-a",
								Namespace: "team-a",
								Status:    pod_status.Pending,
								ResReq:    resource_info.NewResourceRequirementsWithGpus(1),
							},
						},
					},
					expectedResult: true,
				},
				"unlimited queues - preemptible job -  allocated non preemptible above quota": {
					queues: map[common_info.QueueID]*rs.QueueAttributes{
						"top-queue": {
							UID:               "top-queue",
							Name:              "top-queue",
							ParentQueue:       "",
							ChildQueues:       []common_info.QueueID{"mid-queue"},
							CreationTimestamp: metav1.Time{},
							QueueResourceShare: rs.QueueResourceShare{
								GPU: rs.ResourceShare{
									MaxAllowed:              commonconstants.UnlimitedResourceQuantity,
									AllocatedNotPreemptible: 3,
									Deserved:                3,
								},
							},
						},
						"mid-queue": {
							UID:               "mid-queue",
							Name:              "mid-queue",
							ParentQueue:       "top-queue",
							ChildQueues:       []common_info.QueueID{"leaf-queue"},
							CreationTimestamp: metav1.Time{},
							QueueResourceShare: rs.QueueResourceShare{
								GPU: rs.ResourceShare{
									MaxAllowed:              commonconstants.UnlimitedResourceQuantity,
									AllocatedNotPreemptible: 2,
									Deserved:                3,
								},
							},
						},
						"leaf-queue": {
							UID:               "leaf-queue",
							Name:              "leaf-queue",
							ParentQueue:       "mid-queue",
							ChildQueues:       nil,
							CreationTimestamp: metav1.Time{},
							QueueResourceShare: rs.QueueResourceShare{
								GPU: rs.ResourceShare{
									MaxAllowed:              commonconstants.UnlimitedResourceQuantity,
									AllocatedNotPreemptible: 2,
									Deserved:                3,
								},
							},
						},
					},
					job: &podgroup_info.PodGroupInfo{
						Name:         "job-a",
						Namespace:    "team-a",
						Queue:        "leaf-queue",
						Priority:     constants.PriorityBuildNumber,
						JobFitErrors: make(v2alpha2.UnschedulableExplanations, 0),
						PodInfos: map[common_info.PodID]*pod_info.PodInfo{
							"task-a": {
								UID:       "task-a",
								Job:       "job-a",
								Name:      "task-a",
								Namespace: "team-a",
								Status:    pod_status.Pending,
								ResReq:    resource_info.NewResourceRequirementsWithGpus(1),
							},
						},
					},
					expectedResult: false,
				},
			}

			for name, data := range tests {
				testName := name
				testData := data
				It(testName, func() {
					capacityPolicy := New(testData.queues, true)
					tasksToAllocate := podgroup_info.GetTasksToAllocate(testData.job, dummyTasksLessThen,
						true)
					result := capacityPolicy.IsJobOverQueueCapacity(testData.job, tasksToAllocate)
					Expect(result.IsSchedulable).To(Equal(testData.expectedResult))
				})
			}

		})
	})

	Describe("IsNonPreemptibleJobOverQuota", func() {
		Context("allocated non preemptible over quota", func() {
			tests := map[string]struct {
				queues         map[common_info.QueueID]*rs.QueueAttributes
				job            *podgroup_info.PodGroupInfo
				expectedResult bool
			}{
				"unlimited queues - preemptible job - allocated non preemptible below quota": {
					queues: map[common_info.QueueID]*rs.QueueAttributes{
						"top-queue": {
							UID:               "top-queue",
							Name:              "top-queue",
							ParentQueue:       "",
							ChildQueues:       []common_info.QueueID{"mid-queue"},
							CreationTimestamp: metav1.Time{},
							QueueResourceShare: rs.QueueResourceShare{
								GPU: rs.ResourceShare{
									MaxAllowed:              commonconstants.UnlimitedResourceQuantity,
									AllocatedNotPreemptible: 2,
									Deserved:                3,
								},
							},
						},
						"mid-queue": {
							UID:               "mid-queue",
							Name:              "mid-queue",
							ParentQueue:       "top-queue",
							ChildQueues:       []common_info.QueueID{"leaf-queue"},
							CreationTimestamp: metav1.Time{},
							QueueResourceShare: rs.QueueResourceShare{
								GPU: rs.ResourceShare{
									MaxAllowed:              commonconstants.UnlimitedResourceQuantity,
									AllocatedNotPreemptible: 2,
									Deserved:                3,
								},
							},
						},
						"leaf-queue": {
							UID:               "leaf-queue",
							Name:              "leaf-queue",
							ParentQueue:       "mid-queue",
							ChildQueues:       nil,
							CreationTimestamp: metav1.Time{},
							QueueResourceShare: rs.QueueResourceShare{
								GPU: rs.ResourceShare{
									MaxAllowed:              commonconstants.UnlimitedResourceQuantity,
									AllocatedNotPreemptible: 2,
									Deserved:                3,
								},
							},
						},
					},
					job: &podgroup_info.PodGroupInfo{
						Name:         "job-a",
						Namespace:    "team-a",
						Queue:        "leaf-queue",
						Priority:     constants.PriorityBuildNumber,
						JobFitErrors: make(v2alpha2.UnschedulableExplanations, 0),
						PodInfos: map[common_info.PodID]*pod_info.PodInfo{
							"task-a": {
								UID:       "task-a",
								Job:       "job-a",
								Name:      "task-a",
								Namespace: "team-a",
								Status:    pod_status.Pending,
								ResReq:    resource_info.NewResourceRequirementsWithGpus(1),
							},
						},
					},
					expectedResult: true,
				},
				"unlimited queues - preemptible job -  allocated non preemptible above quota": {
					queues: map[common_info.QueueID]*rs.QueueAttributes{
						"top-queue": {
							UID:               "top-queue",
							Name:              "top-queue",
							ParentQueue:       "",
							ChildQueues:       []common_info.QueueID{"mid-queue"},
							CreationTimestamp: metav1.Time{},
							QueueResourceShare: rs.QueueResourceShare{
								GPU: rs.ResourceShare{
									MaxAllowed:              commonconstants.UnlimitedResourceQuantity,
									AllocatedNotPreemptible: 3,
									Deserved:                3,
								},
							},
						},
						"mid-queue": {
							UID:               "mid-queue",
							Name:              "mid-queue",
							ParentQueue:       "top-queue",
							ChildQueues:       []common_info.QueueID{"leaf-queue"},
							CreationTimestamp: metav1.Time{},
							QueueResourceShare: rs.QueueResourceShare{
								GPU: rs.ResourceShare{
									MaxAllowed:              commonconstants.UnlimitedResourceQuantity,
									AllocatedNotPreemptible: 2,
									Deserved:                3,
								},
							},
						},
						"leaf-queue": {
							UID:               "leaf-queue",
							Name:              "leaf-queue",
							ParentQueue:       "mid-queue",
							ChildQueues:       nil,
							CreationTimestamp: metav1.Time{},
							QueueResourceShare: rs.QueueResourceShare{
								GPU: rs.ResourceShare{
									MaxAllowed:              commonconstants.UnlimitedResourceQuantity,
									AllocatedNotPreemptible: 2,
									Deserved:                3,
								},
							},
						},
					},
					job: &podgroup_info.PodGroupInfo{
						Name:         "job-a",
						Namespace:    "team-a",
						Queue:        "leaf-queue",
						Priority:     constants.PriorityBuildNumber,
						JobFitErrors: make(v2alpha2.UnschedulableExplanations, 0),
						PodInfos: map[common_info.PodID]*pod_info.PodInfo{
							"task-a": {
								UID:       "task-a",
								Job:       "job-a",
								Name:      "task-a",
								Namespace: "team-a",
								Status:    pod_status.Pending,
								ResReq:    resource_info.NewResourceRequirementsWithGpus(1),
							},
						},
					},
					expectedResult: false,
				},
			}

			for name, data := range tests {
				testName := name
				testData := data
				It(testName, func() {
					capacityPolicy := New(testData.queues, true)
					tasksToAllocate := podgroup_info.GetTasksToAllocate(testData.job, dummyTasksLessThen,
						true)
					result := capacityPolicy.IsNonPreemptibleJobOverQuota(testData.job, tasksToAllocate)
					Expect(result.IsSchedulable).To(Equal(testData.expectedResult))
				})
			}

		})
	})

	Describe("IsTaskAllocationOnNodeOverCapacity", func() {
		Context("allocated non preemptible over quota", func() {
			tests := map[string]struct {
				queues         map[common_info.QueueID]*rs.QueueAttributes
				job            *podgroup_info.PodGroupInfo
				node           *node_info.NodeInfo
				expectedResult bool
			}{
				"unlimited queues - allocated non preemptible job below quota": {
					queues: map[common_info.QueueID]*rs.QueueAttributes{
						"top-queue": {
							UID:               "top-queue",
							Name:              "top-queue",
							ParentQueue:       "",
							ChildQueues:       []common_info.QueueID{"mid-queue"},
							CreationTimestamp: metav1.Time{},
							QueueResourceShare: rs.QueueResourceShare{
								CPU: rs.ResourceShare{
									MaxAllowed:              commonconstants.UnlimitedResourceQuantity,
									AllocatedNotPreemptible: 2000,
									Deserved:                3000,
								},
							},
						},
						"mid-queue": {
							UID:               "mid-queue",
							Name:              "mid-queue",
							ParentQueue:       "top-queue",
							ChildQueues:       []common_info.QueueID{"leaf-queue"},
							CreationTimestamp: metav1.Time{},
							QueueResourceShare: rs.QueueResourceShare{
								CPU: rs.ResourceShare{
									MaxAllowed:              commonconstants.UnlimitedResourceQuantity,
									AllocatedNotPreemptible: 2000,
									Deserved:                3000,
								},
							},
						},
						"leaf-queue": {
							UID:               "leaf-queue",
							Name:              "leaf-queue",
							ParentQueue:       "mid-queue",
							ChildQueues:       nil,
							CreationTimestamp: metav1.Time{},
							QueueResourceShare: rs.QueueResourceShare{
								CPU: rs.ResourceShare{
									MaxAllowed:              commonconstants.UnlimitedResourceQuantity,
									AllocatedNotPreemptible: 2000,
									Deserved:                3000,
								},
							},
						},
					},
					job: &podgroup_info.PodGroupInfo{
						Name:         "job-a",
						Namespace:    "team-a",
						Queue:        "leaf-queue",
						Priority:     constants.PriorityBuildNumber,
						JobFitErrors: make(v2alpha2.UnschedulableExplanations, 0),
						PodInfos: map[common_info.PodID]*pod_info.PodInfo{
							"task-a": {
								UID:       "task-a",
								Job:       "job-a",
								Name:      "task-a",
								Namespace: "team-a",
								Status:    pod_status.Pending,
								ResReq:    resource_info.NewResourceRequirements(0, 1000, 0),
							},
						},
					},
					node: &node_info.NodeInfo{
						Name: "worker-node",
					},
					expectedResult: true,
				},
				"unlimited queues - allocated non preemptible job above quota": {
					queues: map[common_info.QueueID]*rs.QueueAttributes{
						"top-queue": {
							UID:               "top-queue",
							Name:              "top-queue",
							ParentQueue:       "",
							ChildQueues:       []common_info.QueueID{"mid-queue"},
							CreationTimestamp: metav1.Time{},
							QueueResourceShare: rs.QueueResourceShare{
								CPU: rs.ResourceShare{
									MaxAllowed:              commonconstants.UnlimitedResourceQuantity,
									AllocatedNotPreemptible: 3000,
									Deserved:                3000,
								},
							},
						},
						"mid-queue": {
							UID:               "mid-queue",
							Name:              "mid-queue",
							ParentQueue:       "top-queue",
							ChildQueues:       []common_info.QueueID{"leaf-queue"},
							CreationTimestamp: metav1.Time{},
							QueueResourceShare: rs.QueueResourceShare{
								CPU: rs.ResourceShare{
									MaxAllowed:              commonconstants.UnlimitedResourceQuantity,
									AllocatedNotPreemptible: 2000,
									Deserved:                3000,
								},
							},
						},
						"leaf-queue": {
							UID:               "leaf-queue",
							Name:              "leaf-queue",
							ParentQueue:       "mid-queue",
							ChildQueues:       nil,
							CreationTimestamp: metav1.Time{},
							QueueResourceShare: rs.QueueResourceShare{
								CPU: rs.ResourceShare{
									MaxAllowed:              commonconstants.UnlimitedResourceQuantity,
									AllocatedNotPreemptible: 2000,
									Deserved:                3000,
								},
							},
						},
					},
					job: &podgroup_info.PodGroupInfo{
						Name:         "job-a",
						Namespace:    "team-a",
						Queue:        "leaf-queue",
						Priority:     constants.PriorityBuildNumber,
						JobFitErrors: make(v2alpha2.UnschedulableExplanations, 0),
						PodInfos: map[common_info.PodID]*pod_info.PodInfo{
							"task-a": {
								UID:       "task-a",
								Job:       "job-a",
								Name:      "task-a",
								Namespace: "team-a",
								Status:    pod_status.Pending,
								ResReq:    resource_info.NewResourceRequirements(0, 1000, 0),
							},
						},
					},
					node: &node_info.NodeInfo{
						Name: "worker-node",
					},
					expectedResult: false,
				},
				"unlimited queues - allocated preemptible job below quota": {
					queues: map[common_info.QueueID]*rs.QueueAttributes{
						"top-queue": {
							UID:               "top-queue",
							Name:              "top-queue",
							ParentQueue:       "",
							ChildQueues:       []common_info.QueueID{"mid-queue"},
							CreationTimestamp: metav1.Time{},
							QueueResourceShare: rs.QueueResourceShare{
								CPU: rs.ResourceShare{
									MaxAllowed:              commonconstants.UnlimitedResourceQuantity,
									AllocatedNotPreemptible: 2000,
									Deserved:                3000,
								},
							},
						},
						"mid-queue": {
							UID:               "mid-queue",
							Name:              "mid-queue",
							ParentQueue:       "top-queue",
							ChildQueues:       []common_info.QueueID{"leaf-queue"},
							CreationTimestamp: metav1.Time{},
							QueueResourceShare: rs.QueueResourceShare{
								CPU: rs.ResourceShare{
									MaxAllowed:              commonconstants.UnlimitedResourceQuantity,
									AllocatedNotPreemptible: 2000,
									Deserved:                3000,
								},
							},
						},
						"leaf-queue": {
							UID:               "leaf-queue",
							Name:              "leaf-queue",
							ParentQueue:       "mid-queue",
							ChildQueues:       nil,
							CreationTimestamp: metav1.Time{},
							QueueResourceShare: rs.QueueResourceShare{
								CPU: rs.ResourceShare{
									MaxAllowed:              commonconstants.UnlimitedResourceQuantity,
									AllocatedNotPreemptible: 2000,
									Deserved:                3000,
								},
							},
						},
					},
					job: &podgroup_info.PodGroupInfo{
						Name:         "job-a",
						Namespace:    "team-a",
						Queue:        "leaf-queue",
						Priority:     constants.PriorityTrainNumber,
						JobFitErrors: make(v2alpha2.UnschedulableExplanations, 0),
						PodInfos: map[common_info.PodID]*pod_info.PodInfo{
							"task-a": {
								UID:       "task-a",
								Job:       "job-a",
								Name:      "task-a",
								Namespace: "team-a",
								Status:    pod_status.Pending,
								ResReq:    resource_info.NewResourceRequirements(0, 1000, 0),
							},
						},
					},
					node: &node_info.NodeInfo{
						Name: "worker-node",
					},
					expectedResult: true,
				},
				"unlimited queues - allocated preemptible job above quota": {
					queues: map[common_info.QueueID]*rs.QueueAttributes{
						"top-queue": {
							UID:               "top-queue",
							Name:              "top-queue",
							ParentQueue:       "",
							ChildQueues:       []common_info.QueueID{"mid-queue"},
							CreationTimestamp: metav1.Time{},
							QueueResourceShare: rs.QueueResourceShare{
								CPU: rs.ResourceShare{
									MaxAllowed:              commonconstants.UnlimitedResourceQuantity,
									AllocatedNotPreemptible: 3000,
									Deserved:                3000,
								},
							},
						},
						"mid-queue": {
							UID:               "mid-queue",
							Name:              "mid-queue",
							ParentQueue:       "top-queue",
							ChildQueues:       []common_info.QueueID{"leaf-queue"},
							CreationTimestamp: metav1.Time{},
							QueueResourceShare: rs.QueueResourceShare{
								CPU: rs.ResourceShare{
									MaxAllowed:              commonconstants.UnlimitedResourceQuantity,
									AllocatedNotPreemptible: 2000,
									Deserved:                3000,
								},
							},
						},
						"leaf-queue": {
							UID:               "leaf-queue",
							Name:              "leaf-queue",
							ParentQueue:       "mid-queue",
							ChildQueues:       nil,
							CreationTimestamp: metav1.Time{},
							QueueResourceShare: rs.QueueResourceShare{
								CPU: rs.ResourceShare{
									MaxAllowed:              commonconstants.UnlimitedResourceQuantity,
									AllocatedNotPreemptible: 2000,
									Deserved:                3000,
								},
							},
						},
					},
					job: &podgroup_info.PodGroupInfo{
						Name:         "job-a",
						Namespace:    "team-a",
						Queue:        "leaf-queue",
						Priority:     constants.PriorityTrainNumber,
						JobFitErrors: make(v2alpha2.UnschedulableExplanations, 0),
						PodInfos: map[common_info.PodID]*pod_info.PodInfo{
							"task-a": {
								UID:       "task-a",
								Job:       "job-a",
								Name:      "task-a",
								Namespace: "team-a",
								Status:    pod_status.Pending,
								ResReq:    resource_info.NewResourceRequirements(0, 1000, 0),
							},
						},
					},
					node: &node_info.NodeInfo{
						Name: "worker-node",
					},
					expectedResult: true,
				},
				"limited queue -  allocated non preemptible job below limit": {
					queues: map[common_info.QueueID]*rs.QueueAttributes{
						"top-queue": {
							UID:               "top-queue",
							Name:              "top-queue",
							ParentQueue:       "",
							ChildQueues:       []common_info.QueueID{"mid-queue"},
							CreationTimestamp: metav1.Time{},
							QueueResourceShare: rs.QueueResourceShare{
								CPU: rs.ResourceShare{
									MaxAllowed:              3000,
									Allocated:               2000,
									AllocatedNotPreemptible: 2000,
									Deserved:                3000,
								},
							},
						},
						"mid-queue": {
							UID:               "mid-queue",
							Name:              "mid-queue",
							ParentQueue:       "top-queue",
							ChildQueues:       []common_info.QueueID{"leaf-queue"},
							CreationTimestamp: metav1.Time{},
							QueueResourceShare: rs.QueueResourceShare{
								CPU: rs.ResourceShare{
									MaxAllowed:              3000,
									Allocated:               2000,
									AllocatedNotPreemptible: 2000,
									Deserved:                3000,
								},
							},
						},
						"leaf-queue": {
							UID:               "leaf-queue",
							Name:              "leaf-queue",
							ParentQueue:       "mid-queue",
							ChildQueues:       nil,
							CreationTimestamp: metav1.Time{},
							QueueResourceShare: rs.QueueResourceShare{
								CPU: rs.ResourceShare{
									MaxAllowed:              3000,
									Allocated:               2000,
									AllocatedNotPreemptible: 2000,
									Deserved:                3000,
								},
							},
						},
					},
					job: &podgroup_info.PodGroupInfo{
						Name:         "job-a",
						Namespace:    "team-a",
						Queue:        "leaf-queue",
						Priority:     constants.PriorityBuildNumber,
						JobFitErrors: make(v2alpha2.UnschedulableExplanations, 0),
						PodInfos: map[common_info.PodID]*pod_info.PodInfo{
							"task-a": {
								UID:       "task-a",
								Job:       "job-a",
								Name:      "task-a",
								Namespace: "team-a",
								Status:    pod_status.Pending,
								ResReq:    resource_info.NewResourceRequirements(0, 500, 0),
							},
						},
					},
					node: &node_info.NodeInfo{
						Name: "worker-node",
					},
					expectedResult: true,
				},
				"limited queue -  allocated non preemptible job above limit": {
					queues: map[common_info.QueueID]*rs.QueueAttributes{
						"top-queue": {
							UID:               "top-queue",
							Name:              "top-queue",
							ParentQueue:       "",
							ChildQueues:       []common_info.QueueID{"mid-queue"},
							CreationTimestamp: metav1.Time{},
							QueueResourceShare: rs.QueueResourceShare{
								CPU: rs.ResourceShare{
									MaxAllowed:              3000,
									Allocated:               2000,
									AllocatedNotPreemptible: 2000,
									Deserved:                4000,
								},
							},
						},
						"mid-queue": {
							UID:               "mid-queue",
							Name:              "mid-queue",
							ParentQueue:       "top-queue",
							ChildQueues:       []common_info.QueueID{"leaf-queue"},
							CreationTimestamp: metav1.Time{},
							QueueResourceShare: rs.QueueResourceShare{
								CPU: rs.ResourceShare{
									MaxAllowed:              3000,
									Allocated:               2000,
									AllocatedNotPreemptible: 2000,
									Deserved:                4000,
								},
							},
						},
						"leaf-queue": {
							UID:               "leaf-queue",
							Name:              "leaf-queue",
							ParentQueue:       "mid-queue",
							ChildQueues:       nil,
							CreationTimestamp: metav1.Time{},
							QueueResourceShare: rs.QueueResourceShare{
								CPU: rs.ResourceShare{
									MaxAllowed:              3000,
									Allocated:               2000,
									AllocatedNotPreemptible: 2000,
									Deserved:                4000,
								},
							},
						},
					},
					job: &podgroup_info.PodGroupInfo{
						Name:         "job-a",
						Namespace:    "team-a",
						Queue:        "leaf-queue",
						Priority:     constants.PriorityBuildNumber,
						JobFitErrors: make(v2alpha2.UnschedulableExplanations, 0),
						PodInfos: map[common_info.PodID]*pod_info.PodInfo{
							"task-a": {
								UID:       "task-a",
								Job:       "job-a",
								Name:      "task-a",
								Namespace: "team-a",
								Status:    pod_status.Pending,
								ResReq:    resource_info.NewResourceRequirements(0, 1100, 0),
							},
						},
					},
					node: &node_info.NodeInfo{
						Name: "worker-node",
					},
					expectedResult: false,
				},
				"limited queue -  allocated preemptible job below limit": {
					queues: map[common_info.QueueID]*rs.QueueAttributes{
						"top-queue": {
							UID:               "top-queue",
							Name:              "top-queue",
							ParentQueue:       "",
							ChildQueues:       []common_info.QueueID{"mid-queue"},
							CreationTimestamp: metav1.Time{},
							QueueResourceShare: rs.QueueResourceShare{
								CPU: rs.ResourceShare{
									MaxAllowed:              3000,
									Allocated:               2000,
									AllocatedNotPreemptible: 2000,
									Deserved:                3000,
								},
							},
						},
						"mid-queue": {
							UID:               "mid-queue",
							Name:              "mid-queue",
							ParentQueue:       "top-queue",
							ChildQueues:       []common_info.QueueID{"leaf-queue"},
							CreationTimestamp: metav1.Time{},
							QueueResourceShare: rs.QueueResourceShare{
								CPU: rs.ResourceShare{
									MaxAllowed:              3000,
									Allocated:               2000,
									AllocatedNotPreemptible: 2000,
									Deserved:                3000,
								},
							},
						},
						"leaf-queue": {
							UID:               "leaf-queue",
							Name:              "leaf-queue",
							ParentQueue:       "mid-queue",
							ChildQueues:       nil,
							CreationTimestamp: metav1.Time{},
							QueueResourceShare: rs.QueueResourceShare{
								CPU: rs.ResourceShare{
									MaxAllowed:              3000,
									Allocated:               2000,
									AllocatedNotPreemptible: 2000,
									Deserved:                3000,
								},
							},
						},
					},
					job: &podgroup_info.PodGroupInfo{
						Name:         "job-a",
						Namespace:    "team-a",
						Queue:        "leaf-queue",
						Priority:     constants.PriorityTrainNumber,
						JobFitErrors: make(v2alpha2.UnschedulableExplanations, 0),
						PodInfos: map[common_info.PodID]*pod_info.PodInfo{
							"task-a": {
								UID:       "task-a",
								Job:       "job-a",
								Name:      "task-a",
								Namespace: "team-a",
								Status:    pod_status.Pending,
								ResReq:    resource_info.NewResourceRequirements(0, 500, 0),
							},
						},
					},
					node: &node_info.NodeInfo{
						Name: "worker-node",
					},
					expectedResult: true,
				},
				"limited queue -  allocated preemptible job above limit": {
					queues: map[common_info.QueueID]*rs.QueueAttributes{
						"top-queue": {
							UID:               "top-queue",
							Name:              "top-queue",
							ParentQueue:       "",
							ChildQueues:       []common_info.QueueID{"mid-queue"},
							CreationTimestamp: metav1.Time{},
							QueueResourceShare: rs.QueueResourceShare{
								CPU: rs.ResourceShare{
									MaxAllowed:              3000,
									Allocated:               2000,
									AllocatedNotPreemptible: 2000,
									Deserved:                4000,
								},
							},
						},
						"mid-queue": {
							UID:               "mid-queue",
							Name:              "mid-queue",
							ParentQueue:       "top-queue",
							ChildQueues:       []common_info.QueueID{"leaf-queue"},
							CreationTimestamp: metav1.Time{},
							QueueResourceShare: rs.QueueResourceShare{
								CPU: rs.ResourceShare{
									MaxAllowed:              3000,
									Allocated:               2000,
									AllocatedNotPreemptible: 2000,
									Deserved:                4000,
								},
							},
						},
						"leaf-queue": {
							UID:               "leaf-queue",
							Name:              "leaf-queue",
							ParentQueue:       "mid-queue",
							ChildQueues:       nil,
							CreationTimestamp: metav1.Time{},
							QueueResourceShare: rs.QueueResourceShare{
								CPU: rs.ResourceShare{
									MaxAllowed:              3000,
									Allocated:               2000,
									AllocatedNotPreemptible: 2000,
									Deserved:                4000,
								},
							},
						},
					},
					job: &podgroup_info.PodGroupInfo{
						Name:         "job-a",
						Namespace:    "team-a",
						Queue:        "leaf-queue",
						Priority:     constants.PriorityTrainNumber,
						JobFitErrors: make(v2alpha2.UnschedulableExplanations, 0),
						PodInfos: map[common_info.PodID]*pod_info.PodInfo{
							"task-a": {
								UID:       "task-a",
								Job:       "job-a",
								Name:      "task-a",
								Namespace: "team-a",
								Status:    pod_status.Pending,
								ResReq:    resource_info.NewResourceRequirements(0, 1100, 0),
							},
						},
					},
					node: &node_info.NodeInfo{
						Name: "worker-node",
					},
					expectedResult: false,
				},
			}

			for name, data := range tests {
				testName := name
				testData := data
				It(testName, func() {
					capacityPolicy := New(testData.queues, true)
					result := capacityPolicy.IsTaskAllocationOnNodeOverCapacity(testData.job.PodInfos["task-a"],
						testData.job, testData.node)
					Expect(result.IsSchedulable).To(Equal(testData.expectedResult))
				})
			}

		})
	})
})

func dummyTasksLessThen(_ interface{}, _ interface{}) bool {
	return false
}
