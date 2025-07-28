// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package grove

import (
	"testing"

	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/NVIDIA/KAI-scheduler/pkg/podgrouper/podgrouper/plugins/constants"
	"github.com/NVIDIA/KAI-scheduler/pkg/podgrouper/podgrouper/plugins/defaultgrouper"
)

const (
	queueLabelKey    = "kai.scheduler/queue"
	nodePoolLabelKey = "kai.scheduler/node-pool"
)

func TestGetPodGroupMetadata(t *testing.T) {
	podgang := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"kind":       "PodGang",
			"apiVersion": "scheduler.grove.io/v1alpha1",
			"metadata": map[string]interface{}{
				"name":      "pgs1",
				"namespace": "test-ns",
				"uid":       "1",
				"labels": map[string]interface{}{
					"test_label": "test_value",
				},
				"annotations": map[string]interface{}{
					"test_annotation": "test_value",
				},
			},
			"spec": map[string]interface{}{
				"podgroups": []interface{}{
					map[string]interface{}{
						"podReferences": []interface{}{
							map[string]interface{}{
								"namespace": "test-ns",
								"name":      "pgs1-pga1",
							},
							map[string]interface{}{
								"namespace": "test-ns",
								"name":      "pgs1-pga2",
							},
							map[string]interface{}{
								"namespace": "test-ns",
								"name":      "pgs1-pga3",
							},
							map[string]interface{}{
								"namespace": "test-ns",
								"name":      "pgs1-pga4",
							},
						},
						"minReplicas": int64(4),
					},
					map[string]interface{}{
						"podReferences": []interface{}{
							map[string]interface{}{
								"namespace": "test-ns",
								"name":      "pgs1-pgb1",
							},
							map[string]interface{}{
								"namespace": "test-ns",
								"name":      "pgs1-pgb2",
							},
							map[string]interface{}{
								"namespace": "test-ns",
								"name":      "pgs1-pgb3",
							},
						},
						"minReplicas": int64(3),
					},
					map[string]interface{}{
						"podReferences": []interface{}{
							map[string]interface{}{
								"namespace": "test-ns",
								"name":      "pgs1-pgc1",
							},
							map[string]interface{}{
								"namespace": "test-ns",
								"name":      "pgs1-pgc2",
							},
							map[string]interface{}{
								"namespace": "test-ns",
								"name":      "pgs1-pgc3",
							},
							map[string]interface{}{
								"namespace": "test-ns",
								"name":      "pgs1-pgc4",
							},
							map[string]interface{}{
								"namespace": "test-ns",
								"name":      "pgs1-pgc5",
							},
						},
						"minReplicas": int64(5),
					},
				},
				"priorityClassName": "inference",
			},
		},
	}

	pod := &v1.Pod{
		TypeMeta: metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "pgs1-pga1",
			Namespace: "test-ns",
			Labels: map[string]string{
				queueLabelKey:       "test_queue",
				labelKeyPodGangName: "pgs1",
			},
			UID: "100",
		},
		Spec:   v1.PodSpec{},
		Status: v1.PodStatus{},
	}

	client := fake.NewClientBuilder().WithScheme(scheme.Scheme).WithRuntimeObjects(podgang).Build()
	grouper := NewGroveGrouper(client, defaultgrouper.NewDefaultGrouper(queueLabelKey, nodePoolLabelKey, client))
	metadata, err := grouper.GetPodGroupMetadata(podgang, pod)
	assert.Nil(t, err)
	assert.Equal(t, int32(12), metadata.MinAvailable)
	assert.Equal(t, constants.InferencePriorityClass, metadata.PriorityClassName)
	assert.Equal(t, "test_queue", metadata.Queue)
}
