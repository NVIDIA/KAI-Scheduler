// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package podgroup

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/types"
)

func TestFindSubGroupForPod(t *testing.T) {
	tests := []struct {
		name             string
		metadata         Metadata
		podNamespace     string
		podName          string
		expectedSubGroup *SubGroupMetadata
	}{
		{
			name: "pod found in first subgroup",
			metadata: Metadata{
				SubGroups: []*SubGroupMetadata{
					{
						Name: "subgroup-1",
						PodsReferences: []*types.NamespacedName{
							{Namespace: "ns1", Name: "pod-a"},
							{Namespace: "ns1", Name: "pod-b"},
						},
					},
					{
						Name: "subgroup-2",
						PodsReferences: []*types.NamespacedName{
							{Namespace: "ns1", Name: "pod-c"},
						},
					},
				},
			},
			podNamespace:     "ns1",
			podName:          "pod-a",
			expectedSubGroup: &SubGroupMetadata{Name: "subgroup-1"},
		},
		{
			name: "pod found in second subgroup",
			metadata: Metadata{
				SubGroups: []*SubGroupMetadata{
					{
						Name: "subgroup-1",
						PodsReferences: []*types.NamespacedName{
							{Namespace: "ns1", Name: "pod-a"},
						},
					},
					{
						Name: "subgroup-2",
						PodsReferences: []*types.NamespacedName{
							{Namespace: "ns1", Name: "pod-c"},
						},
					},
				},
			},
			podNamespace:     "ns1",
			podName:          "pod-c",
			expectedSubGroup: &SubGroupMetadata{Name: "subgroup-2"},
		},
		{
			name: "pod not found - different namespace",
			metadata: Metadata{
				SubGroups: []*SubGroupMetadata{
					{
						Name: "subgroup-1",
						PodsReferences: []*types.NamespacedName{
							{Namespace: "ns1", Name: "pod-a"},
						},
					},
				},
			},
			podNamespace:     "ns2",
			podName:          "pod-a",
			expectedSubGroup: nil,
		},
		{
			name: "pod not found - different name",
			metadata: Metadata{
				SubGroups: []*SubGroupMetadata{
					{
						Name: "subgroup-1",
						PodsReferences: []*types.NamespacedName{
							{Namespace: "ns1", Name: "pod-a"},
						},
					},
				},
			},
			podNamespace:     "ns1",
			podName:          "pod-b",
			expectedSubGroup: nil,
		},
		{
			name: "empty subgroups",
			metadata: Metadata{
				SubGroups: []*SubGroupMetadata{},
			},
			podNamespace:     "ns1",
			podName:          "pod-a",
			expectedSubGroup: nil,
		},
		{
			name: "nil subgroups",
			metadata: Metadata{
				SubGroups: nil,
			},
			podNamespace:     "ns1",
			podName:          "pod-a",
			expectedSubGroup: nil,
		},
		{
			name: "subgroup with empty pod references",
			metadata: Metadata{
				SubGroups: []*SubGroupMetadata{
					{
						Name:           "subgroup-1",
						PodsReferences: []*types.NamespacedName{},
					},
				},
			},
			podNamespace:     "ns1",
			podName:          "pod-a",
			expectedSubGroup: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.metadata.FindSubGroupForPod(tt.podNamespace, tt.podName)
			if tt.expectedSubGroup == nil {
				assert.Nil(t, result)
			} else {
				assert.NotNil(t, result)
				assert.Equal(t, tt.expectedSubGroup.Name, result.Name)
			}
		})
	}
}
