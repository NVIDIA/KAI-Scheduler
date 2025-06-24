// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package leader_worker_set

import (
	"fmt"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/NVIDIA/KAI-scheduler/pkg/podgrouper/podgroup"
	"github.com/NVIDIA/KAI-scheduler/pkg/podgrouper/podgrouper/plugins/defaultgrouper"
)

type LwsGrouper struct {
	*defaultgrouper.DefaultGrouper
}

func NewLwsGrouper(defaultGrouper *defaultgrouper.DefaultGrouper) *LwsGrouper {
	return &LwsGrouper{
		DefaultGrouper: defaultGrouper,
	}
}

func (lwsGrouper *LwsGrouper) Name() string {
	return "LWS Grouper"
}

// +kubebuilder:rbac:groups=lws.k8s.io,resources=leaderworkerset,verbs=get;list;watch
// +kubebuilder:rbac:groups=lws.k8s.io,resources=leaderworkerset/finalizers,verbs=patch;update;create

func (lwsGrouper *LwsGrouper) GetPodGroupMetadata(
	lwsJob *unstructured.Unstructured, pod *v1.Pod, _ ...*metav1.PartialObjectMetadata,
) (*podgroup.Metadata, error) {
	podGroupMetadata, err := lwsGrouper.DefaultGrouper.GetPodGroupMetadata(lwsJob, pod)
	if err != nil {
		return nil, err
	}

	replicas, err := getLwsJobReplicas(lwsJob)
	if err != nil {
		return nil, err
	}

	startupPolicy, found, err := unstructured.NestedString(lwsJob.Object, "spec", "startupPolicy")
	if err != nil || !found {
		return nil, fmt.Errorf("failed to get startupPolicy: %w", err)
	}

	// Adjust minAvailable based on startup policy
	if startupPolicy == "Serial" {
		podGroupMetadata.MinAvailable = 1
	} else {
		podGroupMetadata.MinAvailable = replicas
	}

	podGroupMetadata.PriorityClassName = "inference"

	leaderPod := &metav1.OwnerReference{
		APIVersion: lwsJob.GetAPIVersion(),
		Kind:       lwsJob.GetKind(),
		Name:       lwsJob.GetName(),
		UID:        lwsJob.GetUID(),
	}
	podGroupMetadata.Owner = *leaderPod

	return podGroupMetadata, nil
}

func getLwsJobReplicas(lwsJob *unstructured.Unstructured) (int32, error) {
	lwsJobReplicas, found, err :=
		unstructured.NestedInt64(lwsJob.Object, "spec", "leaderWorkerTemplate", "replicas")
	if !found || err != nil {
		return 0, fmt.Errorf("error retrieving LWS replicas count from spec: %w", err)
	}
	return int32(lwsJobReplicas), nil
}
