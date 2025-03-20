// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package resource_info

import (
	"fmt"
	"strings"

	v1 "k8s.io/api/core/v1"

	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/common_info/resources"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/k8s_internal"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/log"
)

type Resource struct {
	BaseResource `json:"baseResource,omitempty"`
	GPUs         float64 `json:"gpus,omitempty"`
}

func EmptyResource() *Resource {
	return &Resource{
		GPUs:         0,
		BaseResource: *EmptyBaseResource(),
	}
}

func NewResource(milliCPU float64, memory float64, gpus float64) *Resource {
	return &Resource{
		GPUs:         gpus,
		BaseResource: *NewBaseResourceWithValues(milliCPU, memory),
	}
}

func ResourceFromResourceList(rList v1.ResourceList) *Resource {
	r := EmptyResource()
	for rName, rQuant := range rList {
		switch rName {
		case v1.ResourceCPU:
			r.CPUMilliCores += float64(rQuant.MilliValue())
		case v1.ResourceMemory:
			r.MemoryBytes += float64(rQuant.Value())
		case GPUResourceName, amdGpuResourceName:
			r.GPUs += float64(rQuant.Value())
		default:
			if IsMigResource(rName) {
				r.ScalarResources[rName] += rQuant.Value()
			} else if k8s_internal.IsScalarResourceName(rName) || rName == v1.ResourceEphemeralStorage || rName == v1.ResourceStorage {
				r.ScalarResources[rName] += rQuant.MilliValue()
			}
		}
	}
	return r
}

func (r *Resource) Add(other *Resource) {
	r.BaseResource.Add(&other.BaseResource)
	r.GPUs += other.GPUs
}

func (r *Resource) Sub(other *Resource) {
	r.BaseResource.Sub(&other.BaseResource)
	r.GPUs -= other.GPUs
}

func (r *Resource) Get(rn v1.ResourceName) float64 {
	switch rn {
	case GPUResourceName, amdGpuResourceName:
		return r.GPUs
	default:
		return r.BaseResource.Get(rn)
	}
}

func (r *Resource) Clone() *Resource {
	return &Resource{
		GPUs:         r.GPUs,
		BaseResource: *r.BaseResource.Clone(),
	}
}

func (r *Resource) LessEqual(rr *Resource) bool {
	if r.GPUs > rr.GPUs {
		return false
	}
	return r.BaseResource.LessEqual(&rr.BaseResource)
}

func (r *Resource) SetMaxResource(rr *Resource) {
	if r == nil || rr == nil {
		return
	}
	r.BaseResource.SetMaxResource(&rr.BaseResource)
	if rr.GPUs > r.GPUs {
		r.GPUs = rr.GPUs
	}
}

func (r *Resource) String() string {
	return fmt.Sprintf(
		"CPU: %s (cores), memory: %s (GB), Gpus: %s",
		HumanizeResource(r.CPUMilliCores, MilliCPUToCores),
		HumanizeResource(r.MemoryBytes, MemoryToGB),
		HumanizeResource(r.GPUs, 1),
	)
}

func (r *Resource) DetailedString() string {
	messageBuilder := strings.Builder{}

	messageBuilder.WriteString(r.String())

	for rName, rQuant := range r.ScalarResources {
		messageBuilder.WriteString(fmt.Sprintf(", %s: %v", rName, rQuant))
	}
	return messageBuilder.String()
}

func (r *Resource) AddResourceRequirements(req *ResourceRequirements) {
	r.BaseResource.Add(&req.BaseResource)
	r.GPUs += req.GPUs()
	for migProfile, migCount := range req.MIGResources {
		r.BaseResource.ScalarResources[migProfile] += migCount
	}
}

func (r *Resource) SubResourceRequirements(req *ResourceRequirements) {
	r.BaseResource.Sub(&req.BaseResource)
	r.GPUs -= req.GPUs()
	for migProfile, migCount := range req.MIGResources {
		r.BaseResource.ScalarResources[migProfile] -= migCount
	}
}

func (r *Resource) GetSumGPUs() float64 {
	var totalMigGPUs float64
	for resourceName, quant := range r.ScalarResources {
		if !IsMigResource(resourceName) {
			continue
		}
		gpuPortion, _, err := resources.ExtractGpuAndMemoryFromMigResourceName(resourceName.String())
		if err != nil {
			log.InfraLogger.Errorf("Failed to get device portion from %v", resourceName)
			continue
		}

		totalMigGPUs += float64(gpuPortion) * float64(quant)
	}

	return totalMigGPUs + r.GPUs
}

func (r *Resource) SetGPUs(gpus float64) {
	r.GPUs = gpus
}

func (r *Resource) AddGPUs(addGpus float64) {
	r.GPUs += addGpus
}

func (r *Resource) SubGPUs(subGpus float64) {
	r.GPUs -= subGpus
}

func StringResourceArray(ra []*Resource) string {
	if len(ra) == 0 {
		return ""
	}

	stringBuilder := strings.Builder{}
	stringBuilder.WriteString(ra[0].String())
	for _, r := range ra[1:] {
		stringBuilder.WriteString(",")
		stringBuilder.WriteString(r.String())
	}
	return stringBuilder.String()
}
