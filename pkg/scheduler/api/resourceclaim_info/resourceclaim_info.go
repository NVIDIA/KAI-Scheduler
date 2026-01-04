// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package resourceclaim_info

import (
	"k8s.io/apimachinery/pkg/types"

	resourceapi "k8s.io/api/resource/v1"

	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/common_info"
)

type ResourceClaimInfo struct {
	UID       common_info.ResourceClaimID
	Name      string
	Namespace string
}

func NewResourceClaimInfo(claim *resourceapi.ResourceClaim) *ResourceClaimInfo {
	name := types.NamespacedName{
		Namespace: claim.Namespace,
		Name:      claim.Name,
	}
	return &ResourceClaimInfo{
		UID:       common_info.ResourceClaimID(name.String()),
		Namespace: claim.Namespace,
		Name:      claim.Name,
	}
}

