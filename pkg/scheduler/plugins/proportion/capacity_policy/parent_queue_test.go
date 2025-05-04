// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package capacity_policy

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/common_info"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/pod_info"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/pod_status"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/podgroup_info"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/queue_info"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/resource_info"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/constants"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/framework"
	rs "github.com/NVIDIA/KAI-scheduler/pkg/scheduler/plugins/proportion/resource_share"
)

func convertToQueueInfo(attrs *rs.QueueAttributes) *queue_info.QueueInfo {
	return &queue_info.QueueInfo{
		Name:        attrs.Name,
		ParentQueue: attrs.ParentQueue,
		Resources: queue_info.QueueQuota{
			GPU: queue_info.ResourceQuota{
				Quota: attrs.GPU.MaxAllowed,
			},
			CPU: queue_info.ResourceQuota{
				Quota: attrs.CPU.MaxAllowed,
			},
			Memory: queue_info.ResourceQuota{
				Quota: attrs.Memory.MaxAllowed,
			},
		},
	}
}

func TestParentQueueQuotaChecking(t *testing.T) {
	tests := []struct {
		name          string
		queues        map[common_info.QueueID]*rs.QueueAttributes
		job           *podgroup_info.PodGroupInfo
		expectedError bool
		errorContains string
	}{
		{
			name: "job within all quota limits",
			queues: map[common_info.QueueID]*rs.QueueAttributes{
				"parent": {
					Name: "parent",
					QueueResourceShare: rs.QueueResourceShare{
						GPU: rs.ResourceShare{
							MaxAllowed: 10,
						},
						CPU: rs.ResourceShare{
							MaxAllowed: 100,
						},
						Memory: rs.ResourceShare{
							MaxAllowed: 1024,
						},
					},
				},
				"child": {
					Name:        "child",
					ParentQueue: "parent",
				},
			},
			job: &podgroup_info.PodGroupInfo{
				Name:      "job-a",
				Namespace: "team-a",
				Queue:     "child",
				PodInfos: map[common_info.PodID]*pod_info.PodInfo{
					"pod-1": {
						Status: pod_status.Pending,
						ResReq: resource_info.NewResourceRequirements(5, 50, 512),
					},
				},
			},
			expectedError: false,
		},
		{
			name: "job exceeds parent queue GPU quota",
			queues: map[common_info.QueueID]*rs.QueueAttributes{
				"parent": {
					Name: "parent",
					QueueResourceShare: rs.QueueResourceShare{
						GPU: rs.ResourceShare{
							MaxAllowed: 5,
						},
					},
				},
				"child": {
					Name:        "child",
					ParentQueue: "parent",
				},
			},
			job: &podgroup_info.PodGroupInfo{
				Name:      "job-a",
				Namespace: "team-a",
				Queue:     "child",
				PodInfos: map[common_info.PodID]*pod_info.PodInfo{
					"pod-1": {
						Status: pod_status.Pending,
						ResReq: resource_info.NewResourceRequirements(15, 0, 0),
					},
				},
			},
			expectedError: true,
			errorContains: "parent queue 'parent' quota has reached the allowable limit of GPUs",
		},
		{
			name: "preemptible job can exceed parent queue GPU quota",
			queues: map[common_info.QueueID]*rs.QueueAttributes{
				"parent": {
					Name: "parent",
					QueueResourceShare: rs.QueueResourceShare{
						GPU: rs.ResourceShare{
							MaxAllowed: 5,
						},
					},
				},
				"child": {
					Name:        "child",
					ParentQueue: "parent",
				},
			},
			job: &podgroup_info.PodGroupInfo{
				Name:      "job-a",
				Namespace: "team-a",
				Queue:     "child",
				Priority:  constants.PriorityTrainNumber, // Preemptible job
				PodInfos: map[common_info.PodID]*pod_info.PodInfo{
					"pod-1": {
						Status: pod_status.Pending,
						ResReq: resource_info.NewResourceRequirements(15, 0, 0), // Requesting 15 GPUs
					},
				},
			},
			expectedError: false, // Should not error for preemptible jobs
		},
		{
			name: "non-preemptible job cannot exceed parent queue GPU quota",
			queues: map[common_info.QueueID]*rs.QueueAttributes{
				"parent": {
					Name: "parent",
					QueueResourceShare: rs.QueueResourceShare{
						GPU: rs.ResourceShare{
							MaxAllowed: 5,
						},
					},
				},
				"child": {
					Name:        "child",
					ParentQueue: "parent",
				},
			},
			job: &podgroup_info.PodGroupInfo{
				Name:      "job-a",
				Namespace: "team-a",
				Queue:     "child",
				Priority:  constants.PriorityBuildNumber, // Non-preemptible job
				PodInfos: map[common_info.PodID]*pod_info.PodInfo{
					"pod-1": {
						Status: pod_status.Pending,
						ResReq: resource_info.NewResourceRequirements(15, 0, 0), // Requesting 15 GPUs
					},
				},
			},
			expectedError: true,
			errorContains: "parent queue 'parent' quota has reached the allowable limit of GPUs",
		},
		{
			name: "job exceeds parent queue CPU quota",
			queues: map[common_info.QueueID]*rs.QueueAttributes{
				"parent": {
					Name: "parent",
					QueueResourceShare: rs.QueueResourceShare{
						CPU: rs.ResourceShare{
							MaxAllowed: 100,
						},
					},
				},
				"child": {
					Name:        "child",
					ParentQueue: "parent",
				},
			},
			job: &podgroup_info.PodGroupInfo{
				Name:      "job-a",
				Namespace: "team-a",
				Queue:     "child",
				PodInfos: map[common_info.PodID]*pod_info.PodInfo{
					"pod-1": {
						Status: pod_status.Pending,
						ResReq: resource_info.NewResourceRequirements(0, 150, 0),
					},
				},
			},
			expectedError: true,
			errorContains: "parent queue 'parent' quota has reached the allowable limit of CPU",
		},
		{
			name: "job exceeds parent queue Memory quota",
			queues: map[common_info.QueueID]*rs.QueueAttributes{
				"parent": {
					Name: "parent",
					QueueResourceShare: rs.QueueResourceShare{
						Memory: rs.ResourceShare{
							MaxAllowed: 1024,
						},
					},
				},
				"child": {
					Name:        "child",
					ParentQueue: "parent",
				},
			},
			job: &podgroup_info.PodGroupInfo{
				Name:      "job-a",
				Namespace: "team-a",
				Queue:     "child",
				PodInfos: map[common_info.PodID]*pod_info.PodInfo{
					"pod-1": {
						Status: pod_status.Pending,
						ResReq: resource_info.NewResourceRequirements(0, 0, 2048),
					},
				},
			},
			expectedError: true,
			errorContains: "parent queue 'parent' quota has reached the allowable limit of Memory",
		},
		{
			name: "multi-level queue hierarchy check with all resources",
			queues: map[common_info.QueueID]*rs.QueueAttributes{
				"top": {
					Name: "top",
					QueueResourceShare: rs.QueueResourceShare{
						GPU: rs.ResourceShare{
							MaxAllowed: 20,
						},
						CPU: rs.ResourceShare{
							MaxAllowed: 200,
						},
						Memory: rs.ResourceShare{
							MaxAllowed: 2048,
						},
					},
				},
				"mid": {
					Name:        "mid",
					ParentQueue: "top",
					QueueResourceShare: rs.QueueResourceShare{
						GPU: rs.ResourceShare{
							MaxAllowed: 10,
						},
						CPU: rs.ResourceShare{
							MaxAllowed: 100,
						},
						Memory: rs.ResourceShare{
							MaxAllowed: 1024,
						},
					},
				},
				"leaf": {
					Name:        "leaf",
					ParentQueue: "mid",
				},
			},
			job: &podgroup_info.PodGroupInfo{
				Name:      "job-a",
				Namespace: "team-a",
				Queue:     "leaf",
				PodInfos: map[common_info.PodID]*pod_info.PodInfo{
					"pod-1": {
						Status: pod_status.Pending,
						ResReq: resource_info.NewResourceRequirements(15, 150, 1500),
					},
				},
			},
			expectedError: true,
			errorContains: "parent queue 'mid' quota has reached the allowable limit",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cp := New(tt.queues, true) // isInferencePreemptible = true

			// Convert QueueAttributes to QueueInfo for the session
			queueInfos := make(map[common_info.QueueID]*queue_info.QueueInfo)
			for id, attrs := range tt.queues {
				queueInfos[id] = convertToQueueInfo(attrs)
			}

			ssn := &framework.Session{
				Queues: queueInfos,
			}

			err := cp.checkParentQueueQuotas(tt.job, ssn)

			if tt.expectedError {
				assert.Error(t, err)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestGetFirstPendingPod(t *testing.T) {
	tests := []struct {
		name     string
		job      *podgroup_info.PodGroupInfo
		expected bool
	}{
		{
			name: "job with pending pod",
			job: &podgroup_info.PodGroupInfo{
				PodInfos: map[common_info.PodID]*pod_info.PodInfo{
					"pod1": {
						Status: pod_status.Pending,
					},
					"pod2": {
						Status: pod_status.Running,
					},
				},
			},
			expected: true,
		},
		{
			name: "job with no pending pods",
			job: &podgroup_info.PodGroupInfo{
				PodInfos: map[common_info.PodID]*pod_info.PodInfo{
					"pod1": {
						Status: pod_status.Running,
					},
					"pod2": {
						Status: pod_status.Running,
					},
				},
			},
			expected: false,
		},
		{
			name: "empty job",
			job: &podgroup_info.PodGroupInfo{
				PodInfos: map[common_info.PodID]*pod_info.PodInfo{},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getFirstPendingPod(tt.job)
			if tt.expected {
				assert.NotNil(t, result)
				assert.Equal(t, pod_status.Pending, result.Status)
			} else {
				assert.Nil(t, result)
			}
		})
	}
}
