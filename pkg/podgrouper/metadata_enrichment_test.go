// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package controllers

import (
	"testing"

	"github.com/go-logr/logr"
	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/NVIDIA/KAI-scheduler/pkg/podgrouper/podgroup"
	pluginconstants "github.com/NVIDIA/KAI-scheduler/pkg/podgrouper/podgrouper/plugins/constants"
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

func TestHandleSubgroupCreationRequest(t *testing.T) {
	tests := []struct {
		name                 string
		topOwnerKind         string
		topOwnerAnnotations  map[string]string
		topOwnerSpec         map[string]interface{}
		existingSubgroups    []string
		initialMinAvailable  int32
		expectedSubgroups    []string
		expectedMinAvailable int32
	}{
		{
			name:                 "nil topOwner does nothing",
			topOwnerAnnotations:  nil, // signals nil topOwner
			initialMinAvailable:  2,
			expectedMinAvailable: 2,
		},
		{
			name:                 "empty annotation does nothing",
			topOwnerKind:         "Job",
			topOwnerAnnotations:  map[string]string{},
			initialMinAvailable:  2,
			expectedMinAvailable: 2,
		},
		{
			name:                 "reserved 'default' name is ignored",
			topOwnerKind:         "Job",
			topOwnerAnnotations:  map[string]string{pluginconstants.CreateSubgroupAnnotationKey: "default"},
			initialMinAvailable:  2,
			expectedMinAvailable: 2,
		},
		{
			name:                 "creates default and requested subgroups when none exist",
			topOwnerKind:         "Job",
			topOwnerAnnotations:  map[string]string{pluginconstants.CreateSubgroupAnnotationKey: "auth-proxy"},
			initialMinAvailable:  2,
			expectedSubgroups:    []string{"default", "auth-proxy"},
			expectedMinAvailable: 3,
		},
		{
			name:                 "adds to existing subgroups without creating default",
			topOwnerKind:         "Job",
			topOwnerAnnotations:  map[string]string{pluginconstants.CreateSubgroupAnnotationKey: "auth-proxy"},
			existingSubgroups:    []string{"workers"},
			initialMinAvailable:  2,
			expectedSubgroups:    []string{"workers", "auth-proxy"},
			expectedMinAvailable: 3,
		},
		{
			name:                 "skips if subgroup already exists",
			topOwnerKind:         "Job",
			topOwnerAnnotations:  map[string]string{pluginconstants.CreateSubgroupAnnotationKey: "auth-proxy"},
			existingSubgroups:    []string{"auth-proxy"},
			initialMinAvailable:  2,
			expectedSubgroups:    []string{"auth-proxy"},
			expectedMinAvailable: 2,
		},
		{
			name:                 "skips for Deployment (multi-PodGroup workload)",
			topOwnerKind:         "Deployment",
			topOwnerAnnotations:  map[string]string{pluginconstants.CreateSubgroupAnnotationKey: "auth-proxy"},
			initialMinAvailable:  2,
			expectedMinAvailable: 2,
		},
		{
			name:                 "skips for JobSet with InOrder startup (default)",
			topOwnerKind:         "JobSet",
			topOwnerAnnotations:  map[string]string{pluginconstants.CreateSubgroupAnnotationKey: "auth-proxy"},
			initialMinAvailable:  2,
			expectedMinAvailable: 2,
		},
		{
			name:                 "skips for JobSet with explicit InOrder startup",
			topOwnerKind:         "JobSet",
			topOwnerAnnotations:  map[string]string{pluginconstants.CreateSubgroupAnnotationKey: "auth-proxy"},
			topOwnerSpec:         map[string]interface{}{"startupPolicy": map[string]interface{}{"startupPolicyOrder": "InOrder"}},
			initialMinAvailable:  2,
			expectedMinAvailable: 2,
		},
		{
			name:                 "allows JobSet with AnyOrder startup (single PodGroup)",
			topOwnerKind:         "JobSet",
			topOwnerAnnotations:  map[string]string{pluginconstants.CreateSubgroupAnnotationKey: "auth-proxy"},
			topOwnerSpec:         map[string]interface{}{"startupPolicy": map[string]interface{}{"startupPolicyOrder": "AnyOrder"}},
			initialMinAvailable:  2,
			expectedSubgroups:    []string{"default", "auth-proxy"},
			expectedMinAvailable: 3,
		},
	}

	logger := logr.Discard()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			metadata := &podgroup.Metadata{MinAvailable: tt.initialMinAvailable}
			for _, name := range tt.existingSubgroups {
				metadata.SubGroups = append(metadata.SubGroups, &podgroup.SubGroupMetadata{Name: name, MinAvailable: 1})
			}

			var topOwner *unstructured.Unstructured
			if tt.topOwnerAnnotations != nil {
				topOwner = &unstructured.Unstructured{Object: map[string]interface{}{
					"kind":     tt.topOwnerKind,
					"metadata": map[string]interface{}{"annotations": toInterfaceMap(tt.topOwnerAnnotations), "name": "test-workload", "namespace": "test-ns"},
				}}
				if tt.topOwnerSpec != nil {
					topOwner.Object["spec"] = tt.topOwnerSpec
				}
			}

			handleSubgroupCreationRequest(topOwner, metadata, logger)

			assert.Equal(t, tt.expectedMinAvailable, metadata.MinAvailable)
			assert.Equal(t, len(tt.expectedSubgroups), len(metadata.SubGroups))
			for i, name := range tt.expectedSubgroups {
				assert.Equal(t, name, metadata.SubGroups[i].Name)
			}
		})
	}
}

func TestHandlePodSubgroupAssignmentRequest(t *testing.T) {
	tests := []struct {
		name              string
		podAnnotations    map[string]string
		existingSubgroups []string
		expectAssignment  bool
	}{
		{
			name:             "nil annotations does nothing",
			podAnnotations:   nil,
			expectAssignment: false,
		},
		{
			name:             "empty annotation does nothing",
			podAnnotations:   map[string]string{},
			expectAssignment: false,
		},
		{
			name:              "assigns pod to matching subgroup",
			podAnnotations:    map[string]string{pluginconstants.RequestedSubgroupAnnotationKey: "auth-proxy"},
			existingSubgroups: []string{"default", "auth-proxy"},
			expectAssignment:  true,
		},
		{
			name:              "no assignment if subgroup doesn't exist",
			podAnnotations:    map[string]string{pluginconstants.RequestedSubgroupAnnotationKey: "auth-proxy"},
			existingSubgroups: []string{"default"},
			expectAssignment:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			metadata := &podgroup.Metadata{}
			for _, name := range tt.existingSubgroups {
				metadata.SubGroups = append(metadata.SubGroups, &podgroup.SubGroupMetadata{Name: name, MinAvailable: 1})
			}

			pod := &v1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "test-pod", Namespace: "test-ns", Annotations: tt.podAnnotations}}

			handlePodSubgroupAssignmentRequest(pod, metadata)

			if tt.expectAssignment {
				var found bool
				for _, sg := range metadata.SubGroups {
					if sg.Name == tt.podAnnotations[pluginconstants.RequestedSubgroupAnnotationKey] {
						for _, ref := range sg.PodsReferences {
							if ref.Name == pod.Name {
								found = true
							}
						}
					}
				}
				assert.True(t, found, "pod should be assigned to subgroup")
			}
		})
	}
}

func TestIsMultiPodGroupWorkload(t *testing.T) {
	tests := []struct {
		name     string
		kind     string
		spec     map[string]interface{}
		expected bool
	}{
		{"Job is not multi-PodGroup", "Job", nil, false},
		{"PyTorchJob is not multi-PodGroup", "PyTorchJob", nil, false},
		{"Deployment is multi-PodGroup", "Deployment", nil, true},
		{"JobSet with no startup policy (defaults to InOrder) is multi-PodGroup", "JobSet", nil, true},
		{"JobSet with InOrder is multi-PodGroup", "JobSet", map[string]interface{}{"startupPolicy": map[string]interface{}{"startupPolicyOrder": "InOrder"}}, true},
		{"JobSet with AnyOrder is not multi-PodGroup", "JobSet", map[string]interface{}{"startupPolicy": map[string]interface{}{"startupPolicyOrder": "AnyOrder"}}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			topOwner := &unstructured.Unstructured{Object: map[string]interface{}{"kind": tt.kind}}
			if tt.spec != nil {
				topOwner.Object["spec"] = tt.spec
			}
			assert.Equal(t, tt.expected, isMultiPodGroupWorkload(topOwner))
		})
	}
}

func toInterfaceMap(m map[string]string) map[string]interface{} {
	result := make(map[string]interface{})
	for k, v := range m {
		result[k] = v
	}
	return result
}
