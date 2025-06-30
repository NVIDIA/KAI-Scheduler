// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package leader_worker_set

import (
	"fmt"
	"strconv"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/NVIDIA/KAI-scheduler/pkg/podgrouper/podgroup"
	"github.com/NVIDIA/KAI-scheduler/pkg/podgrouper/podgrouper/plugins/defaultgrouper"
)

const (
	startupPolicyLeaderReady   = "LeaderReady"
	startupPolicyLeaderCreated = "LeaderCreated"
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

// +kubebuilder:rbac:groups=leaderworkerset.x-k8s.io,resources=leaderworkersets,verbs=get;list;watch
// +kubebuilder:rbac:groups=leaderworkerset.x-k8s.io,resources=leaderworkersets/finalizers,verbs=patch;update;create

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

	startupPolicy := startupPolicyLeaderCreated
	if policy, found, err := unstructured.NestedString(lwsJob.Object, "spec", "startupPolicy"); err == nil && found {
		startupPolicy = policy
	}

	switch startupPolicy {
	case startupPolicyLeaderReady:
		if err := handleLeaderReadyPolicy(pod, podGroupMetadata, replicas); err != nil {
			return nil, fmt.Errorf("error handling leader ready policy: %w", err)
		}
	case startupPolicyLeaderCreated:
		podGroupMetadata.MinAvailable = replicas
	default:
		return nil, fmt.Errorf("unknown startupPolicy: %s", startupPolicy)
	}

	if groupIndex, ok := pod.Labels["leaderworkerset.x-k8s.io/group-index"]; ok {
		podGroupMetadata.Name = fmt.Sprintf("%s-group-%s", podGroupMetadata.Name, groupIndex)
	}

	podGroupMetadata.Owner = metav1.OwnerReference{
		APIVersion: lwsJob.GetAPIVersion(),
		Kind:       lwsJob.GetKind(),
		Name:       lwsJob.GetName(),
		UID:        lwsJob.GetUID(),
	}

	return podGroupMetadata, nil
}

func getLwsJobReplicas(lwsJob *unstructured.Unstructured) (int32, error) {
	lwsJobReplicas, found, err :=
		unstructured.NestedInt64(lwsJob.Object, "spec", "leaderWorkerTemplate", "spec")
	if !found || err != nil {
		return 0, fmt.Errorf("error retrieving LWS replicas count from spec: %w", err)
	}
	return int32(lwsJobReplicas), nil
}

func handleLeaderReadyPolicy(pod *v1.Pod, podGroupMetadata *podgroup.Metadata, fallbackSize int32) error {
	groupSize := fallbackSize
	if sizeStr, ok := pod.Annotations["leaderworkerset.sigs.k8s.io/size"]; ok {
		if parsed, err := strconv.Atoi(sizeStr); err == nil {
			groupSize = int32(parsed)
		}
	}

	// Leader has no group-index label
	_, hasGroupIndex := pod.Labels["leaderworkerset.sigs.k8s.io/group-index"]
	isLeader := !hasGroupIndex
	isScheduled := pod.Spec.NodeName != ""

	if isLeader && !isScheduled {
		podGroupMetadata.MinAvailable = 1
	} else {
		podGroupMetadata.MinAvailable = groupSize
	}

	return nil
}
