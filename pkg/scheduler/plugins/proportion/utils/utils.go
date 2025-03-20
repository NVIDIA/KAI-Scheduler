// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package utils

import (
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/resource_info"
	rs "github.com/NVIDIA/KAI-scheduler/pkg/scheduler/plugins/proportion/resource_share"
)

func QuantifyResource(resource *resource_info.Resource) rs.ResourceQuantities {
	return rs.NewResourceQuantities(resource.CPUMilliCores, resource.MemoryBytes, resource.GetSumGPUs())
}

func QuantifyResourceRequirements(resource *resource_info.ResourceRequirements) rs.ResourceQuantities {
	return rs.NewResourceQuantities(resource.CPUMilliCores, resource.MemoryBytes, resource.GetSumGPUs())
}

func ResourceRequirementsFromQuantities(quantities rs.ResourceQuantities) *resource_info.ResourceRequirements {
	return resource_info.NewResourceRequirements(
		quantities[rs.GpuResource],
		quantities[rs.CpuResource],
		quantities[rs.MemoryResource],
	)
}
