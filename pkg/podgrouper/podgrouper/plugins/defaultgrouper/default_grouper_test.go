// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package defaultgrouper

import (
	"testing"

	"github.com/NVIDIA/KAI-scheduler/pkg/podgrouper/podgrouper/plugins/constants"
	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
	v12 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
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
	assert.Equal(t, 2, len(podGroupMetadata.Annotations))
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

func TestGetPodGroupMetadataOnPriorityClassFromDefaultsGroupKindConfigMap(t *testing.T) {
	owner := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"kind":       "TestKind",
			"apiVersion": "apps/v1",
			"metadata": map[string]interface{}{
				"name":      "test_name",
				"namespace": "test_namespace",
				"uid":       "1",
			},
		},
	}
	pod := &v1.Pod{}

	defaultsConfigmap := &v1.ConfigMap{
		ObjectMeta: v12.ObjectMeta{
			Name:      "priorities-defaults",
			Namespace: "test_namespace_1",
		},
		Data: map[string]string{
			constants.DefaultPrioritiesConfigMapTypesKey: `[{"typeName":"TestKind.apps","priorityName":"high-priority"},{"typeName":"TestKind","priorityName":"low-priority"}]`,
		},
	}
	kubeClient := fake.NewFakeClient(defaultsConfigmap)

	defaultGrouper := NewDefaultGrouper(queueLabelKey, nodePoolLabelKey)
	defaultGrouper.SetDefaultPrioritiesConfigMapParams("priorities-defaults", "test_namespace_1", kubeClient)
	podGroupMetadata, err := defaultGrouper.GetPodGroupMetadata(owner, pod)

	assert.Nil(t, err)
	assert.Equal(t, "high-priority", podGroupMetadata.PriorityClassName)
}

func TestGetPodGroupMetadataOnPriorityClassFromDefaultsKindConfigMap(t *testing.T) {
	owner := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"kind":       "TestKind",
			"apiVersion": "apps/v1",
			"metadata": map[string]interface{}{
				"name":      "test_name",
				"namespace": "test_namespace",
				"uid":       "1",
			},
		},
	}
	pod := &v1.Pod{}

	defaultsConfigmap := &v1.ConfigMap{
		ObjectMeta: v12.ObjectMeta{
			Name:      "priorities-defaults",
			Namespace: "test_namespace_1",
		},
		Data: map[string]string{
			constants.DefaultPrioritiesConfigMapTypesKey: `[{"typeName":"TestKind.differentgroup","priorityName":"high-priority"},{"typeName":"TestKind","priorityName":"low-priority"}]`,
		},
	}
	kubeClient := fake.NewFakeClient(defaultsConfigmap)

	defaultGrouper := NewDefaultGrouper(queueLabelKey, nodePoolLabelKey)
	defaultGrouper.SetDefaultPrioritiesConfigMapParams("priorities-defaults", "test_namespace_1", kubeClient)
	podGroupMetadata, err := defaultGrouper.GetPodGroupMetadata(owner, pod)

	assert.Nil(t, err)
	assert.Equal(t, "low-priority", podGroupMetadata.PriorityClassName)
}

func TestGetPodGroupMetadataOnPriorityClassDefaultsConfigMapOverrideFromPodSpec(t *testing.T) {
	owner := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"kind":       "TestKind",
			"apiVersion": "apps/v1",
			"metadata": map[string]interface{}{
				"name":      "test_name",
				"namespace": "test_namespace",
				"uid":       "1",
				"labels": map[string]interface{}{
					"test_label": "test_value",
				},
			},
		},
	}
	pod := &v1.Pod{
		Spec: v1.PodSpec{
			PriorityClassName: "my-priority",
		},
	}

	defaultsConfigmap := &v1.ConfigMap{
		ObjectMeta: v12.ObjectMeta{
			Name:      "priorities-defaults",
			Namespace: "test_namespace_1",
		},
		Data: map[string]string{
			constants.DefaultPrioritiesConfigMapTypesKey: `[{"typeName":"TestKind.apps","priorityName":"high-priority"},{"typeName":"TestKind","priorityName":"low-priority"}]`,
		},
	}
	kubeClient := fake.NewFakeClient(defaultsConfigmap)

	defaultGrouper := NewDefaultGrouper(queueLabelKey, nodePoolLabelKey)
	defaultGrouper.SetDefaultPrioritiesConfigMapParams("priorities-defaults", "test_namespace_1", kubeClient)
	podGroupMetadata, err := defaultGrouper.GetPodGroupMetadata(owner, pod)

	assert.Nil(t, err)
	assert.Equal(t, "my-priority", podGroupMetadata.PriorityClassName)
}

func TestGetPodGroupMetadataOnPriorityClassDefaultsConfigMapOverrideFromLabel(t *testing.T) {
	owner := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"kind":       "TestKind",
			"apiVersion": "apps/v1",
			"metadata": map[string]interface{}{
				"name":      "test_name",
				"namespace": "test_namespace",
				"uid":       "1",
				"labels": map[string]interface{}{
					"test_label":        "test_value",
					"priorityClassName": "my-priority-2",
				},
			},
		},
	}
	pod := &v1.Pod{
		Spec: v1.PodSpec{
			PriorityClassName: "my-priority",
		},
	}

	defaultsConfigmap := &v1.ConfigMap{
		ObjectMeta: v12.ObjectMeta{
			Name:      "priorities-defaults",
			Namespace: "test_namespace_1",
		},
		Data: map[string]string{
			constants.DefaultPrioritiesConfigMapTypesKey: `[{"typeName":"TestKind.apps","priorityName":"high-priority"},{"typeName":"TestKind","priorityName":"low-priority"}]`,
		},
	}
	kubeClient := fake.NewFakeClient(defaultsConfigmap)

	defaultGrouper := NewDefaultGrouper(queueLabelKey, nodePoolLabelKey)
	defaultGrouper.SetDefaultPrioritiesConfigMapParams("priorities-defaults", "test_namespace_1", kubeClient)
	podGroupMetadata, err := defaultGrouper.GetPodGroupMetadata(owner, pod)

	assert.Nil(t, err)
	assert.Equal(t, "my-priority-2", podGroupMetadata.PriorityClassName)
}

func TestGetPodGroupMetadataOnPriorityClassFromDefaultsConfigMapTestNils(t *testing.T) {
	owner := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"kind":       "TestKind",
			"apiVersion": "apps/v1",
			"metadata": map[string]interface{}{
				"name":      "test_name",
				"namespace": "test_namespace",
				"uid":       "1",
			},
		},
	}
	pod := &v1.Pod{}

	defaultsConfigmap := &v1.ConfigMap{
		ObjectMeta: v12.ObjectMeta{
			Name:      "priorities-defaults",
			Namespace: "test_namespace_1",
		},
		Data: map[string]string{
			constants.DefaultPrioritiesConfigMapTypesKey: `[{"typeName":"TestKind.apps","priorityName":"high-priority"},{"typeName":"TestKind","priorityName":"low-priority"}]`,
		},
	}
	kubeClient := fake.NewFakeClient(defaultsConfigmap)

	// sanity
	defaultGrouper := NewDefaultGrouper(queueLabelKey, nodePoolLabelKey)
	defaultGrouper.SetDefaultPrioritiesConfigMapParams("priorities-defaults", "test_namespace_1", kubeClient)
	podGroupMetadata, err := defaultGrouper.GetPodGroupMetadata(owner, pod)
	assert.Nil(t, err)
	assert.Equal(t, "high-priority", podGroupMetadata.PriorityClassName)

	// nil kubeclient
	defaultGrouper = NewDefaultGrouper(queueLabelKey, nodePoolLabelKey)
	defaultGrouper.SetDefaultPrioritiesConfigMapParams("priorities-defaults", "test_namespace_1", nil)
	podGroupMetadata, err = defaultGrouper.GetPodGroupMetadata(owner, pod)
	assert.Nil(t, err)
	assert.Equal(t, constants.TrainPriorityClass, podGroupMetadata.PriorityClassName)

	// unexisting configmap
	defaultGrouper = NewDefaultGrouper(queueLabelKey, nodePoolLabelKey)
	defaultGrouper.SetDefaultPrioritiesConfigMapParams("unexisting-cm", "test_namespace_1", kubeClient)
	podGroupMetadata, err = defaultGrouper.GetPodGroupMetadata(owner, pod)
	assert.Nil(t, err)
	assert.Equal(t, constants.TrainPriorityClass, podGroupMetadata.PriorityClassName)

	// empty group kind of object
	owner = &unstructured.Unstructured{
		Object: map[string]interface{}{
			"kind":       "",
			"apiVersion": "",
			"metadata": map[string]interface{}{
				"name":      "test_name",
				"namespace": "test_namespace",
				"uid":       "1",
			},
		},
	}
	defaultGrouper = NewDefaultGrouper(queueLabelKey, nodePoolLabelKey)
	defaultGrouper.SetDefaultPrioritiesConfigMapParams("priorities-defaults", "test_namespace_1", kubeClient)
	podGroupMetadata, err = defaultGrouper.GetPodGroupMetadata(owner, pod)
	assert.Nil(t, err)
	assert.Equal(t, constants.TrainPriorityClass, podGroupMetadata.PriorityClassName)
}

func TestGetPodGroupMetadataOnPriorityClassFromDefaultsConfigMapBadConfigmapData(t *testing.T) {
	owner := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"kind":       "TestKind",
			"apiVersion": "apps/v1",
			"metadata": map[string]interface{}{
				"name":      "test_name",
				"namespace": "test_namespace",
				"uid":       "1",
			},
		},
	}
	pod := &v1.Pod{}

	defaultsConfigmap := &v1.ConfigMap{
		ObjectMeta: v12.ObjectMeta{
			Name:      "priorities-defaults",
			Namespace: "test_namespace_1",
		},
		Data: map[string]string{
			constants.DefaultPrioritiesConfigMapTypesKey: `[bad-data!!!!!]`,
		},
	}
	kubeClient := fake.NewFakeClient(defaultsConfigmap)

	defaultGrouper := NewDefaultGrouper(queueLabelKey, nodePoolLabelKey)
	defaultGrouper.SetDefaultPrioritiesConfigMapParams("priorities-defaults", "test_namespace_1", kubeClient)
	podGroupMetadata, err := defaultGrouper.GetPodGroupMetadata(owner, pod)
	assert.Nil(t, err)
	assert.Equal(t, constants.TrainPriorityClass, podGroupMetadata.PriorityClassName)

	defaultsConfigmap.Data = map[string]string{"different-key!!!!": `[{"typeName":"TestKind.apps","priorityName":"high-priority"},{"typeName":"TestKind","priorityName":"low-priority"}]`}
	kubeClient = fake.NewFakeClient(defaultsConfigmap)
	defaultGrouper = NewDefaultGrouper(queueLabelKey, nodePoolLabelKey)
	defaultGrouper.SetDefaultPrioritiesConfigMapParams("priorities-defaults", "test_namespace_1", kubeClient)
	podGroupMetadata, err = defaultGrouper.GetPodGroupMetadata(owner, pod)
	assert.Nil(t, err)
	assert.Equal(t, constants.TrainPriorityClass, podGroupMetadata.PriorityClassName)
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
