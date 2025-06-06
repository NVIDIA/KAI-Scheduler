// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package podgang

import (
	"fmt"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/NVIDIA/KAI-scheduler/pkg/podgrouper/podgroup"
	"github.com/NVIDIA/KAI-scheduler/pkg/podgrouper/podgrouper/plugins/defaultgrouper"
)

type PodGangGrouper struct {
	*defaultgrouper.DefaultGrouper
}

func NewPodGangGrouper(defaultGrouper *defaultgrouper.DefaultGrouper) *PodGangGrouper {
	return &PodGangGrouper{
		defaultGrouper,
	}
}

func (pgg *PodGangGrouper) Name() string {
	return "PodGang Grouper"
}

// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch
// +kubebuilder:rbac:groups=apps,resources=deployments/finalizers,verbs=patch;update;create

func (pgg *PodGangGrouper) GetPodGroupMetadata(
	topOwner *unstructured.Unstructured, pod *v1.Pod, _ ...*metav1.PartialObjectMetadata,
) (*podgroup.Metadata, error) {
	metadata, err := pgg.DefaultGrouper.GetPodGroupMetadata(topOwner, pod)
	if err != nil {
		return nil, err
	}

	priorityClassName, found, err := unstructured.NestedString(topOwner.Object, "spec", "priorityClassName")
	if err != nil {
		return nil, err
	}
	if found {
		metadata.PriorityClassName = priorityClassName
	}

	var minAvailable int64
	pgs, found, err := unstructured.NestedSlice(topOwner.Object, "spec", "podgroups")
	if err != nil {
		return nil, err
	}
	for _, v := range pgs {
		pgr, ok := v.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("invalid podgang structure")
		}
		minReplicas, found, err := unstructured.NestedInt64(pgr, "minReplicas")
		if err != nil {
			return nil, err
		}
		if found {
			minAvailable += minReplicas
		}
	}
	metadata.MinAvailable = int32(minAvailable)

	return metadata, nil
}
