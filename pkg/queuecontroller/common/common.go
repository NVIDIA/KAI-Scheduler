// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package common

import v1 "k8s.io/api/core/v1"

const (
	ParentQueueIndexName = ".spec.parentQueue"
)

// IsActivePod checks if a pod is in an active state (Pending or Running).
func IsActivePod(pod *v1.Pod) bool {
	return pod.Status.Phase == v1.PodPending || pod.Status.Phase == v1.PodRunning
}

// IsAllocatedPod checks if a pod has been allocated resources.
// A pod is considered allocated if it's Running or if it's Pending but scheduled.
func IsAllocatedPod(pod *v1.Pod) bool {
	if pod.Status.Phase == v1.PodPending {
		return IsPodScheduled(pod)
	}
	return pod.Status.Phase == v1.PodRunning
}

// IsPodScheduled checks if a pod has been scheduled.
func IsPodScheduled(pod *v1.Pod) bool {
	for _, condition := range pod.Status.Conditions {
		if condition.Type == v1.PodScheduled {
			return condition.Status == v1.ConditionTrue
		}
	}
	return false
}
