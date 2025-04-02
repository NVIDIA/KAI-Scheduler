// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package resource_info

import (
	"fmt"
	"strings"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"

	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/k8s_internal"
)

const (
	GPUResourceName    = "nvidia.com/gpu"
	amdGpuResourceName = "amd.com/gpu"
)

type ResourceRequirements struct {
	BaseResource
	GpuResourceRequirement
}

func EmptyResourceRequirements() *ResourceRequirements {
	return &ResourceRequirements{
		GpuResourceRequirement: *NewGpuResourceRequirement(),
		BaseResource:           *EmptyBaseResource(),
	}
}

func NewResourceRequirements(gpus, milliCpus, memory float64) *ResourceRequirements {
	return &ResourceRequirements{
		GpuResourceRequirement: *NewGpuResourceRequirementWithGpus(gpus, 0),
		BaseResource:           *NewBaseResourceWithValues(milliCpus, memory),
	}
}

func NewResourceRequirementsWithGpus(gpus float64) *ResourceRequirements {
	return NewResourceRequirements(gpus, 0, 0)
}

func RequirementsFromResourceList(rl v1.ResourceList) *ResourceRequirements {
	r := EmptyResourceRequirements()
	for rName, rQuant := range rl {
		switch rName {
		case v1.ResourceCPU:
			r.milliCpu += float64(rQuant.MilliValue())
		case v1.ResourceMemory:
			r.memory += float64(rQuant.Value())
		case GPUResourceName, amdGpuResourceName:
			if rQuant.Value() >= wholeGpuPortion {
				r.count += rQuant.Value()
				r.portion = wholeGpuPortion
			} else {
				r.portion += float64(rQuant.Value())
				r.count = fractionDefaultCount
			}
		default:
			if IsMigResource(rName) {
				r.MigResources()[rName] += rQuant.Value()
			} else if k8s_internal.IsScalarResourceName(rName) || rName == v1.ResourceEphemeralStorage || rName == v1.ResourceStorage {
				r.scalarResources[rName] += rQuant.MilliValue()
			}
		}
	}
	return r
}

func (r *ResourceRequirements) ToResourceList() v1.ResourceList {
	rl := r.BaseResource.ToResourceList()

	rl[GPUResourceName] = *resource.NewQuantity(int64(r.GPUs()), resource.DecimalSI)
	for rName, rQuant := range r.MigResources() {
		rl[rName] = *resource.NewQuantity(rQuant, resource.DecimalSI)
	}

	return rl
}

func (r *ResourceRequirements) Clone() *ResourceRequirements {
	return &ResourceRequirements{
		GpuResourceRequirement: *r.GpuResourceRequirement.Clone(),
		BaseResource:           *r.BaseResource.Clone(),
	}
}

func (r *ResourceRequirements) IsEmpty() bool {
	if !r.GpuResourceRequirement.IsEmpty() {
		return false
	}
	return r.BaseResource.IsEmpty()
}

func (r *ResourceRequirements) SetMaxResource(rr *ResourceRequirements) error {
	if r == nil || rr == nil {
		return nil
	}
	r.BaseResource.SetMaxResource(&rr.BaseResource)
	return r.GpuResourceRequirement.SetMaxResource(&rr.GpuResourceRequirement)
}

func (r *ResourceRequirements) LessInAtLeastOneResource(rr *ResourceRequirements) bool {
	return !rr.LessEqual(r)
}

func (r *ResourceRequirements) LessEqual(rr *ResourceRequirements) bool {
	if !r.BaseResource.LessEqual(&rr.BaseResource) {
		return false
	}
	if !r.GpuResourceRequirement.LessEqual(&rr.GpuResourceRequirement) {
		return false
	}
	return true
}

func (r *ResourceRequirements) LessEqualResource(rr *Resource) bool {
	if !r.BaseResource.LessEqual(&rr.BaseResource) {
		return false
	}
	if r.GpuResourceRequirement.GPUs() > rr.GPUs() {
		return false
	}
	for migProfile, migRequirementCount := range r.MigResources() {
		migProfileCountRR, migExistsForRR := rr.scalarResources[migProfile]
		if !migExistsForRR || migRequirementCount > migProfileCountRR {
			return false
		}
	}
	return true
}

func (r *ResourceRequirements) String() string {
	return fmt.Sprintf(
		"GPU: %s, CPU: %s (cores), memory: %s (GB)",
		HumanizeResource(r.GetSumGPUs(), 1),
		HumanizeResource(r.milliCpu, MilliCPUToCores),
		HumanizeResource(r.memory, MemoryToGB),
	)
}

func (r *ResourceRequirements) DetailedString() string {
	messageBuilder := strings.Builder{}

	messageBuilder.WriteString(r.String())

	for rName, rQuant := range r.scalarResources {
		messageBuilder.WriteString(fmt.Sprintf(", %s: %v", rName, rQuant))
	}
	for migName, migQuant := range r.MigResources() {
		messageBuilder.WriteString(fmt.Sprintf(", mig %s: %d", migName, migQuant))
	}
	return messageBuilder.String()
}

func (r *ResourceRequirements) Get(rn v1.ResourceName) float64 {
	switch rn {
	case GPUResourceName, amdGpuResourceName:
		return r.GPUs()
	default:
		if IsMigResource(rn) {
			if _, found := r.MigResources()[rn]; !found {
				return 0
			}
			return float64(r.MigResources()[rn])
		}
		return r.BaseResource.Get(rn)
	}
}
