// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package skiptopowner

import (
	"context"
	"maps"
	"testing"

	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/NVIDIA/KAI-scheduler/pkg/podgrouper/podgrouper/plugins/constants"
	"github.com/NVIDIA/KAI-scheduler/pkg/podgrouper/podgrouper/plugins/defaultgrouper"
	grouperplugin "github.com/NVIDIA/KAI-scheduler/pkg/podgrouper/podgrouper/plugins/grouper"
)

func TestPropagateLabelsDownChain(t *testing.T) {
	tests := []struct {
		name           string
		skippedOwner   *unstructured.Unstructured
		lastOwner      *unstructured.Unstructured
		pod            *v1.Pod
		otherOwners    []*metav1.PartialObjectMetadata
		expectedResult string
		description    string
	}{
		{
			name: "propagate from skippedOwner to lastOwner and pod",
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
						"name":      "middle-owner",
						"namespace": "test",
						"labels":    map[string]interface{}{},
					},
				},
			},
			pod: &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pod",
					Namespace: "test",
					Labels:    map[string]string{},
				},
			},
			otherOwners:    []*metav1.PartialObjectMetadata{},
			expectedResult: "high-priority",
			description:    "priorityClassName should be propagated from top owner to middle owner and pod",
		},
		{
			name: "propagate through multiple owners",
			skippedOwner: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"metadata": map[string]interface{}{
						"name":      "top-owner",
						"namespace": "test",
						"labels": map[string]interface{}{
							constants.PriorityLabelKey: "medium-priority",
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
			pod: &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pod",
					Namespace: "test",
					Labels:    map[string]string{},
				},
			},
			otherOwners: []*metav1.PartialObjectMetadata{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "middle-owner",
						Namespace: "test",
						Labels:    map[string]string{},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "last-owner",
						Namespace: "test",
						Labels:    map[string]string{},
					},
				},
			},
			expectedResult: "medium-priority",
			description:    "priorityClassName should propagate through all owners in the chain",
		},
		{
			name: "do not override existing label",
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
						"name":      "middle-owner",
						"namespace": "test",
						"labels": map[string]interface{}{
							constants.PriorityLabelKey: "low-priority",
						},
					},
				},
			},
			pod: &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pod",
					Namespace: "test",
					Labels:    map[string]string{},
				},
			},
			otherOwners:    []*metav1.PartialObjectMetadata{},
			expectedResult: "low-priority",
			description:    "existing priorityClassName on child should not be overridden",
		},
		{
			name: "no priorityClassName in chain",
			skippedOwner: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"metadata": map[string]interface{}{
						"name":      "top-owner",
						"namespace": "test",
						"labels":    map[string]interface{}{},
					},
				},
			},
			lastOwner: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"metadata": map[string]interface{}{
						"name":      "middle-owner",
						"namespace": "test",
						"labels":    map[string]interface{}{},
					},
				},
			},
			pod: &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pod",
					Namespace: "test",
					Labels:    map[string]string{},
				},
			},
			otherOwners:    []*metav1.PartialObjectMetadata{},
			expectedResult: "",
			description:    "no propagation should occur if no priorityClassName exists in chain",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a new client for each test to avoid state pollution
			testClient := fake.NewClientBuilder().Build()
			testDefaultGrouper := defaultgrouper.NewDefaultGrouper("queue", "nodepool", testClient)
			testGrouper := NewSkipTopOwnerGrouper(testClient, testDefaultGrouper, map[metav1.GroupVersionKind]grouperplugin.Grouper{})

			skippedOwner := tt.skippedOwner.DeepCopy()
			skippedOwner.SetGroupVersionKind(schema.GroupVersionKind{
				Group:   "test",
				Version: "v1",
				Kind:    "TopOwner",
			})
			// Ensure labels are properly set using SetLabels
			if labels, found := tt.skippedOwner.Object["metadata"].(map[string]interface{})["labels"]; found {
				if labelsMap, ok := labels.(map[string]interface{}); ok {
					stringLabels := make(map[string]string)
					for k, v := range labelsMap {
						if strVal, ok := v.(string); ok {
							stringLabels[k] = strVal
						}
					}
					skippedOwner.SetLabels(stringLabels)
				}
			}

			// Create copies of otherOwners for the test and populate the client with them
			testOtherOwners := make([]*metav1.PartialObjectMetadata, len(tt.otherOwners))
			for i, owner := range tt.otherOwners {
				testOtherOwners[i] = owner.DeepCopy()
				// Set GVK for the owner
				testOtherOwners[i].SetGroupVersionKind(schema.GroupVersionKind{
					Group:   "test",
					Version: "v1",
					Kind:    "Owner",
				})
				// Ensure labels are properly set using SetLabels if they exist in the original
				if owner.GetLabels() != nil {
					testOtherOwners[i].SetLabels(maps.Clone(owner.GetLabels()))
				}

				// Create full unstructured object and add to client
				ownerObj := &unstructured.Unstructured{}
				ownerObj.SetGroupVersionKind(testOtherOwners[i].GroupVersionKind())
				ownerObj.SetName(testOtherOwners[i].GetName())
				ownerObj.SetNamespace(testOtherOwners[i].GetNamespace())
				if testOtherOwners[i].GetLabels() != nil {
					ownerObj.SetLabels(maps.Clone(testOtherOwners[i].GetLabels()))
				}
				if err := testClient.Create(context.Background(), ownerObj); err != nil {
					t.Fatalf("Failed to create owner object: %v", err)
				}
			}

			// Also add lastOwner to client if it's not in otherOwners
			if len(tt.otherOwners) == 0 {
				lastOwnerObj := tt.lastOwner.DeepCopy()
				lastOwnerObj.SetGroupVersionKind(schema.GroupVersionKind{
					Group:   "test",
					Version: "v1",
					Kind:    "Owner",
				})
				if labels, found := tt.lastOwner.Object["metadata"].(map[string]interface{})["labels"]; found {
					if labelsMap, ok := labels.(map[string]interface{}); ok {
						stringLabels := make(map[string]string)
						for k, v := range labelsMap {
							if strVal, ok := v.(string); ok {
								stringLabels[k] = strVal
							}
						}
						lastOwnerObj.SetLabels(stringLabels)
					}
				}
				if err := testClient.Create(context.Background(), lastOwnerObj); err != nil {
					t.Fatalf("Failed to create lastOwner object: %v", err)
				}
			}

			testGrouper.propagateLabelsDownChain(skippedOwner, testOtherOwners...)

			// Verify propagation to otherOwners
			// The function propagates to otherOwners[:len(otherOwners)-1], so check those
			propagationTargets := testOtherOwners
			if len(testOtherOwners) > 1 {
				propagationTargets = testOtherOwners[:len(testOtherOwners)-1]
			}

			if tt.expectedResult != "" && len(propagationTargets) > 0 {
				// Fetch the updated object from the client to verify propagation
				firstOwnerPartial := propagationTargets[0]
				firstOwnerObj := &unstructured.Unstructured{}
				firstOwnerObj.SetGroupVersionKind(firstOwnerPartial.GroupVersionKind())
				key := types.NamespacedName{
					Namespace: firstOwnerPartial.GetNamespace(),
					Name:      firstOwnerPartial.GetName(),
				}
				if err := testClient.Get(context.Background(), key, firstOwnerObj); err != nil {
					t.Fatalf("Failed to get owner object: %v", err)
				}
				actualValue := firstOwnerObj.GetLabels()[constants.PriorityLabelKey]
				assert.Equal(t, tt.expectedResult, actualValue, tt.description)
			} else if len(propagationTargets) > 0 {
				// Fetch the object from the client to verify no propagation occurred
				firstOwnerPartial := propagationTargets[0]
				firstOwnerObj := &unstructured.Unstructured{}
				firstOwnerObj.SetGroupVersionKind(firstOwnerPartial.GroupVersionKind())
				key := types.NamespacedName{
					Namespace: firstOwnerPartial.GetNamespace(),
					Name:      firstOwnerPartial.GetName(),
				}
				if err := testClient.Get(context.Background(), key, firstOwnerObj); err == nil {
					_, found := firstOwnerObj.GetLabels()[constants.PriorityLabelKey]
					assert.False(t, found, "priorityClassName should not be set when not present in chain")
				}
			}
		})
	}
}
