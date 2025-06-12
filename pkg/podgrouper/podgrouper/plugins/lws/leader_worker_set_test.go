// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package lws

import (
	"testing"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/NVIDIA/KAI-scheduler/pkg/podgrouper/podgroup"
	"github.com/NVIDIA/KAI-scheduler/pkg/podgrouper/podgrouper/plugins/defaultgrouper"
)

const (
	queueLabelKey    = "kai.scheduler/queue"
	nodePoolLabelKey = "kai.scheduler/node-pool"
)

func TestGetGroupIndexFromPod(t *testing.T) {
	tests := []struct {
		name          string
		pod           *v1.Pod
		expectedIndex int
		expectedError bool
	}{
		{
			name: "Valid group index",
			pod: &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pod",
					Namespace: "test-ns",
					Labels: map[string]string{
						lwsGroupIndexLabel: "1",
					},
				},
			},
			expectedIndex: 1,
			expectedError: false,
		},
		{
			name: "Group index zero",
			pod: &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pod",
					Namespace: "test-ns",
					Labels: map[string]string{
						lwsGroupIndexLabel: "0",
					},
				},
			},
			expectedIndex: 0,
			expectedError: false,
		},
		{
			name: "Missing group index label",
			pod: &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pod",
					Namespace: "test-ns",
					Labels:    nil,
				},
			},
			expectedIndex: 0,
			expectedError: true,
		},
		{
			name: "Invalid group index format",
			pod: &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pod",
					Namespace: "test-ns",
					Labels: map[string]string{
						lwsGroupIndexLabel: "invalid",
					},
				},
			},
			expectedIndex: 0,
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defaultGrouper := defaultgrouper.NewDefaultGrouper(queueLabelKey, nodePoolLabelKey)
			lg := NewLwsGrouper(defaultGrouper)

			index, err := lg.getGroupIndexFromPod(tt.pod)
			if (err != nil) != tt.expectedError {
				t.Errorf("getGroupIndexFromPod() error = %v, expectedError %v", err, tt.expectedError)
				return
			}
			if !tt.expectedError && index != tt.expectedIndex {
				t.Errorf("getGroupIndexFromPod() = %v, want %v", index, tt.expectedIndex)
			}
		})
	}
}

func TestGetLwsGroupSize(t *testing.T) {
	tests := []struct {
		name          string
		lwsJob        *unstructured.Unstructured
		expectedSize  int32
		expectedError bool
	}{
		{
			name: "Valid group size",
			lwsJob: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"metadata": map[string]interface{}{
						"name":      "test-lws",
						"namespace": "test-ns",
					},
					"spec": map[string]interface{}{
						"leaderWorkerTemplate": map[string]interface{}{
							"size": int64(3),
						},
					},
				},
			},
			expectedSize:  3,
			expectedError: false,
		},
		{
			name: "Missing leaderWorkerTemplate",
			lwsJob: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"metadata": map[string]interface{}{
						"name":      "test-lws",
						"namespace": "test-ns",
					},
					"spec": map[string]interface{}{},
				},
			},
			expectedSize:  0,
			expectedError: true,
		},
		{
			name: "Missing size field",
			lwsJob: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"metadata": map[string]interface{}{
						"name":      "test-lws",
						"namespace": "test-ns",
					},
					"spec": map[string]interface{}{
						"leaderWorkerTemplate": map[string]interface{}{},
					},
				},
			},
			expectedSize:  0,
			expectedError: true,
		},
		{
			name: "Invalid size (zero)",
			lwsJob: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"metadata": map[string]interface{}{
						"name":      "test-lws",
						"namespace": "test-ns",
					},
					"spec": map[string]interface{}{
						"leaderWorkerTemplate": map[string]interface{}{
							"size": int64(0),
						},
					},
				},
			},
			expectedSize:  0,
			expectedError: true,
		},
		{
			name: "Invalid size (negative)",
			lwsJob: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"metadata": map[string]interface{}{
						"name":      "test-lws",
						"namespace": "test-ns",
					},
					"spec": map[string]interface{}{
						"leaderWorkerTemplate": map[string]interface{}{
							"size": int64(-1),
						},
					},
				},
			},
			expectedSize:  0,
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defaultGrouper := defaultgrouper.NewDefaultGrouper(queueLabelKey, nodePoolLabelKey)
			lg := NewLwsGrouper(defaultGrouper)

			size, err := lg.getLwsGroupSize(tt.lwsJob)
			if (err != nil) != tt.expectedError {
				t.Errorf("getLwsGroupSize() error = %v, expectedError %v", err, tt.expectedError)
				return
			}
			if !tt.expectedError && size != tt.expectedSize {
				t.Errorf("getLwsGroupSize() = %v, want %v", size, tt.expectedSize)
			}
		})
	}
}

func TestIsPodLeader(t *testing.T) {
	tests := []struct {
		name     string
		pod      *v1.Pod
		isLeader bool
	}{
		{
			name: "Pod without worker index label (leader)",
			pod: &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "leader-pod",
					Namespace: "test-ns",
					Labels: map[string]string{
						lwsNameLabel:       "test-lws",
						lwsGroupIndexLabel: "0",
					},
				},
			},
			isLeader: true,
		},
		{
			name: "Pod with worker index 0 (leader)",
			pod: &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "leader-pod",
					Namespace: "test-ns",
					Labels: map[string]string{
						lwsNameLabel:        "test-lws",
						lwsGroupIndexLabel:  "0",
						lwsWorkerIndexLabel: "0",
					},
				},
			},
			isLeader: true,
		},
		{
			name: "Pod with worker index 1 (worker)",
			pod: &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "worker-pod",
					Namespace: "test-ns",
					Labels: map[string]string{
						lwsNameLabel:        "test-lws",
						lwsGroupIndexLabel:  "0",
						lwsWorkerIndexLabel: "1",
					},
				},
			},
			isLeader: false,
		},
		{
			name: "Pod with invalid worker index (not leader)",
			pod: &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "worker-pod",
					Namespace: "test-ns",
					Labels: map[string]string{
						lwsNameLabel:        "test-lws",
						lwsGroupIndexLabel:  "0",
						lwsWorkerIndexLabel: "invalid",
					},
				},
			},
			isLeader: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defaultGrouper := defaultgrouper.NewDefaultGrouper(queueLabelKey, nodePoolLabelKey)
			lg := NewLwsGrouper(defaultGrouper)

			result := lg.isPodLeader(tt.pod)
			if result != tt.isLeader {
				t.Errorf("isPodLeader() = %v, want %v", result, tt.isLeader)
			}
		})
	}
}

func TestIsPodReady(t *testing.T) {
	tests := []struct {
		name    string
		pod     *v1.Pod
		isReady bool
	}{
		{
			name: "Pod is ready",
			pod: &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "ready-pod",
					Namespace: "test-ns",
				},
				Status: v1.PodStatus{
					Conditions: []v1.PodCondition{
						{
							Type:   v1.PodReady,
							Status: v1.ConditionTrue,
						},
					},
				},
			},
			isReady: true,
		},
		{
			name: "Pod is not ready",
			pod: &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "not-ready-pod",
					Namespace: "test-ns",
				},
				Status: v1.PodStatus{
					Conditions: []v1.PodCondition{
						{
							Type:   v1.PodReady,
							Status: v1.ConditionFalse,
						},
					},
				},
			},
			isReady: false,
		},
		{
			name: "Pod without ready condition",
			pod: &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "no-condition-pod",
					Namespace: "test-ns",
				},
				Status: v1.PodStatus{
					Conditions: []v1.PodCondition{},
				},
			},
			isReady: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defaultGrouper := defaultgrouper.NewDefaultGrouper(queueLabelKey, nodePoolLabelKey)
			lg := NewLwsGrouper(defaultGrouper)

			result := lg.isPodReady(tt.pod)
			if result != tt.isReady {
				t.Errorf("isPodReady() = %v, want %v", result, tt.isReady)
			}
		})
	}
}

func TestHandleLeaderReadyStartupPolicy(t *testing.T) {
	tests := []struct {
		name             string
		pod              *v1.Pod
		initialMinAvail  int32
		expectedMinAvail int32
		expectedError    bool
	}{
		{
			name: "Worker pod (no change)",
			pod: &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "worker-pod",
					Namespace: "test-ns",
					Labels: map[string]string{
						lwsWorkerIndexLabel: "1",
					},
				},
			},
			initialMinAvail:  5,
			expectedMinAvail: 5,
			expectedError:    false,
		},
		{
			name: "Ready leader pod (no change)",
			pod: &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "leader-pod",
					Namespace: "test-ns",
					Labels: map[string]string{
						lwsWorkerIndexLabel: "0",
					},
				},
				Status: v1.PodStatus{
					Conditions: []v1.PodCondition{
						{
							Type:   v1.PodReady,
							Status: v1.ConditionTrue,
						},
					},
				},
			},
			initialMinAvail:  5,
			expectedMinAvail: 5,
			expectedError:    false,
		},
		{
			name: "Not ready leader pod (reduce to 1)",
			pod: &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "leader-pod",
					Namespace: "test-ns",
					Labels: map[string]string{
						lwsWorkerIndexLabel: "0",
					},
				},
				Status: v1.PodStatus{
					Conditions: []v1.PodCondition{
						{
							Type:   v1.PodReady,
							Status: v1.ConditionFalse,
						},
					},
				},
			},
			initialMinAvail:  5,
			expectedMinAvail: 1,
			expectedError:    false,
		},
		{
			name: "Terminating ready leader pod (reduce to 1)",
			pod: &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:              "leader-pod",
					Namespace:         "test-ns",
					DeletionTimestamp: &metav1.Time{},
					Labels: map[string]string{
						lwsWorkerIndexLabel: "0",
					},
				},
				Status: v1.PodStatus{
					Conditions: []v1.PodCondition{
						{
							Type:   v1.PodReady,
							Status: v1.ConditionTrue,
						},
					},
				},
			},
			initialMinAvail:  5,
			expectedMinAvail: 1,
			expectedError:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defaultGrouper := defaultgrouper.NewDefaultGrouper(queueLabelKey, nodePoolLabelKey)
			lg := NewLwsGrouper(defaultGrouper)

			podGroupMetadata := &podgroup.Metadata{
				MinAvailable: tt.initialMinAvail,
			}

			err := lg.handleLeaderReadyStartupPolicy(tt.pod, podGroupMetadata)
			if (err != nil) != tt.expectedError {
				t.Errorf("handleLeaderReadyStartupPolicy() error = %v, expectedError %v", err, tt.expectedError)
				return
			}

			if !tt.expectedError && podGroupMetadata.MinAvailable != tt.expectedMinAvail {
				t.Errorf("handleLeaderReadyStartupPolicy() MinAvailable = %v, want %v", podGroupMetadata.MinAvailable, tt.expectedMinAvail)
			}
		})
	}
}

func TestGetPodGroupMetadata(t *testing.T) {
	tests := []struct {
		name               string
		lwsJob             *unstructured.Unstructured
		pod                *v1.Pod
		expectedMinAvail   int32
		expectedNameSuffix string
		expectedError      bool
	}{
		{
			name: "Basic LWS group metadata",
			lwsJob: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"metadata": map[string]interface{}{
						"name":      "test-lws",
						"namespace": "test-ns",
						"uid":       "uid1",
						"labels": map[string]interface{}{
							queueLabelKey: "test-queue",
						},
					},
					"spec": map[string]interface{}{
						"leaderWorkerTemplate": map[string]interface{}{
							"size": int64(3),
						},
					},
				},
			},
			pod: &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pod",
					Namespace: "test-ns",
					Labels: map[string]string{
						lwsGroupIndexLabel: "1",
						queueLabelKey:      "test-queue",
					},
				},
			},
			expectedMinAvail:   3,
			expectedNameSuffix: "-group-1",
			expectedError:      false,
		},
		{
			name: "LWS with LeaderReady startup policy - worker pod",
			lwsJob: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"metadata": map[string]interface{}{
						"name":      "test-lws",
						"namespace": "test-ns",
						"uid":       "uid1",
						"labels": map[string]interface{}{
							queueLabelKey: "test-queue",
						},
					},
					"spec": map[string]interface{}{
						"startupPolicy": leaderReadyStartupPolicy,
						"leaderWorkerTemplate": map[string]interface{}{
							"size": int64(3),
						},
					},
				},
			},
			pod: &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "worker-pod",
					Namespace: "test-ns",
					Labels: map[string]string{
						lwsGroupIndexLabel:  "0",
						lwsWorkerIndexLabel: "1",
						queueLabelKey:       "test-queue",
					},
				},
			},
			expectedMinAvail:   3,
			expectedNameSuffix: "-group-0",
			expectedError:      false,
		},
		{
			name: "LWS with LeaderReady startup policy - not ready leader",
			lwsJob: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"metadata": map[string]interface{}{
						"name":      "test-lws",
						"namespace": "test-ns",
						"uid":       "uid1",
						"labels": map[string]interface{}{
							queueLabelKey: "test-queue",
						},
					},
					"spec": map[string]interface{}{
						"startupPolicy": leaderReadyStartupPolicy,
						"leaderWorkerTemplate": map[string]interface{}{
							"size": int64(3),
						},
					},
				},
			},
			pod: &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "leader-pod",
					Namespace: "test-ns",
					Labels: map[string]string{
						lwsGroupIndexLabel:  "0",
						lwsWorkerIndexLabel: "0",
						queueLabelKey:       "test-queue",
					},
				},
				Status: v1.PodStatus{
					Conditions: []v1.PodCondition{
						{
							Type:   v1.PodReady,
							Status: v1.ConditionFalse,
						},
					},
				},
			},
			expectedMinAvail:   1,
			expectedNameSuffix: "-group-0",
			expectedError:      false,
		},
		{
			name: "Invalid group index",
			lwsJob: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"metadata": map[string]interface{}{
						"name":      "test-lws",
						"namespace": "test-ns",
						"uid":       "uid1",
						"labels": map[string]interface{}{
							queueLabelKey: "test-queue",
						},
					},
					"spec": map[string]interface{}{
						"leaderWorkerTemplate": map[string]interface{}{
							"size": int64(3),
						},
					},
				},
			},
			pod: &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pod",
					Namespace: "test-ns",
					Labels: map[string]string{
						lwsGroupIndexLabel: "invalid",
						queueLabelKey:      "test-queue",
					},
				},
			},
			expectedMinAvail:   0,
			expectedNameSuffix: "",
			expectedError:      true,
		},
		{
			name: "Invalid group size",
			lwsJob: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"metadata": map[string]interface{}{
						"name":      "test-lws",
						"namespace": "test-ns",
						"uid":       "uid1",
						"labels": map[string]interface{}{
							queueLabelKey: "test-queue",
						},
					},
					"spec": map[string]interface{}{
						"leaderWorkerTemplate": map[string]interface{}{
							"size": int64(0),
						},
					},
				},
			},
			pod: &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pod",
					Namespace: "test-ns",
					Labels: map[string]string{
						lwsGroupIndexLabel: "0",
						queueLabelKey:      "test-queue",
					},
				},
			},
			expectedMinAvail:   0,
			expectedNameSuffix: "",
			expectedError:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defaultGrouper := defaultgrouper.NewDefaultGrouper(queueLabelKey, nodePoolLabelKey)
			lg := NewLwsGrouper(defaultGrouper)

			metadata, err := lg.GetPodGroupMetadata(tt.lwsJob, tt.pod)
			if (err != nil) != tt.expectedError {
				t.Errorf("GetPodGroupMetadata() error = %v, expectedError %v", err, tt.expectedError)
				return
			}

			if tt.expectedError {
				return
			}

			if metadata.MinAvailable != tt.expectedMinAvail {
				t.Errorf("GetPodGroupMetadata() MinAvailable = %v, want %v", metadata.MinAvailable, tt.expectedMinAvail)
			}

			if tt.expectedNameSuffix != "" {
				expectedName := "pg-test-lws-uid1" + tt.expectedNameSuffix
				if metadata.Name != expectedName {
					t.Errorf("GetPodGroupMetadata() Name = %v, want %v", metadata.Name, expectedName)
				}
			}
		})
	}
}

func TestName(t *testing.T) {
	defaultGrouper := defaultgrouper.NewDefaultGrouper(queueLabelKey, nodePoolLabelKey)
	lg := NewLwsGrouper(defaultGrouper)

	expectedName := "LwsGrouper"
	if lg.Name() != expectedName {
		t.Errorf("Name() = %v, want %v", lg.Name(), expectedName)
	}
}
