// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package resourceclaimtemplate_info

import (
	"k8s.io/apimachinery/pkg/types"

	resourceapi "k8s.io/api/resource/v1"

	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/common_info"
)

type ResourceClaimTemplateInfo struct {
	UID       common_info.ResourceClaimTemplateID
	Name      string
	Namespace string
}

func NewResourceClaimTemplateInfo(template *resourceapi.ResourceClaimTemplate) *ResourceClaimTemplateInfo {
	name := types.NamespacedName{
		Namespace: template.Namespace,
		Name:      template.Name,
	}
	return &ResourceClaimTemplateInfo{
		UID:       common_info.ResourceClaimTemplateID(name.String()),
		Namespace: template.Namespace,
		Name:      template.Name,
	}
}

