// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package resource_share

import (
	"math"

	commonconstants "github.com/NVIDIA/KAI-scheduler/pkg/common/constants"
)

type ResourceShare struct {
	Deserved                float64
	FairShare               float64
	MaxAllowed              float64
	OverQuotaWeight         float64
	Allocated               float64
	AllocatedNotPreemptible float64
	Request                 float64
	AbsoluteUsage           float64
	VacantAdjustedUsage     float64
}

func EmptyResource() ResourceShare {
	return ResourceShare{}
}

func (rs *ResourceShare) Clone() *ResourceShare {
	return &ResourceShare{
		Deserved:                rs.Deserved,
		FairShare:               rs.FairShare,
		MaxAllowed:              rs.MaxAllowed,
		OverQuotaWeight:         rs.OverQuotaWeight,
		Allocated:               rs.Allocated,
		AllocatedNotPreemptible: rs.AllocatedNotPreemptible,
		Request:                 rs.Request,
		AbsoluteUsage:           rs.AbsoluteUsage,
		VacantAdjustedUsage:     rs.VacantAdjustedUsage,
	}
}

func (rs *ResourceShare) GetRequestableShare() float64 {
	if rs.MaxAllowed == commonconstants.UnlimitedResourceQuantity {
		return rs.Request
	}
	return math.Min(rs.MaxAllowed, rs.Request)
}

func (rs *ResourceShare) GetRequestShare() float64 {
	return rs.Request
}

func (rs *ResourceShare) GetAllocatableShare() float64 {
	if rs.Deserved == commonconstants.UnlimitedResourceQuantity {
		return rs.MaxAllowed
	}

	allocatable := math.Max(rs.Deserved, rs.FairShare)
	if rs.MaxAllowed != commonconstants.UnlimitedResourceQuantity {
		allocatable = math.Min(rs.MaxAllowed, allocatable)
	}
	return allocatable
}

func (rs *ResourceShare) GetAbsoluteUsage() float64 {
	return rs.AbsoluteUsage
}

func (rs *ResourceShare) GetVacantAdjustedUsage() float64 {
	return rs.VacantAdjustedUsage
}
