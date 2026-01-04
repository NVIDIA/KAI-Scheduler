// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package resourceslice_info

import (
	resourceapi "k8s.io/api/resource/v1"

	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/common_info"
)

type ResourceSliceInfo struct {
	UID  common_info.ResourceSliceID
	Name string
}

func NewResourceSliceInfo(slice *resourceapi.ResourceSlice) *ResourceSliceInfo {
	return &ResourceSliceInfo{
		UID:  common_info.ResourceSliceID(slice.UID),
		Name: slice.Name,
	}
}

