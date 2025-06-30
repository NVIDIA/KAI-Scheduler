// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package grove

import (
	"context"
	"fmt"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/NVIDIA/KAI-scheduler/pkg/podgrouper/podgroup"
	"github.com/NVIDIA/KAI-scheduler/pkg/podgrouper/podgrouper/plugins/defaultgrouper"
)

const (
	podGangNameKey = "grove.io/podgang"
)

type GroveGrouper struct {
	client client.Client
	*defaultgrouper.DefaultGrouper
}

func NewGroveGrouper(client client.Client, defaultGrouper *defaultgrouper.DefaultGrouper) *GroveGrouper {
	return &GroveGrouper{
		client:         client,
		DefaultGrouper: defaultGrouper,
	}
}

func (gg *GroveGrouper) Name() string {
	return "Grove Grouper"
}

// +kubebuilder:rbac:groups=grove.io,resources=podgangsets,verbs=get;list;watch
// +kubebuilder:rbac:groups=grove.io,resources=podgangsets/finalizers,verbs=patch;update;create
// +kubebuilder:rbac:groups=grove.io,resources=podcliques,verbs=get;list;watch
// +kubebuilder:rbac:groups=grove.io,resources=podcliques/finalizers,verbs=patch;update;create
// +kubebuilder:rbac:groups=scheduler.grove.io,resources=podgangs,verbs=get;list;watch
// +kubebuilder:rbac:groups=scheduler.grove.io,resources=podgangs/finalizers,verbs=patch;update;create

func (gg *GroveGrouper) GetPodGroupMetadata(
	_ *unstructured.Unstructured, pod *v1.Pod, _ ...*metav1.PartialObjectMetadata,
) (*podgroup.Metadata, error) {
	if pod.Labels == nil {
		return nil, fmt.Errorf("Labels map not found in pod %s/%s", pod.Namespace, pod.Name)
	}

	podGangName, ok := pod.Labels[podGangNameKey]
	if !ok {
		return nil, fmt.Errorf("Label for podgang name (key: %s) not found in pod %s/%s",
			podGangNameKey, pod.Namespace, pod.Name)
	}

	podGang := &unstructured.Unstructured{}
	podGang.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "scheduler.grove.io",
		Kind:    "podgang",
		Version: "v1alpha1",
	})

	err := gg.client.Get(context.Background(), client.ObjectKey{
		Namespace: pod.Namespace,
		Name:      podGangName,
	}, podGang)
	if err != nil {
		return nil, err
	}

	metadata, err := gg.DefaultGrouper.GetPodGroupMetadata(podGang, pod)
	if err != nil {
		return nil, err
	}

	priorityClassName, found, err := unstructured.NestedString(podGang.Object, "spec", "priorityClassName")
	if err != nil {
		return nil, err
	}
	if found {
		metadata.PriorityClassName = priorityClassName
	}

	var minAvailable int32
	pgSlice, found, err := unstructured.NestedSlice(podGang.Object, "spec", "podgroups")
	if err != nil {
		return nil, err
	}
	for _, v := range pgSlice {
		pgr, ok := v.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("invalid podgang structure")
		}
		podSlice, found, err := unstructured.NestedSlice(pgr, "podReferences")
		if err != nil {
			return nil, err
		}
		if !found {
			return nil, fmt.Errorf("unexpected podgroup structure")
		}
		minReplicas, found, err := unstructured.NestedInt64(pgr, "minReplicas")
		if err != nil {
			return nil, err
		}
		if found && int(minReplicas) != len(podSlice) {
			return nil, fmt.Errorf("Unsupported minReplicas: expected: %v, found: %v", len(podSlice), minReplicas)
		}
		minAvailable += int32(len(podSlice))
	}
	metadata.MinAvailable = minAvailable

	return metadata, nil
}
