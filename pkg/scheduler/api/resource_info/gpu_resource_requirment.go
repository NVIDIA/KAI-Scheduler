// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package resource_info

import (
	"fmt"
	"math"
	"strings"

	"golang.org/x/exp/maps"
	v1 "k8s.io/api/core/v1"

	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/common_info/resources"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/log"
)

const (
	minGPUs                             = 0.01
	gpuPortionsAsDecimalsRoundingFactor = 100
	wholeGpuPortion                     = 1
	fractionDefaultCount                = 1
)

type GpuResourceRequirement struct {
	Count        int64                     `json:"count,omitempty"`
	Portion      float64                   `json:"portion,omitempty"`
	GPUMemory    int64                     `json:"gpuMemory,omitempty"`
	MIGResources map[v1.ResourceName]int64 `json:"migResources,omitempty"`
}

func NewGpuResourceRequirement() *GpuResourceRequirement {
	return &GpuResourceRequirement{
		Count:        0,
		Portion:      0,
		GPUMemory:    0,
		MIGResources: make(map[v1.ResourceName]int64),
	}
}

func NewGpuResourceRequirementWithGpus(gpus float64, gpuMemory int64) *GpuResourceRequirement {
	gResource := &GpuResourceRequirement{
		Count:        fractionDefaultCount,
		Portion:      gpus,
		GPUMemory:    gpuMemory,
		MIGResources: make(map[v1.ResourceName]int64),
	}
	if gpus >= wholeGpuPortion {
		gResource.Count = int64(gpus)
		gResource.Portion = wholeGpuPortion
	}
	return gResource
}

func NewGpuResourceRequirementWithMultiFraction(count int64, portion float64, gpuMemory int64) *GpuResourceRequirement {
	gResource := &GpuResourceRequirement{
		Count:        count,
		Portion:      portion,
		GPUMemory:    gpuMemory,
		MIGResources: make(map[v1.ResourceName]int64),
	}
	return gResource
}

func NewGpuResourceRequirementWithMig(migResources map[v1.ResourceName]int64) *GpuResourceRequirement {
	return &GpuResourceRequirement{
		Count:        0,
		Portion:      0,
		GPUMemory:    0,
		MIGResources: migResources,
	}
}

func (g *GpuResourceRequirement) IsEmpty() bool {
	if getNonMigGpus(g.Portion, g.Count) > minGPUs {
		return false
	}
	for _, rQuant := range g.MIGResources {
		if rQuant > 0 {
			return false
		}
	}
	return true
}

func (g *GpuResourceRequirement) Clone() *GpuResourceRequirement {
	return &GpuResourceRequirement{
		Count:        g.Count,
		Portion:      g.Portion,
		GPUMemory:    g.GPUMemory,
		MIGResources: maps.Clone(g.MIGResources),
	}
}

func (g *GpuResourceRequirement) SetMaxResource(gg *GpuResourceRequirement) error {
	if g.Portion != 0 && gg.Portion != 0 && g.Portion != gg.Portion {
		return fmt.Errorf("cannot calculate max resource for GpuResourceRequirements with different fractional portions. %v vs %v", g.Portion, gg.Portion)
	}
	if getNonMigGpus(gg.Portion, gg.Count) > getNonMigGpus(g.Portion, g.Count) {
		g.Count = gg.Count
		g.Portion = gg.Portion
	}

	for name, ggQuant := range gg.MIGResources {
		if gQuant, found := g.MIGResources[name]; !found || ggQuant > gQuant {
			g.MIGResources[name] = ggQuant
		}
	}
	return nil
}

func (g *GpuResourceRequirement) LessEqual(gg *GpuResourceRequirement) bool {
	if g.Count > gg.Count {
		return false
	}
	if !lessEqualWithMinDiff(g.Portion, gg.Portion, minGPUs) {
		return false
	}

	for name, gQuant := range g.MIGResources {
		ggQuant, found := gg.MIGResources[name]
		if !found || gQuant > ggQuant {
			return false
		}
	}
	return true
}

func (g *GpuResourceRequirement) GetSumGPUs() float64 {
	var totalMigGPUs float64
	for migResource, quant := range g.MIGResources {
		gpuPortion, _, err := resources.ExtractGpuAndMemoryFromMigResourceName(migResource.String())
		if err != nil {
			log.InfraLogger.Errorf("Failed to get device portion from %v", migResource)
			continue
		}

		totalMigGPUs += float64(gpuPortion) * float64(quant)
	}

	return totalMigGPUs + getNonMigGpus(g.Portion, g.Count)
}

func (g *GpuResourceRequirement) ClearMigResources() {
	g.MIGResources = map[v1.ResourceName]int64{}
}

func (g *GpuResourceRequirement) GpuMemory() int64 {
	return g.GPUMemory
}

func (g *GpuResourceRequirement) GPUs() float64 {
	return getNonMigGpus(g.Portion, g.Count)
}

func (g *GpuResourceRequirement) GetNumOfGpuDevices() int64 {
	return g.Count
}

func (g *GpuResourceRequirement) GpuFractionalPortion() float64 {
	return g.Portion
}

func (g *GpuResourceRequirement) IsFractionalRequest() bool {
	if g.GPUMemory > 0 {
		return true
	}
	return g.Count > 0 && g.Portion < wholeGpuPortion
}

func getNonMigGpus(portion float64, count int64) float64 {
	// use fixed-point arithmetic to avoid floating point errors
	portionAsDecimals := int64(math.Round(portion * gpuPortionsAsDecimalsRoundingFactor))
	return float64(portionAsDecimals*count) / gpuPortionsAsDecimalsRoundingFactor
}

func IsMigResource(rName v1.ResourceName) bool {
	return strings.HasPrefix(string(rName), "nvidia.com/mig-")
}

func lessEqualWithMinDiff(l, r, minDiff float64) bool {
	return l < r || math.Abs(l-r) < minDiff
}
