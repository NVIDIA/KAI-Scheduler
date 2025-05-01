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
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/framework"
)

func TestParentQueueQuotaChecking(t *testing.T) {
	tests := []struct {
		name          string
		queues        map[common_info.QueueID]*queue_info.QueueInfo
		job           *podgroup_info.PodGroupInfo
		expectedError bool
		errorContains string
	}{
		{
			name: "job within all quota limits",
			queues: map[common_info.QueueID]*queue_info.QueueInfo{
				"parent-queue": {
					UID:  "parent-queue",
					Name: "parent-queue",
					Resources: queue_info.QueueQuota{
						GPU: queue_info.ResourceQuota{
							Quota: 10,
						},
						CPU: queue_info.ResourceQuota{
							Quota: 100,
						},
						Memory: queue_info.ResourceQuota{
							Quota: 1024,
						},
					},
				},
				"child-queue": {
					UID:         "child-queue",
					Name:        "child-queue",
					ParentQueue: "parent-queue",
				},
			},
			job: &podgroup_info.PodGroupInfo{
				Queue: "child-queue",
				PodInfos: map[common_info.PodID]*pod_info.PodInfo{
					"pod1": {
						Status: pod_status.Pending,
						ResReq: resource_info.NewResourceRequirements(5, 50, 512),
					},
				},
			},
			expectedError: false,
		},
		{
			name: "job exceeds parent queue GPU quota",
			queues: map[common_info.QueueID]*queue_info.QueueInfo{
				"parent-queue": {
					UID:  "parent-queue",
					Name: "parent-queue",
					Resources: queue_info.QueueQuota{
						GPU: queue_info.ResourceQuota{
							Quota: 10,
						},
					},
				},
				"child-queue": {
					UID:         "child-queue",
					Name:        "child-queue",
					ParentQueue: "parent-queue",
				},
			},
			job: &podgroup_info.PodGroupInfo{
				Queue: "child-queue",
				PodInfos: map[common_info.PodID]*pod_info.PodInfo{
					"pod1": {
						Status: pod_status.Pending,
						ResReq: resource_info.NewResourceRequirements(15, 0, 0),
					},
				},
			},
			expectedError: true,
			errorContains: "quota has reached the allowable limit of GPUs",
		},
		{
			name: "job exceeds parent queue CPU quota",
			queues: map[common_info.QueueID]*queue_info.QueueInfo{
				"parent-queue": {
					UID:  "parent-queue",
					Name: "parent-queue",
					Resources: queue_info.QueueQuota{
						CPU: queue_info.ResourceQuota{
							Quota: 100,
						},
					},
				},
				"child-queue": {
					UID:         "child-queue",
					Name:        "child-queue",
					ParentQueue: "parent-queue",
				},
			},
			job: &podgroup_info.PodGroupInfo{
				Queue: "child-queue",
				PodInfos: map[common_info.PodID]*pod_info.PodInfo{
					"pod1": {
						Status: pod_status.Pending,
						ResReq: resource_info.NewResourceRequirements(0, 150, 0),
					},
				},
			},
			expectedError: true,
			errorContains: "quota has reached the allowable limit of CPU",
		},
		{
			name: "job exceeds parent queue Memory quota",
			queues: map[common_info.QueueID]*queue_info.QueueInfo{
				"parent-queue": {
					UID:  "parent-queue",
					Name: "parent-queue",
					Resources: queue_info.QueueQuota{
						Memory: queue_info.ResourceQuota{
							Quota: 1024,
						},
					},
				},
				"child-queue": {
					UID:         "child-queue",
					Name:        "child-queue",
					ParentQueue: "parent-queue",
				},
			},
			job: &podgroup_info.PodGroupInfo{
				Queue: "child-queue",
				PodInfos: map[common_info.PodID]*pod_info.PodInfo{
					"pod1": {
						Status: pod_status.Pending,
						ResReq: resource_info.NewResourceRequirements(0, 0, 2048),
					},
				},
			},
			expectedError: true,
			errorContains: "quota has reached the allowable limit of Memory",
		},
		{
			name: "multi-level queue hierarchy check with all resources",
			queues: map[common_info.QueueID]*queue_info.QueueInfo{
				"top-queue": {
					UID:  "top-queue",
					Name: "top-queue",
					Resources: queue_info.QueueQuota{
						GPU: queue_info.ResourceQuota{
							Quota: 20,
						},
						CPU: queue_info.ResourceQuota{
							Quota: 200,
						},
						Memory: queue_info.ResourceQuota{
							Quota: 2048,
						},
					},
				},
				"mid-queue": {
					UID:         "mid-queue",
					Name:        "mid-queue",
					ParentQueue: "top-queue",
					Resources: queue_info.QueueQuota{
						GPU: queue_info.ResourceQuota{
							Quota: 10,
						},
						CPU: queue_info.ResourceQuota{
							Quota: 100,
						},
						Memory: queue_info.ResourceQuota{
							Quota: 1024,
						},
					},
				},
				"leaf-queue": {
					UID:         "leaf-queue",
					Name:        "leaf-queue",
					ParentQueue: "mid-queue",
				},
			},
			job: &podgroup_info.PodGroupInfo{
				Queue: "leaf-queue",
				PodInfos: map[common_info.PodID]*pod_info.PodInfo{
					"pod1": {
						Status: pod_status.Pending,
						ResReq: resource_info.NewResourceRequirements(15, 150, 1500),
					},
				},
			},
			expectedError: true,
			errorContains: "quota has reached the allowable limit",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create session with test queues
			ssn := &framework.Session{
				Queues: tt.queues,
			}

			// Create capacity policy
			cp := &CapacityPolicy{}

			// Run the quota check
			err := cp.checkParentQueueQuotas(tt.job, ssn)

			if tt.expectedError {
				if assert.Error(t, err) && tt.errorContains != "" {
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
