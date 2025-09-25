// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package defaultgrouper

import (
	"testing"

	"github.com/NVIDIA/KAI-scheduler/pkg/podgrouper/podgrouper/plugins/constants"
	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
	schedulingv1 "k8s.io/api/scheduling/v1"
	v12 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

const (
	prioritiesConfigMapName      = "priorities-defaults"
	prioritiesConfigMapNamespace = "test_namespace_1"
)

func TestGetPodGroupMetadata(t *testing.T) {
	// Create the train priority class that the test expects
	trainPriorityClass := priorityClassObj(constants.TrainPriorityClass, 1000)
	kubeClient := fake.NewFakeClient(trainPriorityClass)

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

	defaultGrouper := NewDefaultGrouper(queueLabelKey, nodePoolLabelKey, kubeClient)
	defaultGrouper.SetDefaultPrioritiesConfigMapParams("", "")
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
	assert.Empty(t, podGroupMetadata.Preemptibility)
	assert.Empty(t, podGroupMetadata.PreferredTopologyLevel)
	assert.Empty(t, podGroupMetadata.RequiredTopologyLevel)
	assert.Empty(t, podGroupMetadata.Topology)
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

	defaultGrouper := NewDefaultGrouper(queueLabelKey, nodePoolLabelKey, fake.NewFakeClient())
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

	defaultGrouper := NewDefaultGrouper(queueLabelKey, nodePoolLabelKey, fake.NewFakeClient())
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

	defaultGrouper := NewDefaultGrouper(queueLabelKey, nodePoolLabelKey, fake.NewFakeClient())
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

	defaultGrouper := NewDefaultGrouper(queueLabelKey, nodePoolLabelKey, fake.NewFakeClient())
	podGroupMetadata, err := defaultGrouper.GetPodGroupMetadata(owner, pod)

	assert.Nil(t, err)
	assert.Equal(t, "my-queue", podGroupMetadata.Queue)
}

func TestGetPodGroupMetadataOnPriorityClassFromOwner(t *testing.T) {
	myPriorityClass := priorityClassObj("my-priority", 1000)
	kubeClient := fake.NewFakeClient(myPriorityClass)

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

	defaultGrouper := NewDefaultGrouper(queueLabelKey, nodePoolLabelKey, kubeClient)
	defaultGrouper.SetDefaultPrioritiesConfigMapParams("", "")
	podGroupMetadata, err := defaultGrouper.GetPodGroupMetadata(owner, pod)

	assert.Nil(t, err)
	assert.Equal(t, "my-priority", podGroupMetadata.PriorityClassName)
}

func TestGetPodGroupMetadataOnPriorityClassFromPod(t *testing.T) {
	myPriorityClass := priorityClassObj("my-priority", 1000)
	kubeClient := fake.NewFakeClient(myPriorityClass)

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

	defaultGrouper := NewDefaultGrouper(queueLabelKey, nodePoolLabelKey, kubeClient)
	defaultGrouper.SetDefaultPrioritiesConfigMapParams("", "")
	podGroupMetadata, err := defaultGrouper.GetPodGroupMetadata(owner, pod)

	assert.Nil(t, err)
	assert.Equal(t, "my-priority", podGroupMetadata.PriorityClassName)
}

func TestGetPodGroupMetadataOnPriorityClassFromPodSpec(t *testing.T) {
	myPriorityClass := priorityClassObj("my-priority", 1000)
	kubeClient := fake.NewFakeClient(myPriorityClass)

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

	defaultGrouper := NewDefaultGrouper(queueLabelKey, nodePoolLabelKey, kubeClient)
	defaultGrouper.SetDefaultPrioritiesConfigMapParams("", "")
	podGroupMetadata, err := defaultGrouper.GetPodGroupMetadata(owner, pod)

	assert.Nil(t, err)
	assert.Equal(t, "my-priority", podGroupMetadata.PriorityClassName)
}

func TestGetPodGroupMetadataOnPriorityClassFromDefaultsGroupKindConfigMap(t *testing.T) {
	// Create the priority class that the test expects
	highPriorityClass := priorityClassObj("high-priority", 1000)
	defaultsConfigmap := &v1.ConfigMap{
		ObjectMeta: v12.ObjectMeta{
			Name:      prioritiesConfigMapName,
			Namespace: prioritiesConfigMapNamespace,
		},
		Data: map[string]string{
			constants.DefaultPrioritiesConfigMapTypesKey: `[{"typeName":"TestKind","group":"apps","priorityName":"high-priority"},{"typeName":"TestKind","group":"","priorityName":"low-priority"}]`,
		},
	}
	kubeClient := fake.NewFakeClient(highPriorityClass, defaultsConfigmap)

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

	defaultGrouper := NewDefaultGrouper(queueLabelKey, nodePoolLabelKey, kubeClient)
	defaultGrouper.SetDefaultPrioritiesConfigMapParams(prioritiesConfigMapName, prioritiesConfigMapNamespace)
	podGroupMetadata, err := defaultGrouper.GetPodGroupMetadata(owner, pod)

	assert.Nil(t, err)
	assert.Equal(t, "high-priority", podGroupMetadata.PriorityClassName)
}

func TestGetPodGroupMetadataOnPriorityClassFromDefaultsKindConfigMap(t *testing.T) {
	lowPriorityClass := priorityClassObj("low-priority", 1000)
	defaultsConfigmap := &v1.ConfigMap{
		ObjectMeta: v12.ObjectMeta{
			Name:      prioritiesConfigMapName,
			Namespace: prioritiesConfigMapNamespace,
		},
		Data: map[string]string{
			constants.DefaultPrioritiesConfigMapTypesKey: `[{"typeName":"TestKind","group":"differentgroup","priorityName":"high-priority"},{"typeName":"TestKind","priorityName":"low-priority"}]`,
		},
	}
	kubeClient := fake.NewFakeClient(lowPriorityClass, defaultsConfigmap)

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

	defaultGrouper := NewDefaultGrouper(queueLabelKey, nodePoolLabelKey, kubeClient)
	defaultGrouper.SetDefaultPrioritiesConfigMapParams(prioritiesConfigMapName, prioritiesConfigMapNamespace)
	podGroupMetadata, err := defaultGrouper.GetPodGroupMetadata(owner, pod)

	assert.Nil(t, err)
	assert.Equal(t, "low-priority", podGroupMetadata.PriorityClassName)
}

func TestGetPodGroupMetadataOnPriorityClassDefaultsConfigMapOverrideFromPodSpec(t *testing.T) {
	myPriorityClass := priorityClassObj("my-priority", 1000)
	defaultsConfigmap := &v1.ConfigMap{
		ObjectMeta: v12.ObjectMeta{
			Name:      prioritiesConfigMapName,
			Namespace: prioritiesConfigMapNamespace,
		},
		Data: map[string]string{
			constants.DefaultPrioritiesConfigMapTypesKey: `[{"typeName":"TestKind","group":"apps","priorityName":"high-priority"},{"typeName":"TestKind","group":"","priorityName":"low-priority"}]`,
		},
	}
	kubeClient := fake.NewFakeClient(myPriorityClass, defaultsConfigmap)

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

	defaultGrouper := NewDefaultGrouper(queueLabelKey, nodePoolLabelKey, kubeClient)
	defaultGrouper.SetDefaultPrioritiesConfigMapParams(prioritiesConfigMapName, prioritiesConfigMapNamespace)
	podGroupMetadata, err := defaultGrouper.GetPodGroupMetadata(owner, pod)

	assert.Nil(t, err)
	assert.Equal(t, "my-priority", podGroupMetadata.PriorityClassName)
}

func TestGetPodGroupMetadataOnPriorityClassDefaultsConfigMapOverrideFromLabel(t *testing.T) {
	myPriority2Class := priorityClassObj("my-priority-2", 1000)
	defaultsConfigmap := &v1.ConfigMap{
		ObjectMeta: v12.ObjectMeta{
			Name:      prioritiesConfigMapName,
			Namespace: prioritiesConfigMapNamespace,
		},
		Data: map[string]string{
			constants.DefaultPrioritiesConfigMapTypesKey: `[{"typeName":"TestKind","group":"apps","priorityName":"high-priority"},{"typeName":"TestKind","group":"differentgroup","priorityName":"low-priority"}]`,
		},
	}
	kubeClient := fake.NewFakeClient(myPriority2Class, defaultsConfigmap)

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

	defaultGrouper := NewDefaultGrouper(queueLabelKey, nodePoolLabelKey, kubeClient)
	defaultGrouper.SetDefaultPrioritiesConfigMapParams(prioritiesConfigMapName, prioritiesConfigMapNamespace)
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

	highPriorityClass := priorityClassObj("high-priority", 1000)
	trainClass := priorityClassObj(constants.TrainPriorityClass, 1000)
	defaultsConfigmap := &v1.ConfigMap{
		ObjectMeta: v12.ObjectMeta{
			Name:      prioritiesConfigMapName,
			Namespace: prioritiesConfigMapNamespace,
		},
		Data: map[string]string{
			constants.DefaultPrioritiesConfigMapTypesKey: `[{"typeName":"TestKind","group":"apps","priorityName":"high-priority"},{"typeName":"TestKind","priorityName":"low-priority"}]`,
		},
	}
	kubeClient := fake.NewFakeClient(highPriorityClass, trainClass, defaultsConfigmap)

	// sanity
	defaultGrouper := NewDefaultGrouper(queueLabelKey, nodePoolLabelKey, kubeClient)
	defaultGrouper.SetDefaultPrioritiesConfigMapParams(prioritiesConfigMapName, prioritiesConfigMapNamespace)
	podGroupMetadata, err := defaultGrouper.GetPodGroupMetadata(owner, pod)
	assert.Nil(t, err)
	assert.Equal(t, "high-priority", podGroupMetadata.PriorityClassName)

	// unexisting configmap
	defaultGrouper = NewDefaultGrouper(queueLabelKey, nodePoolLabelKey, kubeClient)
	defaultGrouper.SetDefaultPrioritiesConfigMapParams("unexisting-cm", prioritiesConfigMapNamespace)
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
	defaultGrouper = NewDefaultGrouper(queueLabelKey, nodePoolLabelKey, kubeClient)
	defaultGrouper.SetDefaultPrioritiesConfigMapParams(prioritiesConfigMapName, prioritiesConfigMapNamespace)
	podGroupMetadata, err = defaultGrouper.GetPodGroupMetadata(owner, pod)
	assert.Nil(t, err)
	assert.Equal(t, constants.TrainPriorityClass, podGroupMetadata.PriorityClassName)
}

func TestGetPodGroupMetadataOnPriorityClassFromDefaultsConfigMapBadConfigmapData(t *testing.T) {
	// Create the train priority class that the test falls back to
	trainPriorityClass := priorityClassObj(constants.TrainPriorityClass, 1000)

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
			Name:      prioritiesConfigMapName,
			Namespace: prioritiesConfigMapNamespace,
		},
		Data: map[string]string{
			constants.DefaultPrioritiesConfigMapTypesKey: `[bad-data!!!!!]`,
		},
	}
	kubeClient := fake.NewFakeClient(trainPriorityClass, defaultsConfigmap)

	defaultGrouper := NewDefaultGrouper(queueLabelKey, nodePoolLabelKey, kubeClient)
	defaultGrouper.SetDefaultPrioritiesConfigMapParams(prioritiesConfigMapName, prioritiesConfigMapNamespace)
	podGroupMetadata, err := defaultGrouper.GetPodGroupMetadata(owner, pod)
	assert.Nil(t, err)
	assert.Equal(t, constants.TrainPriorityClass, podGroupMetadata.PriorityClassName)

	defaultsConfigmap.Data = map[string]string{"different-key!!!!": `[{"typeName":"TestKind.apps","priorityName":"high-priority"},{"typeName":"TestKind","priorityName":"low-priority"}]`}
	kubeClient = fake.NewFakeClient(trainPriorityClass, defaultsConfigmap)
	defaultGrouper = NewDefaultGrouper(queueLabelKey, nodePoolLabelKey, kubeClient)
	defaultGrouper.SetDefaultPrioritiesConfigMapParams(prioritiesConfigMapName, prioritiesConfigMapNamespace)
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

	defaultGrouper := NewDefaultGrouper(queueLabelKey, nodePoolLabelKey, fake.NewFakeClient())
	podGroupMetadata, err := defaultGrouper.GetPodGroupMetadata(owner, pod)

	assert.Nil(t, err)
	assert.Equal(t, "ownerUser", podGroupMetadata.Labels["user"])
}

func TestGetPodGroupMetadataWithTopology(t *testing.T) {
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
					"kai.scheduler/topology-preferred-placement": "rack",
					"kai.scheduler/topology-required-placement":  "zone",
					"kai.scheduler/topology":                     "network",
				},
			},
		},
	}
	pod := &v1.Pod{}

	defaultGrouper := NewDefaultGrouper(queueLabelKey, nodePoolLabelKey, fake.NewFakeClient())
	podGroupMetadata, err := defaultGrouper.GetPodGroupMetadata(owner, pod)

	assert.Nil(t, err)
	assert.Equal(t, "rack", podGroupMetadata.PreferredTopologyLevel)
	assert.Equal(t, "zone", podGroupMetadata.RequiredTopologyLevel)
	assert.Equal(t, "network", podGroupMetadata.Topology)
}

// TestCalcPodGroupPriorityClass_NonExistentDefaultFromConfigMap tests when default priority class from configmap doesn't exist
func TestCalcPodGroupPriorityClass_NonExistentDefaultFromConfigMap(t *testing.T) {
	defaultsConfigmap := &v1.ConfigMap{
		ObjectMeta: v12.ObjectMeta{
			Name:      prioritiesConfigMapName,
			Namespace: prioritiesConfigMapNamespace,
		},
		Data: map[string]string{
			constants.DefaultPrioritiesConfigMapTypesKey: `[{"typeName":"TestKind","group":"apps","priorityName":"non-existent-configmap-priority"}]`,
		},
	}
	kubeClient := fake.NewFakeClient(defaultsConfigmap)

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

	defaultGrouper := NewDefaultGrouper(queueLabelKey, nodePoolLabelKey, kubeClient)
	defaultGrouper.SetDefaultPrioritiesConfigMapParams(prioritiesConfigMapName, prioritiesConfigMapNamespace)
	podGroupMetadata, err := defaultGrouper.GetPodGroupMetadata(owner, pod)

	assert.Nil(t, err)
	// Should fall back to default since the configmap priority class doesn't exist
	assert.Equal(t, constants.TrainPriorityClass, podGroupMetadata.PriorityClassName)
}

// TestCalcPodGroupPriorityClass_ValidPriorityClassOverridesInvalidDefault tests when owner has valid priority class but configmap has invalid one
func TestCalcPodGroupPriorityClass_ValidPriorityClassOverridesInvalidDefault(t *testing.T) {
	// Create only the valid priority class
	validPriorityClass := priorityClassObj("valid-priority", 1000)
	kubeClient := fake.NewFakeClient(validPriorityClass)

	defaultsConfigmap := &v1.ConfigMap{
		ObjectMeta: v12.ObjectMeta{
			Name:      prioritiesConfigMapName,
			Namespace: prioritiesConfigMapNamespace,
		},
		Data: map[string]string{
			constants.DefaultPrioritiesConfigMapTypesKey: `[{"typeName":"TestKind","group":"apps","priorityName":"invalid-configmap-priority"}]`,
		},
	}
	kubeClient = fake.NewFakeClient(validPriorityClass, defaultsConfigmap)

	owner := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"kind":       "TestKind",
			"apiVersion": "apps/v1",
			"metadata": map[string]interface{}{
				"name":      "test_name",
				"namespace": "test_namespace",
				"uid":       "1",
				"labels": map[string]interface{}{
					"priorityClassName": "valid-priority",
				},
			},
		},
	}
	pod := &v1.Pod{}

	defaultGrouper := NewDefaultGrouper(queueLabelKey, nodePoolLabelKey, kubeClient)
	defaultGrouper.SetDefaultPrioritiesConfigMapParams(prioritiesConfigMapName, prioritiesConfigMapNamespace)
	podGroupMetadata, err := defaultGrouper.GetPodGroupMetadata(owner, pod)

	assert.Nil(t, err)
	// Should use the valid priority class from owner, not fall back to default
	assert.Equal(t, "valid-priority", podGroupMetadata.PriorityClassName)
}

// TestCalcPodGroupPriorityClass_InvalidPriorityClassFallsBackToConfigMap tests when invalid priority class is specified but configmap has valid one
func TestCalcPodGroupPriorityClass_InvalidPriorityClassFallsBackToConfigMap(t *testing.T) {
	// Create the configmap priority class
	configmapPriorityClass := priorityClassObj("configmap-priority", 1000)

	defaultsConfigmap := &v1.ConfigMap{
		ObjectMeta: v12.ObjectMeta{
			Name:      prioritiesConfigMapName,
			Namespace: prioritiesConfigMapNamespace,
		},
		Data: map[string]string{
			constants.DefaultPrioritiesConfigMapTypesKey: `[{"typeName":"TestKind","group":"apps","priorityName":"configmap-priority"}]`,
		},
	}
	kubeClient := fake.NewFakeClient(configmapPriorityClass, defaultsConfigmap)

	owner := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"kind":       "TestKind",
			"apiVersion": "apps/v1",
			"metadata": map[string]interface{}{
				"name":      "test_name",
				"namespace": "test_namespace",
				"uid":       "1",
				"labels": map[string]interface{}{
					"priorityClassName": "invalid-priority",
				},
			},
		},
	}
	pod := &v1.Pod{}

	defaultGrouper := NewDefaultGrouper(queueLabelKey, nodePoolLabelKey, kubeClient)
	defaultGrouper.SetDefaultPrioritiesConfigMapParams(prioritiesConfigMapName, prioritiesConfigMapNamespace)
	podGroupMetadata, err := defaultGrouper.GetPodGroupMetadata(owner, pod)

	assert.Nil(t, err)
	// Should use the configmap priority class since no explicit priority class was specified
	assert.Equal(t, "configmap-priority", podGroupMetadata.PriorityClassName)
}

func priorityClassObj(name string, value int32) *schedulingv1.PriorityClass {
	return &schedulingv1.PriorityClass{
		ObjectMeta: v12.ObjectMeta{
			Name: name,
		},
		Value: value,
	}
}

// TestCalcPodGroupPreemptibility tests for the CalcPodGroupPreemptibility method
func TestCalcPodGroupPreemptibility(t *testing.T) {
	tests := []struct {
		name           string
		ownerLabels    map[string]interface{}
		podLabels      map[string]string
		expectedResult string
	}{
		{
			name: "valid preemptible from owner",
			ownerLabels: map[string]interface{}{
				"kai.scheduler/preemptibility": "preemptible",
			},
			podLabels:      nil,
			expectedResult: "preemptible",
		},
		{
			name: "valid non-preemptible from owner",
			ownerLabels: map[string]interface{}{
				"kai.scheduler/preemptibility": "non-preemptible",
			},
			podLabels:      nil,
			expectedResult: "non-preemptible",
		},
		{
			name:        "valid preemptible from pod",
			ownerLabels: nil,
			podLabels: map[string]string{
				"kai.scheduler/preemptibility": "preemptible",
			},
			expectedResult: "preemptible",
		},
		{
			name:        "valid non-preemptible from pod",
			ownerLabels: nil,
			podLabels: map[string]string{
				"kai.scheduler/preemptibility": "non-preemptible",
			},
			expectedResult: "non-preemptible",
		},
		{
			name: "invalid value from owner",
			ownerLabels: map[string]interface{}{
				"kai.scheduler/preemptibility": "invalid-value",
			},
			podLabels:      nil,
			expectedResult: "",
		},
		{
			name:        "invalid value from pod",
			ownerLabels: nil,
			podLabels: map[string]string{
				"kai.scheduler/preemptibility": "invalid-value",
			},
			expectedResult: "",
		},
		{
			name:           "no labels",
			ownerLabels:    nil,
			podLabels:      nil,
			expectedResult: "",
		},
		{
			name: "owner overrides pod",
			ownerLabels: map[string]interface{}{
				"kai.scheduler/preemptibility": "non-preemptible",
			},
			podLabels: map[string]string{
				"kai.scheduler/preemptibility": "preemptible",
			},
			expectedResult: "non-preemptible",
		},
		{
			name: "owner invalid pod valid",
			ownerLabels: map[string]interface{}{
				"kai.scheduler/preemptibility": "invalid-value",
			},
			podLabels: map[string]string{
				"kai.scheduler/preemptibility": "preemptible",
			},
			expectedResult: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create owner with labels
			owner := &unstructured.Unstructured{
				Object: map[string]interface{}{
					"kind":       "test_kind",
					"apiVersion": "test_version",
					"metadata": map[string]interface{}{
						"name":      "test_name",
						"namespace": "test_namespace",
						"uid":       "1",
						"labels":    tt.ownerLabels,
					},
				},
			}

			// Create pod with labels
			pod := &v1.Pod{}
			if tt.podLabels != nil {
				pod.ObjectMeta = v12.ObjectMeta{
					Labels: tt.podLabels,
				}
			}

			defaultGrouper := NewDefaultGrouper(queueLabelKey, nodePoolLabelKey, fake.NewFakeClient())
			preemptibility := defaultGrouper.calcPodGroupPreemptibility(owner, pod)

			assert.Equal(t, tt.expectedResult, string(preemptibility))
		})
	}
}
