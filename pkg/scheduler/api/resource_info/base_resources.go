// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package resource_info

import (
	"github.com/dustin/go-humanize"
	"golang.org/x/exp/maps"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"

	commonconstants "github.com/NVIDIA/KAI-scheduler/pkg/common/constants"
)

const (
	minMilliCPU             float64 = 10
	minMilliScalarResources int64   = 10
	MinMemory               float64 = 10 * 1024 * 1024
	MilliCPUToCores         float64 = 1000
	MemoryToGB              float64 = 1000 * 1000 * 1000
)

type BaseResource struct {
	CPUMilliCores   float64                   `json:"milliCpu,omitempty"`
	MemoryBytes     float64                   `json:"memory,omitempty"`
	ScalarResources map[v1.ResourceName]int64 `json:"scalarResources,omitempty"`
}

func EmptyBaseResource() *BaseResource {
	return &BaseResource{
		CPUMilliCores:   0,
		MemoryBytes:     0,
		ScalarResources: make(map[v1.ResourceName]int64),
	}
}

func NewBaseResourceWithValues(milliCPU float64, memory float64) *BaseResource {
	return &BaseResource{
		CPUMilliCores:   milliCPU,
		MemoryBytes:     memory,
		ScalarResources: make(map[v1.ResourceName]int64),
	}
}

func (r *BaseResource) Clone() *BaseResource {
	return &BaseResource{
		CPUMilliCores:   r.CPUMilliCores,
		MemoryBytes:     r.MemoryBytes,
		ScalarResources: maps.Clone(r.ScalarResources),
	}
}

func (r *BaseResource) Add(other *BaseResource) {
	r.CPUMilliCores += other.CPUMilliCores
	r.MemoryBytes += other.MemoryBytes
	for rName, rrValue := range other.ScalarResources {
		r.ScalarResources[rName] += rrValue
	}
}

func (r *BaseResource) Sub(other *BaseResource) {
	r.CPUMilliCores -= other.CPUMilliCores
	r.MemoryBytes -= other.MemoryBytes
	for rName, rrValue := range other.ScalarResources {
		r.ScalarResources[rName] -= rrValue
	}
}

func (r *BaseResource) Get(rn v1.ResourceName) float64 {
	switch rn {
	case v1.ResourceCPU:
		return r.CPUMilliCores
	case v1.ResourceMemory:
		return r.MemoryBytes
	default:
		rQuan, found := r.ScalarResources[rn]
		if !found {
			return 0
		}
		return float64(rQuan)
	}
}

func (r *BaseResource) LessEqual(rr *BaseResource) bool {
	if r.CPUMilliCores > rr.CPUMilliCores {
		return false
	}
	if r.MemoryBytes > rr.MemoryBytes {
		return false
	}
	for rName, rQuant := range r.ScalarResources {
		rrQuant, found := rr.ScalarResources[rName]
		if !found || rQuant > rrQuant {
			return false
		}
	}

	return true
}

func (r *BaseResource) ToResourceList() v1.ResourceList {
	rl := v1.ResourceList{}

	rl[v1.ResourceCPU] = *resource.NewMilliQuantity(int64(r.CPUMilliCores), resource.DecimalSI)
	rl[v1.ResourceMemory] = *resource.NewQuantity(int64(r.MemoryBytes), resource.DecimalSI)
	for rName, rQuant := range r.ScalarResources {
		rl[rName] = *resource.NewMilliQuantity(int64(rQuant), resource.DecimalSI)
	}

	return rl
}

func (r *BaseResource) IsEmpty() bool {
	if r.CPUMilliCores >= minMilliCPU || r.MemoryBytes >= MinMemory {
		return false
	}
	for _, rQuant := range r.ScalarResources {
		if rQuant >= minMilliScalarResources {
			return false
		}
	}

	return true
}

func (r *BaseResource) SetMaxResource(rr *BaseResource) {
	if r == nil || rr == nil {
		return
	}

	if rr.CPUMilliCores > r.CPUMilliCores {
		r.CPUMilliCores = rr.CPUMilliCores
	}
	if rr.MemoryBytes > r.MemoryBytes {
		r.MemoryBytes = rr.MemoryBytes
	}

	if r.ScalarResources == nil {
		r.ScalarResources = make(map[v1.ResourceName]int64)
	}
	for rrName, rrQuant := range rr.ScalarResources {
		if rQuant, found := r.ScalarResources[rrName]; !found || rrQuant > rQuant {
			r.ScalarResources[rrName] = rrQuant
		}
	}
}

func HumanizeResource(value float64, unitAdjustment float64) string {
	if value == commonconstants.UnlimitedResourceQuantity {
		return "Unlimited"
	}
	return humanize.FtoaWithDigits(value/unitAdjustment, 3)
}
