// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package k8s_internal

import (
	v1 "k8s.io/api/core/v1"
	podutil "k8s.io/kubernetes/pkg/api/v1/pod"
	v1helper "k8s.io/kubernetes/pkg/apis/core/v1/helper"
)

func IsScalarResourceName(name v1.ResourceName) bool {
	return v1helper.IsExtendedResourceName(name) || v1helper.IsHugePageResourceName(name) ||
		v1helper.IsPrefixedNativeResource(name) || v1helper.IsAttachableVolumeResourceName(name)
}

func UpdatePodCondition(status *v1.PodStatus, condition *v1.PodCondition) bool {
	return podutil.UpdatePodCondition(status, condition)
}

func IsPodScheduled(pod *v1.Pod) bool {
	for _, condition := range pod.Status.Conditions {
		if condition.Type == v1.PodScheduled && condition.Status == v1.ConditionTrue {
			return true
		}
	}
	return false
}
