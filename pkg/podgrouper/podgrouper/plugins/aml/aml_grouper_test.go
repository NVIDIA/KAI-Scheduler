// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package aml

import (
	"testing"

	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/NVIDIA/KAI-scheduler/pkg/podgrouper/podgrouper/plugins/defaultgrouper"
)

const queueLabelKey = "kai.scheduler/queue"

func TestGetPodGroupMetadata(t *testing.T) {
	owner := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"kind":       "test_kind",
			"apiVersion": "test_version",
			"metadata": map[string]interface{}{
				"name":      "test_name",
				"namespace": "test_namespace",
				"uid":       "1",
				"labels": map[string]interface{}{
					"test_label": "test_value",
				},
				"annotations": map[string]interface{}{
					"test_annotation": "test_value",
				},
			},
			"spec": map[string]interface{}{
				"job": map[string]interface{}{
					"options": map[string]interface{}{
						"envs": map[string]interface{}{
							"AZUREML_NODE_COUNT": int64(3),
						},
					},
				},
			},
		},
	}
	pod := &v1.Pod{}

	amlGrouper := NewAmlGrouper(defaultgrouper.NewDefaultGrouper(queueLabelKey))
	podGroupMetadata, err := amlGrouper.GetPodGroupMetadata(owner, pod)

	assert.Nil(t, err)
	assert.Equal(t, "test_kind", podGroupMetadata.Owner.Kind)
	assert.Equal(t, "test_version", podGroupMetadata.Owner.APIVersion)
	assert.Equal(t, "1", string(podGroupMetadata.Owner.UID))
	assert.Equal(t, "test_name", podGroupMetadata.Owner.Name)
	assert.Equal(t, 3, len(podGroupMetadata.Annotations))
	assert.Equal(t, 1, len(podGroupMetadata.Labels))
	assert.Equal(t, "default-queue", podGroupMetadata.Queue)
	assert.Equal(t, "train", podGroupMetadata.PriorityClassName)
	assert.Equal(t, int32(3), podGroupMetadata.MinAvailable)
}

func TestGetPodGroupMetadataWithoutReplicas(t *testing.T) {
	owner := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"kind":       "test_kind",
			"apiVersion": "test_version",
			"metadata": map[string]interface{}{
				"name":      "test_name",
				"namespace": "test_namespace",
				"uid":       "1",
				"labels": map[string]interface{}{
					"test_label": "test_value",
				},
				"annotations": map[string]interface{}{
					"test_annotation": "test_value",
				},
			},
			"spec": map[string]interface{}{
				"job": map[string]interface{}{
					"options": map[string]interface{}{
						"envs": map[string]interface{}{},
					},
				},
			},
		},
	}
	pod := &v1.Pod{}

	amlGrouper := NewAmlGrouper(defaultgrouper.NewDefaultGrouper(queueLabelKey))
	_, err := amlGrouper.GetPodGroupMetadata(owner, pod)

	assert.NotNil(t, err)
}
