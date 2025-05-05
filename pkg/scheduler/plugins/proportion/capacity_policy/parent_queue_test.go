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
				Limit: attrs.QueueResourceShare.GPU.MaxAllowed,
				Quota: 5, // Set a default quota for testing
			},
			CPU: queue_info.ResourceQuota{
				Limit: attrs.QueueResourceShare.CPU.MaxAllowed,
				Quota: 50, // Set a default quota for testing
			},
			Memory: queue_info.ResourceQuota{
				Limit: attrs.QueueResourceShare.Memory.MaxAllowed,
				Quota: 512, // Set a default quota for testing
			},
		},
	}
}

func TestParentQueueLimitChecking(t *testing.T) {
	tests := []struct {
		name          string
		queues        map[common_info.QueueID]*rs.QueueAttributes
		job           *podgroup_info.PodGroupInfo
		expectedError bool
		errorContains string
	}{
		{
			name: "job within all limit limits",
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
			name: "job exceeds parent queue GPU limit",
			queues: map[common_info.QueueID]*rs.QueueAttributes{
				"parent": {
					Name: "parent",
					QueueResourceShare: rs.QueueResourceShare{
						GPU: rs.ResourceShare{
							MaxAllowed: 5,
						},
						CPU: rs.ResourceShare{
							MaxAllowed: 0,
						},
						Memory: rs.ResourceShare{
							MaxAllowed: 0,
						},
					},
				},
				"child": {
					Name:        "child",
					ParentQueue: "parent",
					QueueResourceShare: rs.QueueResourceShare{
						GPU: rs.ResourceShare{
							MaxAllowed: 0,
						},
						CPU: rs.ResourceShare{
							MaxAllowed: 0,
						},
						Memory: rs.ResourceShare{
							MaxAllowed: 0,
						},
					},
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
						ResReq: resource_info.NewResourceRequirements(15, 0, 0),
					},
				},
			},
			expectedError: true,
			errorContains: "parent queue 'parent' has reached the limit/quota of GPU. Value is 5, workload requested 15",
		},
		{
			name: "elastic job with minimum required resources",
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
				Name:         "job-a",
				Namespace:    "team-a",
				Queue:        "child",
				MinAvailable: 2, // Elastic job with minimum 2 pods
				PodInfos: map[common_info.PodID]*pod_info.PodInfo{
					"pod-1": {
						Status: pod_status.Pending,
						ResReq: resource_info.NewResourceRequirements(2, 0, 0),
					},
					"pod-2": {
						Status: pod_status.Pending,
						ResReq: resource_info.NewResourceRequirements(2, 0, 0),
					},
				},
			},
			expectedError: false, // Should pass as minimum required resources are within limits
		},
		{
			name: "preemptible job can exceed parent queue GPU limit",
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
				Priority:  constants.PriorityInferenceNumber, // Preemptible job
				PodInfos: map[common_info.PodID]*pod_info.PodInfo{
					"pod-1": {
						Status: pod_status.Pending,
						ResReq: resource_info.NewResourceRequirements(15, 0, 0),
					},
				},
			},
			expectedError: false, // Preemptible jobs can exceed limits
		},
		{
			name: "non-preemptible job cannot exceed parent queue GPU limit",
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
						ResReq: resource_info.NewResourceRequirements(15, 0, 0),
					},
				},
			},
			expectedError: true,
			errorContains: "parent queue 'parent' has reached the limit/quota of GPU. Value is 5, workload requested 15",
		},
		{
			name: "job exceeds parent queue CPU limit",
			queues: map[common_info.QueueID]*rs.QueueAttributes{
				"parent": {
					Name: "parent",
					QueueResourceShare: rs.QueueResourceShare{
						GPU: rs.ResourceShare{
							MaxAllowed: 0,
						},
						CPU: rs.ResourceShare{
							MaxAllowed: 100,
						},
						Memory: rs.ResourceShare{
							MaxAllowed: 0,
						},
					},
				},
				"child": {
					Name:        "child",
					ParentQueue: "parent",
					QueueResourceShare: rs.QueueResourceShare{
						GPU: rs.ResourceShare{
							MaxAllowed: 0,
						},
						CPU: rs.ResourceShare{
							MaxAllowed: 0,
						},
						Memory: rs.ResourceShare{
							MaxAllowed: 0,
						},
					},
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
						ResReq: resource_info.NewResourceRequirements(0, 150, 0),
					},
				},
			},
			expectedError: true,
			errorContains: "parent queue 'parent' has reached the limit/quota of CPU. Value is 50, workload requested 150",
		},
		{
			name: "job exceeds parent queue Memory limit",
			queues: map[common_info.QueueID]*rs.QueueAttributes{
				"parent": {
					Name: "parent",
					QueueResourceShare: rs.QueueResourceShare{
						GPU: rs.ResourceShare{
							MaxAllowed: 0,
						},
						CPU: rs.ResourceShare{
							MaxAllowed: 0,
						},
						Memory: rs.ResourceShare{
							MaxAllowed: 1024,
						},
					},
				},
				"child": {
					Name:        "child",
					ParentQueue: "parent",
					QueueResourceShare: rs.QueueResourceShare{
						GPU: rs.ResourceShare{
							MaxAllowed: 0,
						},
						CPU: rs.ResourceShare{
							MaxAllowed: 0,
						},
						Memory: rs.ResourceShare{
							MaxAllowed: 0,
						},
					},
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
						ResReq: resource_info.NewResourceRequirements(0, 0, 2048),
					},
				},
			},
			expectedError: true,
			errorContains: "parent queue 'parent' has reached the limit/quota of Memory. Value is 512, workload requested 2048",
		},
		{
			name: "multi-level queue hierarchy check with all resources",
			queues: map[common_info.QueueID]*rs.QueueAttributes{
				"root": {
					Name: "root",
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
					ParentQueue: "root",
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
					QueueResourceShare: rs.QueueResourceShare{
						GPU: rs.ResourceShare{
							MaxAllowed: 0,
						},
						CPU: rs.ResourceShare{
							MaxAllowed: 0,
						},
						Memory: rs.ResourceShare{
							MaxAllowed: 0,
						},
					},
				},
			},
			job: &podgroup_info.PodGroupInfo{
				Name:      "job-a",
				Namespace: "team-a",
				Queue:     "leaf",
				Priority:  constants.PriorityBuildNumber, // Non-preemptible job
				PodInfos: map[common_info.PodID]*pod_info.PodInfo{
					"pod-1": {
						Status: pod_status.Pending,
						ResReq: resource_info.NewResourceRequirements(15, 150, 1536),
					},
				},
			},
			expectedError: true,
			errorContains: "parent queue 'mid' has reached the limit/quota of GPU. Value is 5, workload requested 15",
		},
		{
			name: "job exceeds parent queue GPU quota (no limit)",
			queues: map[common_info.QueueID]*rs.QueueAttributes{
				"parent": {
					Name: "parent",
					QueueResourceShare: rs.QueueResourceShare{
						GPU: rs.ResourceShare{
							MaxAllowed: 0,
						},
						CPU: rs.ResourceShare{
							MaxAllowed: 0,
						},
						Memory: rs.ResourceShare{
							MaxAllowed: 0,
						},
					},
				},
				"child": {
					Name:        "child",
					ParentQueue: "parent",
					QueueResourceShare: rs.QueueResourceShare{
						GPU: rs.ResourceShare{
							MaxAllowed: 0,
						},
						CPU: rs.ResourceShare{
							MaxAllowed: 0,
						},
						Memory: rs.ResourceShare{
							MaxAllowed: 0,
						},
					},
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
						ResReq: resource_info.NewResourceRequirements(15, 0, 0),
					},
				},
			},
			expectedError: true,
			errorContains: "parent queue 'parent' has reached the quota of GPU. Value is 5, workload requested 15",
		},
		{
			name: "job exceeds parent queue GPU quota (quota more restrictive than limit)",
			queues: map[common_info.QueueID]*rs.QueueAttributes{
				"parent": {
					Name: "parent",
					QueueResourceShare: rs.QueueResourceShare{
						GPU: rs.ResourceShare{
							MaxAllowed: 10,
						},
						CPU: rs.ResourceShare{
							MaxAllowed: 0,
						},
						Memory: rs.ResourceShare{
							MaxAllowed: 0,
						},
					},
				},
				"child": {
					Name:        "child",
					ParentQueue: "parent",
					QueueResourceShare: rs.QueueResourceShare{
						GPU: rs.ResourceShare{
							MaxAllowed: 0,
						},
						CPU: rs.ResourceShare{
							MaxAllowed: 0,
						},
						Memory: rs.ResourceShare{
							MaxAllowed: 0,
						},
					},
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
						ResReq: resource_info.NewResourceRequirements(15, 0, 0),
					},
				},
			},
			expectedError: true,
			errorContains: "parent queue 'parent' has reached the limit/quota of GPU. Value is 5, workload requested 15",
		},
		{
			name: "interactive preemptible job can exceed parent queue GPU limit",
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
				Priority:  constants.PriorityInteractivePreemptibleNumber, // Interactive preemptible job
				PodInfos: map[common_info.PodID]*pod_info.PodInfo{
					"pod-1": {
						Status: pod_status.Pending,
						ResReq: resource_info.NewResourceRequirements(15, 0, 0),
					},
				},
			},
			expectedError: false, // Interactive preemptible jobs can exceed limits
		},
		{
			name: "training job can exceed parent queue GPU limit",
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
				Priority:  constants.PriorityTrainNumber, // Training job
				PodInfos: map[common_info.PodID]*pod_info.PodInfo{
					"pod-1": {
						Status: pod_status.Pending,
						ResReq: resource_info.NewResourceRequirements(15, 0, 0),
					},
				},
			},
			expectedError: false, // Training jobs can exceed limits
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

			err := cp.checkParentQueueLimits(tt.job, ssn)

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
