// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package skiptopowner

import (
	"testing"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/NVIDIA/KAI-scheduler/pkg/podgrouper/podgrouper/plugins/constants"
	"github.com/NVIDIA/KAI-scheduler/pkg/podgrouper/podgrouper/plugins/defaultgrouper"
	grouperplugin "github.com/NVIDIA/KAI-scheduler/pkg/podgrouper/podgrouper/plugins/grouper"
)

func TestPropagateMetadataDownChain(t *testing.T) {
	tests := []struct {
		name                string
		skippedOwner        *unstructured.Unstructured
		lastOwner           *unstructured.Unstructured
		expectedLabels      map[string]string
		expectedAnnotations map[string]string
		description         string
	}{
		{
			name: "propagate labels from skippedOwner to lastOwner",
			skippedOwner: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"metadata": map[string]interface{}{
						"name":      "top-owner",
						"namespace": "test",
						"labels": map[string]interface{}{
							constants.PriorityLabelKey: "high-priority",
						},
					},
				},
			},
			lastOwner: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"metadata": map[string]interface{}{
						"name":      "last-owner",
						"namespace": "test",
						"labels":    map[string]interface{}{},
					},
				},
			},
			expectedLabels: map[string]string{
				constants.PriorityLabelKey: "high-priority",
			},
			expectedAnnotations: nil,
			description:         "labels should be propagated from skippedOwner to lastOwner",
		},
		{
			name: "propagate annotations from skippedOwner to lastOwner",
			skippedOwner: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"metadata": map[string]interface{}{
						"name":      "top-owner",
						"namespace": "test",
						"annotations": map[string]interface{}{
							"test-annotation": "test-value",
						},
					},
				},
			},
			lastOwner: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"metadata": map[string]interface{}{
						"name":        "last-owner",
						"namespace":   "test",
						"annotations": map[string]interface{}{},
					},
				},
			},
			expectedLabels: nil,
			expectedAnnotations: map[string]string{
				"test-annotation": "test-value",
			},
			description: "annotations should be propagated from skippedOwner to lastOwner",
		},
		{
			name: "do not override existing labels",
			skippedOwner: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"metadata": map[string]interface{}{
						"name":      "top-owner",
						"namespace": "test",
						"labels": map[string]interface{}{
							constants.PriorityLabelKey: "high-priority",
						},
					},
				},
			},
			lastOwner: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"metadata": map[string]interface{}{
						"name":      "last-owner",
						"namespace": "test",
						"labels": map[string]interface{}{
							constants.PriorityLabelKey: "low-priority",
						},
					},
				},
			},
			expectedLabels: map[string]string{
				constants.PriorityLabelKey: "low-priority",
			},
			expectedAnnotations: nil,
			description:         "existing labels on lastOwner should not be overridden",
		},
		{
			name: "do not override existing annotations",
			skippedOwner: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"metadata": map[string]interface{}{
						"name":      "top-owner",
						"namespace": "test",
						"annotations": map[string]interface{}{
							"test-annotation": "skipped-value",
						},
					},
				},
			},
			lastOwner: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"metadata": map[string]interface{}{
						"name":      "last-owner",
						"namespace": "test",
						"annotations": map[string]interface{}{
							"test-annotation": "existing-value",
						},
					},
				},
			},
			expectedLabels: nil,
			expectedAnnotations: map[string]string{
				"test-annotation": "existing-value",
			},
			description: "existing annotations on lastOwner should not be overridden",
		},
		{
			name: "no labels or annotations in skippedOwner",
			skippedOwner: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"metadata": map[string]interface{}{
						"name":      "top-owner",
						"namespace": "test",
					},
				},
			},
			lastOwner: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"metadata": map[string]interface{}{
						"name":      "last-owner",
						"namespace": "test",
					},
				},
			},
			expectedLabels:      nil,
			expectedAnnotations: nil,
			description:         "no propagation should occur if skippedOwner has no labels or annotations",
		},
		{
			name: "propagate both labels and annotations",
			skippedOwner: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"metadata": map[string]interface{}{
						"name":      "top-owner",
						"namespace": "test",
						"labels": map[string]interface{}{
							constants.PriorityLabelKey: "high-priority",
							"extra-label":              "extra-value",
						},
						"annotations": map[string]interface{}{
							"annotation-1": "value-1",
						},
					},
				},
			},
			lastOwner: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"metadata": map[string]interface{}{
						"name":      "last-owner",
						"namespace": "test",
						"labels": map[string]interface{}{
							"existing-label": "existing-value",
						},
						"annotations": map[string]interface{}{},
					},
				},
			},
			expectedLabels: map[string]string{
				constants.PriorityLabelKey: "high-priority",
				"extra-label":              "extra-value",
				"existing-label":           "existing-value",
			},
			expectedAnnotations: map[string]string{
				"annotation-1": "value-1",
			},
			description: "both labels and annotations should be propagated without overriding existing ones",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testClient := fake.NewClientBuilder().Build()
			testDefaultGrouper := defaultgrouper.NewDefaultGrouper("queue", "nodepool", testClient)
			testGrouper := NewSkipTopOwnerGrouper(testClient, testDefaultGrouper, map[metav1.GroupVersionKind]grouperplugin.Grouper{})

			// Deep copy to avoid modifying test data
			lastOwner := tt.lastOwner.DeepCopy()
			skippedOwner := tt.skippedOwner.DeepCopy()

			testGrouper.propagateMetadataDownChain(lastOwner, skippedOwner)

			// Verify labels
			if tt.expectedLabels != nil {
				assert.Equal(t, tt.expectedLabels, lastOwner.GetLabels(), tt.description+" (labels)")
			} else {
				// If no expected labels, lastOwner should have nil or empty labels
				labels := lastOwner.GetLabels()
				assert.True(t, labels == nil || len(labels) == 0, tt.description+" (labels should be empty)")
			}

			// Verify annotations
			if tt.expectedAnnotations != nil {
				assert.Equal(t, tt.expectedAnnotations, lastOwner.GetAnnotations(), tt.description+" (annotations)")
			} else {
				// If no expected annotations, lastOwner should have nil or empty annotations
				annotations := lastOwner.GetAnnotations()
				assert.True(t, annotations == nil || len(annotations) == 0, tt.description+" (annotations should be empty)")
			}
		})
	}
}
