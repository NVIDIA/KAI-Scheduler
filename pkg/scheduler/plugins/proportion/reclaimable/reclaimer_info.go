// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package reclaimable

import (
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/common_info"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/resource_info"
)

type ReclaimerInfo struct {
	Name              string
	Namespace         string
	Queue             common_info.QueueID
	RequiredResources *resource_info.Resource
	IsPreemptable     bool
}
