// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package dynamo

import (
	"testing"

	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
	schedulingv1 "k8s.io/api/scheduling/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/NVIDIA/KAI-scheduler/pkg/apis/scheduling/v2alpha2"
	"github.com/NVIDIA/KAI-scheduler/pkg/podgrouper/podgrouper/plugins/constants"
	"github.com/NVIDIA/KAI-scheduler/pkg/podgrouper/podgrouper/plugins/defaultgrouper"
)

const (
	queueLabelKey    = "kai.scheduler/queue"
	nodePoolLabelKey = "kai.scheduler/node-pool"
)

func TestDynamoGrouper_WithPodCliqueSet(t *testing.T) {
	// Create DynamoGraphDeployment (top owner)
	dynamoGraphDeployment := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"kind":       "DynamoGraphDeployment",
			"apiVersion": "nvidia.com/v1alpha1",
			"metadata": map[string]interface{}{
				"name":      "test-dynamo",
				"namespace": "test-ns",
				"uid":       "dynamo-uid",
				"labels": map[string]interface{}{
					constants.PriorityLabelKey:       "high-priority",
					constants.PreemptibilityLabelKey: "non-preemptible",
				},
				"annotations": map[string]interface{}{
					"kai.scheduler/topology":                     "gpu",
					"kai.scheduler/topology-preferred-placement": "node",
					"kai.scheduler/topology-required-placement":  "rack",
				},
			},
			"spec": map[string]interface{}{
				"graphGroups": []interface{}{
					map[string]interface{}{
						"name": "dynamo-subgroup-1",
						"podReferences": []interface{}{
							map[string]interface{}{
								"namespace": "test-ns",
								"name":      "pod-1",
							},
							map[string]interface{}{
								"namespace": "test-ns",
								"name":      "pod-2",
							},
						},
						"minReplicas": int64(2),
					},
				},
			},
		},
	}
	dynamoGraphDeployment.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "nvidia.com",
		Kind:    "DynamoGraphDeployment",
		Version: "v1alpha1",
	})

	// Create PodCliqueSet with DynamoGraphDeployment as owner
	podCliqueSet := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"kind":       "PodCliqueSet",
			"apiVersion": "grove.io/v1alpha1",
			"metadata": map[string]interface{}{
				"name":      "test-podcliqueset",
				"namespace": "test-ns",
				"uid":       "podcliqueset-uid",
				"ownerReferences": []interface{}{
					map[string]interface{}{
						"apiVersion": "nvidia.com/v1alpha1",
						"kind":       "DynamoGraphDeployment",
						"name":       "test-dynamo",
						"uid":        "dynamo-uid",
					},
				},
				"labels": map[string]interface{}{
					"test_label": "test_value",
				},
			},
			"spec": map[string]interface{}{},
		},
	}
	podCliqueSet.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "grove.io",
		Kind:    "PodCliqueSet",
		Version: "v1alpha1",
	})

	pod := &v1.Pod{
		TypeMeta: metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "pod-1",
			Namespace: "test-ns",
			Labels: map[string]string{
				labelKeyPodGangName: "test-podgang",
			},
			UID: "pod-uid",
		},
		Spec:   v1.PodSpec{},
		Status: v1.PodStatus{},
	}

	// Create otherOwners to simulate the owner chain: Pod -> PodCliqueSet -> DynamoGraphDeployment
	podCliqueSetOwner := &metav1.PartialObjectMetadata{
		TypeMeta: metav1.TypeMeta{
			Kind:       "PodCliqueSet",
			APIVersion: "grove.io/v1alpha1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-podcliqueset",
			Namespace: "test-ns",
			UID:       types.UID("podcliqueset-uid"),
		},
	}

	// Create PriorityClass resources for validation
	highPriorityClass := &schedulingv1.PriorityClass{
		ObjectMeta: metav1.ObjectMeta{
			Name: "high-priority",
		},
		Value: 1000,
	}
	client := fake.NewClientBuilder().WithScheme(scheme.Scheme).WithRuntimeObjects(podCliqueSet, dynamoGraphDeployment, highPriorityClass).Build()
	defaultGrouper := defaultgrouper.NewDefaultGrouper(queueLabelKey, nodePoolLabelKey, client)
	grouper := NewDynamoGrouper(client, defaultGrouper)

	metadata, err := grouper.GetPodGroupMetadata(dynamoGraphDeployment, pod, podCliqueSetOwner)
	assert.Nil(t, err)
	assert.NotNil(t, metadata)

	// Verify metadata from DynamoGraphDeployment is applied
	assert.Equal(t, "high-priority", metadata.PriorityClassName)
	assert.Equal(t, v2alpha2.NonPreemptible, metadata.Preemptibility)
	assert.Equal(t, "gpu", metadata.Topology)
	assert.Equal(t, "node", metadata.PreferredTopologyLevel)
	assert.Equal(t, "rack", metadata.RequiredTopologyLevel)

	// Verify SubGroups from DynamoGraphDeployment spec.graphGroups
	assert.Equal(t, 1, len(metadata.SubGroups))
	assert.Equal(t, "dynamo-subgroup-1", metadata.SubGroups[0].Name)
	assert.Equal(t, int32(2), metadata.SubGroups[0].MinAvailable)
	assert.Equal(t, int32(2), metadata.MinAvailable)
}

func TestDynamoGrouper_WithLeaderWorkerSet(t *testing.T) {
	// Create DynamoGraphDeployment (top owner)
	dynamoGraphDeployment := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"kind":       "DynamoGraphDeployment",
			"apiVersion": "nvidia.com/v1alpha1",
			"metadata": map[string]interface{}{
				"name":      "test-dynamo",
				"namespace": "test-ns",
				"uid":       "dynamo-uid",
				"labels": map[string]interface{}{
					constants.PriorityLabelKey:       "medium-priority",
					constants.PreemptibilityLabelKey: "preemptible",
				},
				"annotations": map[string]interface{}{
					"kai.scheduler/topology": "cpu",
				},
			},
			"spec": map[string]interface{}{},
		},
	}
	dynamoGraphDeployment.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "nvidia.com",
		Kind:    "DynamoGraphDeployment",
		Version: "v1alpha1",
	})

	// Create LeaderWorkerSet with DynamoGraphDeployment as owner
	lws := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"kind":       "LeaderWorkerSet",
			"apiVersion": "leaderworkerset.x-k8s.io/v1",
			"metadata": map[string]interface{}{
				"name":      "test-lws",
				"namespace": "test-ns",
				"uid":       "lws-uid",
				"ownerReferences": []interface{}{
					map[string]interface{}{
						"apiVersion": "nvidia.com/v1alpha1",
						"kind":       "DynamoGraphDeployment",
						"name":       "test-dynamo",
						"uid":        "dynamo-uid",
					},
				},
			},
			"spec": map[string]interface{}{
				"startupPolicy": "LeaderCreated",
				"leaderWorkerTemplate": map[string]interface{}{
					"size": int64(3),
				},
			},
		},
	}
	lws.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "leaderworkerset.x-k8s.io",
		Kind:    "LeaderWorkerSet",
		Version: "v1",
	})

	pod := &v1.Pod{
		TypeMeta: metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "pod-1",
			Namespace: "test-ns",
			UID:       "pod-uid",
		},
		Spec:   v1.PodSpec{},
		Status: v1.PodStatus{},
	}

	// Create otherOwners to simulate the owner chain: Pod -> LeaderWorkerSet -> DynamoGraphDeployment
	lwsOwner := &metav1.PartialObjectMetadata{
		TypeMeta: metav1.TypeMeta{
			Kind:       "LeaderWorkerSet",
			APIVersion: "leaderworkerset.x-k8s.io/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-lws",
			Namespace: "test-ns",
			UID:       types.UID("lws-uid"),
		},
	}

	// Create PriorityClass resources for validation
	mediumPriorityClass := &schedulingv1.PriorityClass{
		ObjectMeta: metav1.ObjectMeta{
			Name: "medium-priority",
		},
		Value: 500,
	}
	client := fake.NewClientBuilder().WithScheme(scheme.Scheme).WithRuntimeObjects(lws, dynamoGraphDeployment, mediumPriorityClass).Build()
	defaultGrouper := defaultgrouper.NewDefaultGrouper(queueLabelKey, nodePoolLabelKey, client)
	grouper := NewDynamoGrouper(client, defaultGrouper)

	metadata, err := grouper.GetPodGroupMetadata(dynamoGraphDeployment, pod, lwsOwner)
	assert.Nil(t, err)
	assert.NotNil(t, metadata)

	// Verify metadata from DynamoGraphDeployment is applied
	assert.Equal(t, "medium-priority", metadata.PriorityClassName)
	assert.Equal(t, v2alpha2.Preemptible, metadata.Preemptibility)
	assert.Equal(t, "cpu", metadata.Topology)

	// Verify LWS-specific metadata (MinAvailable should be from LWS)
	assert.Equal(t, int32(3), metadata.MinAvailable)
}

func TestDynamoGrouper_FindOwnedCRD_NoOwnerReferences(t *testing.T) {
	dynamoGraphDeployment := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"kind":       "DynamoGraphDeployment",
			"apiVersion": "nvidia.com/v1alpha1",
			"metadata": map[string]interface{}{
				"name":      "test-dynamo",
				"namespace": "test-ns",
				"uid":       "dynamo-uid",
			},
		},
	}
	dynamoGraphDeployment.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "nvidia.com",
		Kind:    "DynamoGraphDeployment",
		Version: "v1alpha1",
	})

	client := fake.NewClientBuilder().WithScheme(scheme.Scheme).Build()
	defaultGrouper := defaultgrouper.NewDefaultGrouper(queueLabelKey, nodePoolLabelKey, client)
	grouper := NewDynamoGrouper(client, defaultGrouper)

	pod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "pod-1",
			Namespace: "test-ns",
		},
	}

	// No otherOwners provided and no PodCliqueSet/LWS found in namespace
	_, err := grouper.GetPodGroupMetadata(dynamoGraphDeployment, pod)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no PodCliqueSet or LeaderWorkerSet found")
}

func TestDynamoGrouper_FindOwnedCRD_MultipleOwnerReferences(t *testing.T) {
	dynamoGraphDeployment := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"kind":       "DynamoGraphDeployment",
			"apiVersion": "nvidia.com/v1alpha1",
			"metadata": map[string]interface{}{
				"name":      "test-dynamo",
				"namespace": "test-ns",
				"uid":       "dynamo-uid",
			},
		},
	}
	dynamoGraphDeployment.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "nvidia.com",
		Kind:    "DynamoGraphDeployment",
		Version: "v1alpha1",
	})

	// Create both PodCliqueSet and LWS owners in the chain
	podCliqueSetOwner := &metav1.PartialObjectMetadata{
		TypeMeta: metav1.TypeMeta{
			Kind:       "PodCliqueSet",
			APIVersion: "grove.io/v1alpha1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-podcliqueset",
			Namespace: "test-ns",
			UID:       types.UID("podcliqueset-uid"),
		},
	}

	lwsOwner := &metav1.PartialObjectMetadata{
		TypeMeta: metav1.TypeMeta{
			Kind:       "LeaderWorkerSet",
			APIVersion: "leaderworkerset.x-k8s.io/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-lws",
			Namespace: "test-ns",
			UID:       types.UID("lws-uid"),
		},
	}

	client := fake.NewClientBuilder().WithScheme(scheme.Scheme).Build()
	defaultGrouper := defaultgrouper.NewDefaultGrouper(queueLabelKey, nodePoolLabelKey, client)
	grouper := NewDynamoGrouper(client, defaultGrouper)

	pod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "pod-1",
			Namespace: "test-ns",
		},
	}

	// Both PodCliqueSet and LWS in the owner chain
	_, err := grouper.GetPodGroupMetadata(dynamoGraphDeployment, pod, podCliqueSetOwner, lwsOwner)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "both PodCliqueSet and LeaderWorkerSet")
}

func TestDynamoGrouper_SearchForOwnedCRD_NotInOwnerChain(t *testing.T) {
	// Create DynamoGraphDeployment (top owner)
	dynamoGraphDeployment := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"kind":       "DynamoGraphDeployment",
			"apiVersion": "nvidia.com/v1alpha1",
			"metadata": map[string]interface{}{
				"name":      "test-dynamo",
				"namespace": "test-ns",
				"uid":       "dynamo-uid",
				"labels": map[string]interface{}{
					constants.PriorityLabelKey: "test-priority",
				},
			},
		},
	}
	dynamoGraphDeployment.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "nvidia.com",
		Kind:    "DynamoGraphDeployment",
		Version: "v1alpha1",
	})

	// Create PodCliqueSet with DynamoGraphDeployment as owner (not in pod's owner chain)
	podCliqueSet := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"kind":       "PodCliqueSet",
			"apiVersion": "grove.io/v1alpha1",
			"metadata": map[string]interface{}{
				"name":      "test-podcliqueset",
				"namespace": "test-ns",
				"uid":       "podcliqueset-uid",
				"ownerReferences": []interface{}{
					map[string]interface{}{
						"apiVersion": "nvidia.com/v1alpha1",
						"kind":       "DynamoGraphDeployment",
						"name":       "test-dynamo",
						"uid":        "dynamo-uid",
					},
				},
			},
			"spec": map[string]interface{}{},
		},
	}
	podCliqueSet.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "grove.io",
		Kind:    "PodCliqueSet",
		Version: "v1alpha1",
	})

	pod := &v1.Pod{
		TypeMeta: metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "pod-1",
			Namespace: "test-ns",
			Labels: map[string]string{
				labelKeyPodGangName: "test-podgang",
			},
			UID: "pod-uid",
		},
		Spec:   v1.PodSpec{},
		Status: v1.PodStatus{},
	}

	// No otherOwners - PodCliqueSet is not in the pod's owner chain
	// But it exists in the namespace with DynamoGraphDeployment as owner
	// Create PriorityClass resource for validation
	testPriorityClass := &schedulingv1.PriorityClass{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-priority",
		},
		Value: 750,
	}
	client := fake.NewClientBuilder().WithScheme(scheme.Scheme).WithRuntimeObjects(podCliqueSet, dynamoGraphDeployment, testPriorityClass).Build()
	defaultGrouper := defaultgrouper.NewDefaultGrouper(queueLabelKey, nodePoolLabelKey, client)
	grouper := NewDynamoGrouper(client, defaultGrouper)

	metadata, err := grouper.GetPodGroupMetadata(dynamoGraphDeployment, pod)
	assert.Nil(t, err)
	assert.NotNil(t, metadata)

	// Verify metadata from DynamoGraphDeployment is applied
	assert.Equal(t, "test-priority", metadata.PriorityClassName)
}

func TestTransferMetadataToCRD(t *testing.T) {
	dynamoGraphDeployment := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"kind":       "DynamoGraphDeployment",
			"apiVersion": "nvidia.com/v1alpha1",
			"metadata": map[string]interface{}{
				"name":      "test-dynamo",
				"namespace": "test-ns",
				"labels": map[string]interface{}{
					constants.PriorityLabelKey:       "test-priority",
					constants.PreemptibilityLabelKey: "non-preemptible",
				},
				"annotations": map[string]interface{}{
					"kai.scheduler/topology":                     "test-topology",
					"kai.scheduler/topology-preferred-placement": "preferred-level",
					"kai.scheduler/topology-required-placement":  "required-level",
				},
			},
		},
	}
	dynamoGraphDeployment.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "nvidia.com",
		Kind:    "DynamoGraphDeployment",
		Version: "v1alpha1",
	})

	podCliqueSet := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"kind":       "PodCliqueSet",
			"apiVersion": "grove.io/v1alpha1",
			"metadata": map[string]interface{}{
				"name":      "test-podcliqueset",
				"namespace": "test-ns",
			},
		},
	}
	podCliqueSet.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "grove.io",
		Kind:    "PodCliqueSet",
		Version: "v1alpha1",
	})

	client := fake.NewClientBuilder().WithScheme(scheme.Scheme).Build()
	defaultGrouper := defaultgrouper.NewDefaultGrouper(queueLabelKey, nodePoolLabelKey, client)
	grouper := NewDynamoGrouper(client, defaultGrouper)

	modifiedCRD := grouper.transferMetadataToCRD(podCliqueSet, dynamoGraphDeployment)

	// Verify labels were transferred
	labels := modifiedCRD.GetLabels()
	assert.Equal(t, "test-priority", labels[constants.PriorityLabelKey])
	assert.Equal(t, "non-preemptible", labels[constants.PreemptibilityLabelKey])

	// Verify annotations were transferred
	annotations := modifiedCRD.GetAnnotations()
	assert.Equal(t, "test-topology", annotations["kai.scheduler/topology"])
	assert.Equal(t, "preferred-level", annotations["kai.scheduler/topology-preferred-placement"])
	assert.Equal(t, "required-level", annotations["kai.scheduler/topology-required-placement"])
	assert.Equal(t, "test-topology", annotations["grove.io/topology-name"]) // Grove compatibility
}

func TestParseGroveSubGroup_Success(t *testing.T) {
	input := map[string]interface{}{
		"name":        "mysubgroup",
		"minReplicas": int64(2),
		"podReferences": []interface{}{
			map[string]interface{}{"namespace": "ns", "name": "a"},
			map[string]interface{}{"namespace": "ns", "name": "b"},
		},
	}
	subgroup, err := parseGroveSubGroup(input, 0, "ns", "pg")
	assert.NoError(t, err)
	assert.Equal(t, "mysubgroup", subgroup.Name)
	assert.Equal(t, int32(2), subgroup.MinAvailable)
	assert.Equal(t, 2, len(subgroup.PodsReferences))
	assert.Equal(t, "a", subgroup.PodsReferences[0].Name)
	assert.Equal(t, "ns", subgroup.PodsReferences[0].Namespace)
	assert.Equal(t, "b", subgroup.PodsReferences[1].Name)
	assert.Equal(t, "ns", subgroup.PodsReferences[1].Namespace)
}

func TestParseGroveSubGroup_MissingFields(t *testing.T) {
	// Missing name
	input := map[string]interface{}{
		"minReplicas": int64(1),
		"podReferences": []interface{}{
			map[string]interface{}{"namespace": "ns", "name": "p"},
		},
	}
	_, err := parseGroveSubGroup(input, 0, "ns", "gang")
	assert.Error(t, err)
	assert.Equal(t, err.Error(), "missing required 'name' field")

	// Missing minReplicas
	input = map[string]interface{}{
		"name": "sg",
		"podReferences": []interface{}{
			map[string]interface{}{"namespace": "ns", "name": "p"},
		},
	}
	_, err = parseGroveSubGroup(input, 0, "ns", "gang")
	assert.Error(t, err)
	assert.Equal(t, err.Error(), "missing required 'minReplicas' field")

	// Missing podReferences
	input = map[string]interface{}{
		"name":        "sg",
		"minReplicas": int64(1),
	}
	_, err = parseGroveSubGroup(input, 0, "ns", "gang")
	assert.Error(t, err)
	assert.Equal(t, err.Error(), "missing required 'podReferences' field")
}

func TestParseGroveSubGroup_NegativeMinAvailable(t *testing.T) {
	input := map[string]interface{}{
		"name":        "sg",
		"minReplicas": int64(-1),
		"podReferences": []interface{}{
			map[string]interface{}{"namespace": "ns"},
		},
	}
	_, err := parseGroveSubGroup(input, 1, "ns", "gang")
	assert.Error(t, err)
	assert.Equal(t, err.Error(), "invalid 'minReplicas' field. Must be greater than 0")
}

func TestParseGroveSubGroup_InvalidPodReference(t *testing.T) {
	input := map[string]interface{}{
		"name":        "sg",
		"minReplicas": int64(1),
		"podReferences": []interface{}{
			"notamap",
		},
	}
	_, err := parseGroveSubGroup(input, 2, "ns", "gg")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid spec.podgroup[2].podReferences[0]")
}

func TestParsePodReference_Success(t *testing.T) {
	ref := map[string]interface{}{
		"namespace": "ns1",
		"name":      "mypod",
	}
	nn, err := parsePodReference(ref)
	assert.NoError(t, err)
	assert.Equal(t, &types.NamespacedName{Namespace: "ns1", Name: "mypod"}, nn)
}

func TestParseGroveSubGroup_ParsePodReferenceError(t *testing.T) {
	input := map[string]interface{}{
		"name":        "sg",
		"minReplicas": int64(1),
		"podReferences": []interface{}{
			map[string]interface{}{"namespace": "ns"},
		},
	}
	_, err := parseGroveSubGroup(input, 1, "ns", "gang")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "missing required 'name' field")
}

func TestParsePodReference_MissingFields(t *testing.T) {
	// Missing namespace
	ref := map[string]interface{}{"name": "pod"}
	_, err := parsePodReference(ref)
	assert.Error(t, err)
	assert.Equal(t, err.Error(), "missing required 'namespace' field")

	// Missing name
	ref = map[string]interface{}{"namespace": "ns"}
	_, err = parsePodReference(ref)
	assert.Error(t, err)
	assert.Equal(t, err.Error(), "missing required 'name' field")
}
