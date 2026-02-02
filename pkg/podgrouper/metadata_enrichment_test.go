// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package controllers

import (
	"testing"

	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/NVIDIA/KAI-scheduler/pkg/podgrouper/podgroup"
)

const nodePoolKey = "kai.scheduler/node-pool"

func TestAddNodePoolLabel(t *testing.T) {
	metadata := podgroup.Metadata{
		Annotations:       nil,
		Labels:            nil,
		PriorityClassName: "",
		Queue:             "",
		Namespace:         "",
		Name:              "",
		MinAvailable:      0,
		Owner:             metav1.OwnerReference{},
	}

	pod := v1.Pod{
		TypeMeta: metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{
			Labels: map[string]string{
				nodePoolKey: "my-node-pool",
			},
		},
		Spec:   v1.PodSpec{},
		Status: v1.PodStatus{},
	}

	addNodePoolLabel(&metadata, &pod, nodePoolKey)
	assert.Equal(t, "my-node-pool", metadata.Labels[nodePoolKey])

	metadata.Labels = nil
	pod.Labels = nil

	addNodePoolLabel(&metadata, &pod, nodePoolKey)
	assert.Equal(t, "", metadata.Labels[nodePoolKey])

	metadata.Labels = map[string]string{
		nodePoolKey: "non-default-pool",
	}

	addNodePoolLabel(&metadata, &pod, nodePoolKey)
	assert.Equal(t, "non-default-pool", metadata.Labels[nodePoolKey])
}

func TestGetCreateSubgroupAnnotation(t *testing.T) {
	tests := []struct {
		name          string
		annotations   map[string]interface{}
		expectedName  string
		expectedFound bool
	}{
		{"valid annotation", map[string]interface{}{"kai.alpha.scheduler/create-subgroup": "auth-proxy"}, "auth-proxy", true},
		{"empty annotation", map[string]interface{}{"kai.alpha.scheduler/create-subgroup": ""}, "", false},
		{"reserved default name", map[string]interface{}{"kai.alpha.scheduler/create-subgroup": "default"}, "", false},
		{"missing annotation", map[string]interface{}{}, "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			topOwner := &unstructured.Unstructured{
				Object: map[string]interface{}{"metadata": map[string]interface{}{"annotations": tt.annotations}},
			}
			name, found := getCreateSubgroupAnnotation(topOwner)
			assert.Equal(t, tt.expectedFound, found)
			assert.Equal(t, tt.expectedName, name)
		})
	}
}

func TestEnsureRequestedSubgroupExists(t *testing.T) {
	tests := []struct {
		name                 string
		existingSubgroups    []string
		initialMinAvailable  int32
		requestedSubgroup    string
		expectedSubgroups    []string
		expectedMinAvailable int32
	}{
		{"creates default and requested when empty", nil, 4, "auth-proxy", []string{"default", "auth-proxy"}, 5},
		{"adds to existing subgroups", []string{"workers"}, 3, "auth-proxy", []string{"workers", "auth-proxy"}, 4},
		{"skips if already exists", []string{"auth-proxy"}, 2, "auth-proxy", []string{"auth-proxy"}, 2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			metadata := &podgroup.Metadata{MinAvailable: tt.initialMinAvailable}
			for _, name := range tt.existingSubgroups {
				metadata.SubGroups = append(metadata.SubGroups, &podgroup.SubGroupMetadata{Name: name, MinAvailable: 1})
			}

			ensureRequestedSubgroupExists(metadata, tt.requestedSubgroup)

			assert.Equal(t, len(tt.expectedSubgroups), len(metadata.SubGroups))
			assert.Equal(t, tt.expectedMinAvailable, metadata.MinAvailable)
			for i, name := range tt.expectedSubgroups {
				assert.Equal(t, name, metadata.SubGroups[i].Name)
			}
		})
	}
}

func TestHandleRequestedSubgroups(t *testing.T) {
	tests := []struct {
		name                  string
		existingSubgroups     []string
		initialMinAvailable   int32
		topOwnerAnnotation    string // empty means no annotation
		podAnnotation         string // empty means no annotation
		expectedSubgroups     []string
		expectedMinAvailable  int32
		expectedPodInSubgroup string
	}{
		{
			name:                 "creates subgroups from topOwner annotation",
			initialMinAvailable:  4,
			topOwnerAnnotation:   "auth-proxy",
			expectedSubgroups:    []string{"default", "auth-proxy"},
			expectedMinAvailable: 5,
		},
		{
			name:                  "pod joins created subgroup",
			initialMinAvailable:   4,
			topOwnerAnnotation:    "auth-proxy",
			podAnnotation:         "auth-proxy",
			expectedSubgroups:     []string{"default", "auth-proxy"},
			expectedMinAvailable:  5,
			expectedPodInSubgroup: "auth-proxy",
		},
		{
			name:                  "pod joins existing subgroup without topOwner annotation",
			existingSubgroups:     []string{"workers", "auth-proxy"},
			initialMinAvailable:   4,
			podAnnotation:         "auth-proxy",
			expectedSubgroups:     []string{"workers", "auth-proxy"},
			expectedMinAvailable:  4,
			expectedPodInSubgroup: "auth-proxy",
		},
		{
			name:                 "no changes without annotations",
			initialMinAvailable:  4,
			expectedMinAvailable: 4,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			metadata := &podgroup.Metadata{MinAvailable: tt.initialMinAvailable}
			for _, name := range tt.existingSubgroups {
				metadata.SubGroups = append(metadata.SubGroups, &podgroup.SubGroupMetadata{Name: name, MinAvailable: 1})
			}

			pod := &v1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "test-pod", Namespace: "test-ns"}}
			if tt.podAnnotation != "" {
				pod.Annotations = map[string]string{"kai.alpha.scheduler/requested-subgroup": tt.podAnnotation}
			}

			var topOwner *unstructured.Unstructured
			if tt.topOwnerAnnotation != "" {
				topOwner = &unstructured.Unstructured{
					Object: map[string]interface{}{
						"metadata": map[string]interface{}{
							"annotations": map[string]interface{}{
								"kai.alpha.scheduler/create-subgroup": tt.topOwnerAnnotation,
							},
						},
					},
				}
			}

			handleRequestedSubgroups(metadata, pod, topOwner)

			assert.Equal(t, len(tt.expectedSubgroups), len(metadata.SubGroups))
			assert.Equal(t, tt.expectedMinAvailable, metadata.MinAvailable)

			if tt.expectedPodInSubgroup != "" {
				var found bool
				for _, sg := range metadata.SubGroups {
					if sg.Name == tt.expectedPodInSubgroup {
						for _, ref := range sg.PodsReferences {
							if ref.Name == pod.Name {
								found = true
							}
						}
					}
				}
				assert.True(t, found, "pod should be in subgroup %s", tt.expectedPodInSubgroup)
			}
		})
	}
}
