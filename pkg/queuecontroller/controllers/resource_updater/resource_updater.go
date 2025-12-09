// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package resource_updater

import (
	"context"
	"fmt"

	v1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	v2 "github.com/NVIDIA/KAI-scheduler/pkg/apis/scheduling/v2"
	"github.com/NVIDIA/KAI-scheduler/pkg/apis/scheduling/v2alpha2"
	"github.com/NVIDIA/KAI-scheduler/pkg/common/constants"
	"github.com/NVIDIA/KAI-scheduler/pkg/common/resources"
	"github.com/NVIDIA/KAI-scheduler/pkg/podgroupcontroller/controllers/cluster_relations"
	"github.com/NVIDIA/KAI-scheduler/pkg/queuecontroller/common"
)

type ResourceUpdater struct {
	client.Client
	QueueLabelKey string
}

func (ru *ResourceUpdater) UpdateQueue(ctx context.Context, queue *v2.Queue) error {
	queue.Status.Requested = v1.ResourceList{}
	queue.Status.Allocated = v1.ResourceList{}
	queue.Status.AllocatedNonPreemptible = v1.ResourceList{}

	err := ru.sumChildQueueResources(ctx, queue)
	if err != nil {
		return fmt.Errorf("failed to update queue resources status: %v", err)
	}

	err = ru.sumPodGroupsResources(ctx, queue)
	if err != nil {
		return fmt.Errorf("failed to update queue resources status: %v", err)
	}

	return nil
}

func (ru *ResourceUpdater) sumChildQueueResources(ctx context.Context, queue *v2.Queue) error {
	children := v2.QueueList{}
	err := ru.Client.List(ctx, &children, client.MatchingFields{common.ParentQueueIndexName: queue.Name})
	if err != nil {
		return err
	}

	for _, q := range children.Items {
		if q.Spec.ParentQueue != queue.Name {
			continue
		}

		queue.Status.Allocated = resources.SumResources(q.Status.Allocated, queue.Status.Allocated)
		queue.Status.AllocatedNonPreemptible = resources.SumResources(q.Status.AllocatedNonPreemptible, queue.Status.AllocatedNonPreemptible)
		queue.Status.Requested = resources.SumResources(q.Status.Requested, queue.Status.Requested)
	}
	return nil
}

func (ru *ResourceUpdater) sumPodGroupsResources(ctx context.Context, queue *v2.Queue) error {
	listOption := client.MatchingLabels{
		ru.QueueLabelKey: queue.Name,
	}

	queuePodGroups := v2alpha2.PodGroupList{}
	err := ru.Client.List(ctx, &queuePodGroups, listOption)
	if err != nil {
		return err
	}

	logger := log.FromContext(ctx)

	for _, pg := range queuePodGroups.Items {
		// Sum primitive resources from PodGroup status
		queue.Status.Allocated = resources.SumResources(pg.Status.ResourcesStatus.Allocated, queue.Status.Allocated)
		queue.Status.AllocatedNonPreemptible = resources.SumResources(pg.Status.ResourcesStatus.AllocatedNonPreemptible,
			queue.Status.AllocatedNonPreemptible)
		queue.Status.Requested = resources.SumResources(pg.Status.ResourcesStatus.Requested, queue.Status.Requested)

		// Extract DRA GPU resources from pods in this PodGroup
		draGPURequested, draGPUAllocated, err := ru.extractDRAGPUResourcesFromPodGroup(ctx, &pg)
		if err != nil {
			logger.V(1).Info("failed to extract DRA GPU resources from PodGroup",
				"podgroup", fmt.Sprintf("%s/%s", pg.Namespace, pg.Name),
				"error", err)
			// Continue processing other PodGroups even if one fails
			continue
		}

		// Add DRA GPU resources to queue totals
		queue.Status.Requested = resources.SumResources(draGPURequested, queue.Status.Requested)
		queue.Status.Allocated = resources.SumResources(draGPUAllocated, queue.Status.Allocated)

		// For non-preemptible PodGroups, also add DRA GPU allocated resources to AllocatedNonPreemptible
		if pg.Spec.Preemptibility == v2alpha2.NonPreemptible { // ???
			queue.Status.AllocatedNonPreemptible = resources.SumResources(draGPUAllocated, queue.Status.AllocatedNonPreemptible)
		}
	}

	return nil
}

// extractDRAGPUResourcesFromPodGroup extracts DRA GPU resources from all pods in a PodGroup.
// Returns requested and allocated DRA GPU resources separately.
func (ru *ResourceUpdater) extractDRAGPUResourcesFromPodGroup(ctx context.Context, podGroup *v2alpha2.PodGroup) (
	v1.ResourceList, v1.ResourceList, error) {

	requested := v1.ResourceList{}
	allocated := v1.ResourceList{}

	// List pods belonging to this PodGroup
	pods, err := ru.listPodsForPodGroup(ctx, podGroup)
	if err != nil {
		return requested, allocated, fmt.Errorf("failed to list pods for PodGroup %s/%s: %v",
			podGroup.Namespace, podGroup.Name, err)
	}

	for i := range pods.Items {
		pod := &pods.Items[i]

		// Extract DRA GPU resources for requested (all active pods)
		if isActivePod(pod) {
			draGPURequested, err := resources.ExtractDRAGPUResources(ctx, pod, ru.Client)
			if err != nil {
				// Log but continue processing other pods
				logger := log.FromContext(ctx)
				logger.V(1).Info("failed to extract DRA GPU requested resources from pod",
					"pod", fmt.Sprintf("%s/%s", pod.Namespace, pod.Name),
					"error", err)
			} else {
				requested = resources.SumResources(requested, draGPURequested)
			}
		}

		// Extract DRA GPU resources for allocated (only allocated pods)
		if isAllocatedPod(pod) {
			draGPUAllocated, err := resources.ExtractDRAGPUResources(ctx, pod, ru.Client)
			if err != nil {
				// Log but continue processing other pods
				logger := log.FromContext(ctx)
				logger.V(1).Info("failed to extract DRA GPU allocated resources from pod",
					"pod", fmt.Sprintf("%s/%s", pod.Namespace, pod.Name),
					"error", err)
			} else {
				allocated = resources.SumResources(allocated, draGPUAllocated)
			}
		}
	}

	return requested, allocated, nil
}

// listPodsForPodGroup lists all pods belonging to a PodGroup.
// Uses the PodGroupToPodsIndexer if available, otherwise falls back to annotation-based filtering.
func (ru *ResourceUpdater) listPodsForPodGroup(ctx context.Context, podGroup *v2alpha2.PodGroup) (
	v1.PodList, error) {

	// Try to use the existing helper function first
	podList, err := cluster_relations.GetAllPodsOfPodGroup(ctx, podGroup, ru.Client)
	if err == nil {
		return podList, nil
	}

	// Fallback: if indexer is not available, manually filter pods by annotation
	podList = v1.PodList{}
	allPods := v1.PodList{}
	err = ru.Client.List(ctx, &allPods, client.InNamespace(podGroup.Namespace))
	if err != nil {
		return podList, fmt.Errorf("failed to list pods in namespace %s: %v", podGroup.Namespace, err)
	}

	// Filter pods that have the pod-group-name annotation matching this PodGroup
	for i := range allPods.Items {
		pod := &allPods.Items[i]
		if pod.Annotations != nil {
			if pgName, found := pod.Annotations[constants.PodGroupAnnotationForPod]; found && pgName == podGroup.Name {
				podList.Items = append(podList.Items, *pod)
			}
		}
	}

	return podList, nil
}

// isActivePod checks if a pod is in an active state (Pending or Running).
func isActivePod(pod *v1.Pod) bool { // TODO: is definitly implemented somewhere else
	return pod.Status.Phase == v1.PodPending || pod.Status.Phase == v1.PodRunning
}

// isAllocatedPod checks if a pod has been allocated resources.
// A pod is considered allocated if it's Running or if it's Pending but scheduled.
func isAllocatedPod(pod *v1.Pod) bool {
	if pod.Status.Phase == v1.PodPending {
		return isPodScheduled(pod)
	}
	return pod.Status.Phase == v1.PodRunning
}

// isPodScheduled checks if a pod has been scheduled.
func isPodScheduled(pod *v1.Pod) bool {
	for _, condition := range pod.Status.Conditions {
		if condition.Type == v1.PodScheduled {
			return condition.Status == v1.ConditionTrue
		}
	}
	return false
}
