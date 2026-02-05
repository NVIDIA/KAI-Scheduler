// Copyright 2026 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package utils

import (
	"github.com/NVIDIA/KAI-scheduler/pkg/podgrouper/podgrouper/plugins/constants"
	v1 "k8s.io/api/core/v1"
)

func IsAuxiliaryPod(pod *v1.Pod) bool {
	if pod == nil || pod.Annotations == nil {
		return false
	}
	_, isAux := pod.Annotations[constants.AuxiliaryPodAnnotationKey]
	return isAux
}
