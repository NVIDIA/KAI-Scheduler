// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package lws

import (
	"context"
	"fmt"
	"strconv"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/NVIDIA/KAI-scheduler/pkg/podgrouper/podgroup"
	"github.com/NVIDIA/KAI-scheduler/pkg/podgrouper/podgrouper/plugins/defaultgrouper"
)

const (
	// LWS labels
	lwsNameLabel        = "leaderworkerset.sigs.k8s.io/name"
	lwsGroupIndexLabel  = "leaderworkerset.sigs.k8s.io/group-index"
	lwsWorkerIndexLabel = "leaderworkerset.sigs.k8s.io/worker-index"

	// LWS startup policies
	leaderReadyStartupPolicy = "LeaderReady"
)

var (
	logger = log.FromContext(context.Background())
)

type LwsGrouper struct {
	*defaultgrouper.DefaultGrouper
}

func NewLwsGrouper(defaultGrouper *defaultgrouper.DefaultGrouper) *LwsGrouper {
	return &LwsGrouper{
		DefaultGrouper: defaultGrouper,
	}
}

func (lg *LwsGrouper) Name() string {
	return "LwsGrouper"
}

func (lg *LwsGrouper) GetPodGroupMetadata(
	lwsJob *unstructured.Unstructured, pod *v1.Pod, _ ...*metav1.PartialObjectMetadata,
) (*podgroup.Metadata, error) {
	podGroupMetadata, err := lg.DefaultGrouper.GetPodGroupMetadata(lwsJob, pod)
	if err != nil {
		return nil, err
	}

	// Modify the podGroup name to include the group index so each LWS replica gets its own podGroup
	groupIndex, err := lg.getGroupIndexFromPod(pod)
	if err != nil {
		return nil, err
	}
	podGroupMetadata.Name = fmt.Sprintf("%s-group-%d", podGroupMetadata.Name, groupIndex)

	// Get the size of one LeaderWorkerSet group (leader + workers)
	groupSize, err := lg.getLwsGroupSize(lwsJob)
	if err != nil {
		return nil, err
	}
	podGroupMetadata.MinAvailable = groupSize

	startupPolicy, found, err := unstructured.NestedString(lwsJob.Object, "spec", "startupPolicy")
	if err != nil {
		logger.V(1).Info("Failed to extract lws startup policy for %s/%s", lwsJob.GetName(), lwsJob.GetNamespace())
	}
	if found && startupPolicy == leaderReadyStartupPolicy {
		if err := lg.handleLeaderReadyStartupPolicy(pod, podGroupMetadata); err != nil {
			return podGroupMetadata, err
		}
	}

	return podGroupMetadata, nil
}

func (lg *LwsGrouper) getGroupIndexFromPod(pod *v1.Pod) (int, error) {
	groupIndexStr, found := pod.Labels[lwsGroupIndexLabel]
	if !found {
		return 0, fmt.Errorf("pod %s/%s does not have group index label %s", pod.Namespace, pod.Name, lwsGroupIndexLabel)
	}

	groupIndex, err := strconv.Atoi(groupIndexStr)
	if err != nil {
		return 0, fmt.Errorf("invalid group index %s for pod %s/%s: %w", groupIndexStr, pod.Namespace, pod.Name, err)
	}

	return groupIndex, nil
}

func (lg *LwsGrouper) getLwsGroupSize(lwsJob *unstructured.Unstructured) (int32, error) {
	size, found, err := unstructured.NestedInt64(lwsJob.Object, "spec", "leaderWorkerTemplate", "size")
	if err != nil {
		return 0, fmt.Errorf("failed to get leaderWorkerTemplate.size from LWS %s/%s with error: %w",
			lwsJob.GetNamespace(), lwsJob.GetName(), err)
	}
	if !found {
		return 0, fmt.Errorf("leaderWorkerTemplate.size not found in LWS %s/%s", lwsJob.GetNamespace(), lwsJob.GetName())
	}
	if size <= 0 {
		return 0, fmt.Errorf("invalid leaderWorkerTemplate.size %d in LWS %s/%s", size, lwsJob.GetNamespace(), lwsJob.GetName())
	}

	return int32(size), nil
}

func (lg *LwsGrouper) handleLeaderReadyStartupPolicy(
	pod *v1.Pod,
	podGroupMetadata *podgroup.Metadata,
) error {
	// If the startup policy is LeaderReady, and this pod isn't the leader
	// , then we know the leader is ready and the workers are been created (and should be scheduled)
	if !lg.isPodLeader(pod) {
		return nil
	}

	// If this pod is the leader and he is ready, then we know the leader is ready and the workers are been created (and should be scheduled)
	if lg.isPodReady(pod) && pod.GetDeletionTimestamp() == nil {
		return nil
	}

	// If the leader pod is not ready under the current policy (LeaderReady), we need to schedule the leader by himself
	podGroupMetadata.MinAvailable = 1

	return nil
}

func (lg *LwsGrouper) isPodLeader(pod *v1.Pod) bool {
	// In LWS, leader pods typically don't have the worker-index label or have worker-index=0
	workerIndexStr, hasWorkerIndex := pod.Labels[lwsWorkerIndexLabel]
	if !hasWorkerIndex {
		return true // No worker index means it's likely a leader
	}

	// If worker index is 0, it might be a leader (depending on LWS implementation)
	workerIndex, err := strconv.Atoi(workerIndexStr)
	if err != nil {
		return false
	}

	return workerIndex == 0
}

func (lg *LwsGrouper) isPodReady(pod *v1.Pod) bool {
	for _, condition := range pod.Status.Conditions {
		if condition.Type == v1.PodReady && condition.Status == v1.ConditionTrue {
			return true
		}
	}
	return false
}
