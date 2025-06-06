// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package podgang

import (
	"testing"

	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

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
				"podgroups": []map[string]interface{}{
					{
						"podReferences": []map[string]interface{}{
							{
								"namespace": "test-ns"
								"name": "pgs1-pga1"
							},
							{
								"namespace": "test-ns"
								"name": "pgs1-pga2"
							},
							{
								"namespace": "test-ns"
								"name": "pgs1-pga3"
							},
							{
								"namespace": "test-ns"
								"name": "pgs1-pga4"
							},
						},
						"minReplicas": 4,
					},
					{
						"podReferences": []map[string]interface{}{
							{
								"namespace": "test-ns"
								"name": "pgs1-pgb1"
							},
							{
								"namespace": "test-ns"
								"name": "pgs1-pgb2"
							},
							{
								"namespace": "test-ns"
								"name": "pgs1-pgb3"
							},
						},
						"minReplicas": 3,
					},
					{
						"podReferences": []map[string]interface{}{
							{
								"namespace": "test-ns"
								"name": "pgs1-pgc1"
							},
							{
								"namespace": "test-ns"
								"name": "pgs1-pgc2"
							},
							{
								"namespace": "test-ns"
								"name": "pgs1-pgc3"
							},
							{
								"namespace": "test-ns"
								"name": "pgs1-pgc4"
							},
							{
								"namespace": "test-ns"
								"name": "pgs1-pgc5"
							},
						},
						"minReplicas": 5,
					},
				},
				"priorityClassName": "inference",
			},
		},
	}

	pod1 := &v1.Pod{
		TypeMeta: metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "pgs1-pga1",
			Namespace: "test-ns",
			Labels: map[string]string{
				queueLabelKey: "test_queue",
			},
			UID: "3",
		},
		Spec:   v1.PodSpec{},
		Status: v1.PodStatus{},
	}

	grouper := NewPodGangGrouper(defaultgrouper.NewDefaultGrouper(queueLabelKey, nodePoolLabelKey))
	metadata, err := grouper.GetPodGroupMetadata(podgang, pod1)
	assert.Nil(t, err)
	assert.Equal(t, 12, metadata.minAvailable)
	assert.Equal(t, constants.InferencePriorityClass, metadata.PriorityClassName)
	assert.Equal(t, "test_queue", metadata.Queue)
}
