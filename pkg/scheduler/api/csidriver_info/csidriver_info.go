// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package csidriver_info

import "github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/common_info"

type CSIDriverInfo struct {
	ID              common_info.CSIDriverID `json:"id,omitempty"`
	CapacityEnabled bool                    `json:"capacityEnabled,omitempty"`
}
