// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package defaultgrouper

import (
	"testing"

	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
	v12 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

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
		},
	}
	pod := &v1.Pod{}

	defaultGrouper := NewDefaultGrouper(queueLabelKey, nodePoolLabelKey)
	podGroupMetadata, err := defaultGrouper.GetPodGroupMetadata(owner, pod)

	assert.Nil(t, err)
	assert.Equal(t, "test_kind", podGroupMetadata.Owner.Kind)
	assert.Equal(t, "test_version", podGroupMetadata.Owner.APIVersion)
	assert.Equal(t, "1", string(podGroupMetadata.Owner.UID))
	assert.Equal(t, "test_name", podGroupMetadata.Owner.Name)
	assert.Equal(t, 3, len(podGroupMetadata.Annotations))
	assert.Equal(t, 1, len(podGroupMetadata.Labels))
	assert.Equal(t, "default-queue", podGroupMetadata.Queue)
	assert.Equal(t, "train", podGroupMetadata.PriorityClassName)
}

func TestGetPodGroupMetadataOnQueueFromOwnerDefaultNP(t *testing.T) {
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
					"project":    "my-proj",
				},
				"annotations": map[string]interface{}{
					"test_annotation": "test_value",
				},
			},
		},
	}
	pod := &v1.Pod{}

	defaultGrouper := NewDefaultGrouper(queueLabelKey, nodePoolLabelKey)
	podGroupMetadata, err := defaultGrouper.GetPodGroupMetadata(owner, pod)

	assert.Nil(t, err)
	assert.Equal(t, "my-proj", podGroupMetadata.Queue)
}

func TestGetPodGroupMetadataInferQueueFromProjectNodepool(t *testing.T) {
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
					"project":    "my-proj",
				},
				"annotations": map[string]interface{}{
					"test_annotation": "test_value",
				},
			},
		},
	}
	pod := &v1.Pod{
		ObjectMeta: v12.ObjectMeta{
			Labels: map[string]string{
				nodePoolLabelKey: "np-1",
			},
		},
	}

	defaultGrouper := NewDefaultGrouper(queueLabelKey, nodePoolLabelKey)
	podGroupMetadata, err := defaultGrouper.GetPodGroupMetadata(owner, pod)

	assert.Nil(t, err)
	assert.Equal(t, "my-proj-np-1", podGroupMetadata.Queue)
}

func TestGetPodGroupMetadataOnQueueFromOwner(t *testing.T) {
	owner := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"kind":       "test_kind",
			"apiVersion": "test_version",
			"metadata": map[string]interface{}{
				"name":      "test_name",
				"namespace": "test_namespace",
				"uid":       "1",
				"labels": map[string]interface{}{
					"test_label":  "test_value",
					"project":     "my-proj",
					queueLabelKey: "my-proj-np-1",
				},
				"annotations": map[string]interface{}{
					"test_annotation": "test_value",
				},
			},
		},
	}
	pod := &v1.Pod{}

	defaultGrouper := NewDefaultGrouper(queueLabelKey, nodePoolLabelKey)
	podGroupMetadata, err := defaultGrouper.GetPodGroupMetadata(owner, pod)

	assert.Nil(t, err)
	assert.Equal(t, "my-proj-np-1", podGroupMetadata.Queue)
}

func TestGetPodGroupMetadataOnQueueFromPod(t *testing.T) {
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
		},
	}
	pod := &v1.Pod{
		ObjectMeta: v12.ObjectMeta{
			Labels: map[string]string{
				queueLabelKey: "my-queue",
			},
		},
	}

	defaultGrouper := NewDefaultGrouper(queueLabelKey, nodePoolLabelKey)
	podGroupMetadata, err := defaultGrouper.GetPodGroupMetadata(owner, pod)

	assert.Nil(t, err)
	assert.Equal(t, "my-queue", podGroupMetadata.Queue)
}

func TestGetPodGroupMetadataOnPriorityClassFromOwner(t *testing.T) {
	owner := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"kind":       "test_kind",
			"apiVersion": "test_version",
			"metadata": map[string]interface{}{
				"name":      "test_name",
				"namespace": "test_namespace",
				"uid":       "1",
				"labels": map[string]interface{}{
					"test_label":        "test_value",
					"priorityClassName": "my-priority",
				},
				"annotations": map[string]interface{}{
					"test_annotation": "test_value",
				},
			},
		},
	}
	pod := &v1.Pod{}

	defaultGrouper := NewDefaultGrouper(queueLabelKey, nodePoolLabelKey)
	podGroupMetadata, err := defaultGrouper.GetPodGroupMetadata(owner, pod)

	assert.Nil(t, err)
	assert.Equal(t, "my-priority", podGroupMetadata.PriorityClassName)
}

func TestGetPodGroupMetadataOnPriorityClassFromPod(t *testing.T) {
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
		},
	}
	pod := &v1.Pod{
		ObjectMeta: v12.ObjectMeta{
			Labels: map[string]string{
				"priorityClassName": "my-priority",
			},
		},
	}

	defaultGrouper := NewDefaultGrouper(queueLabelKey, nodePoolLabelKey)
	podGroupMetadata, err := defaultGrouper.GetPodGroupMetadata(owner, pod)

	assert.Nil(t, err)
	assert.Equal(t, "my-priority", podGroupMetadata.PriorityClassName)
}

func TestGetPodGroupMetadataOnPriorityClassFromPodSpec(t *testing.T) {
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
		},
	}
	pod := &v1.Pod{
		Spec: v1.PodSpec{
			PriorityClassName: "my-priority",
		},
	}

	defaultGrouper := NewDefaultGrouper(queueLabelKey, nodePoolLabelKey)
	podGroupMetadata, err := defaultGrouper.GetPodGroupMetadata(owner, pod)

	assert.Nil(t, err)
	assert.Equal(t, "my-priority", podGroupMetadata.PriorityClassName)
}

func TestGetPodGroupMetadata_OwnerUserOverridePodUser(t *testing.T) {
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
					"user":       "ownerUser",
				},
				"annotations": map[string]interface{}{
					"test_annotation": "test_value",
				},
			},
		},
	}
	pod := &v1.Pod{
		ObjectMeta: v12.ObjectMeta{
			Labels: map[string]string{
				"user": "podUser",
			},
		},
	}

	defaultGrouper := NewDefaultGrouper(queueLabelKey, nodePoolLabelKey)
	podGroupMetadata, err := defaultGrouper.GetPodGroupMetadata(owner, pod)

	assert.Nil(t, err)
	assert.Equal(t, "ownerUser", podGroupMetadata.Labels["user"])
}
