/*
Copyright 2017 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package resource_info

import (
	"fmt"
	"strconv"
	"strings"

	"golang.org/x/exp/maps"
	v1 "k8s.io/api/core/v1"

	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/common_info/resources"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/k8s_internal"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/log"
)

type Resource struct {
	BaseResource
	draGpuCounts map[v1.ResourceName]int64
	gpus         float64
}

func EmptyResource() *Resource {
	return &Resource{
		gpus:         0,
		draGpuCounts: make(map[v1.ResourceName]int64),
		BaseResource: *EmptyBaseResource(),
	}
}

func NewResource(milliCPU float64, memory float64, gpus float64) *Resource {
	return &Resource{
		gpus:         gpus,
		draGpuCounts: make(map[v1.ResourceName]int64),
		BaseResource: *NewBaseResourceWithValues(milliCPU, memory),
	}
}

func ResourceFromResourceList(rList v1.ResourceList) *Resource {
	r := EmptyResource()
	for rName, rQuant := range rList {
		switch rName {
		case v1.ResourceCPU:
			r.milliCpu += float64(rQuant.MilliValue())
		case v1.ResourceMemory:
			r.memory += float64(rQuant.Value())
		case GPUResourceName, amdGpuResourceName:
			r.gpus += float64(rQuant.Value())
		default:
			if IsMigResource(rName) {
				r.scalarResources[rName] += rQuant.Value()
			} else if rName == v1.ResourceEphemeralStorage || rName == v1.ResourceStorage {
				r.scalarResources[rName] += rQuant.Value()
			} else if k8s_internal.IsScalarResourceName(rName) {
				r.scalarResources[rName] += rQuant.MilliValue()
			}
		}
	}
	return r
}

func (r *Resource) Add(other *Resource) {
	r.BaseResource.Add(&other.BaseResource)
	r.gpus += other.gpus

	for rName, rQuant := range other.draGpuCounts {
		r.draGpuCounts[rName] += rQuant
	}
}

func (r *Resource) Sub(other *Resource) {
	r.BaseResource.Sub(&other.BaseResource)
	r.gpus -= other.gpus

	for rName, rQuant := range other.draGpuCounts {
		r.draGpuCounts[rName] -= rQuant
	}
}

func (r *Resource) Get(rn v1.ResourceName) float64 {
	switch rn {
	case GPUResourceName, amdGpuResourceName:
		return r.gpus
	default:
		return r.BaseResource.Get(rn)
	}
}

func (r *Resource) Clone() *Resource {
	return &Resource{
		gpus:         r.gpus,
		draGpuCounts: maps.Clone(r.draGpuCounts),
		BaseResource: *r.BaseResource.Clone(),
	}
}

func (r *Resource) LessEqual(rr *Resource) bool {
	if r.gpus > rr.gpus {
		return false
	}
	for rrName, rrQuant := range rr.draGpuCounts {
		if rrQuant <= r.draGpuCounts[rrName] {
			return false
		}
	}
	return r.BaseResource.LessEqual(&rr.BaseResource)
}

func (r *Resource) SetMaxResource(rr *Resource) {
	if r == nil || rr == nil {
		return
	}
	r.BaseResource.SetMaxResource(&rr.BaseResource)
	if rr.gpus > r.gpus {
		r.gpus = rr.gpus
	}
	for rrName, rrQuant := range rr.draGpuCounts {
		if rrQuant > r.draGpuCounts[rrName] {
			r.draGpuCounts[rrName] = rrQuant
		}
	}
}

func (r *Resource) String() string {
	return fmt.Sprintf(
		"CPU: %s (cores), memory: %s (GB), Gpus: %s",
		HumanizeResource(r.milliCpu, MilliCPUToCores),
		HumanizeResource(r.memory, MemoryToGB),
		HumanizeResource(r.gpus, 1),
	)
}

func (r *Resource) DetailedString() string {
	messageBuilder := strings.Builder{}

	messageBuilder.WriteString(r.String())

	for rName, rQuant := range r.scalarResources {
		messageBuilder.WriteString(fmt.Sprintf(", %s: %v", rName, rQuant))
	}
	for rName, rQuant := range r.draGpuCounts {
		messageBuilder.WriteString(fmt.Sprintf(", dra class %s: %v devices", rName, rQuant))
	}
	return messageBuilder.String()
}

func (r *Resource) AddResourceRequirements(req *ResourceRequirements) {
	if req == nil {
		return
	}
	r.BaseResource.Add(&req.BaseResource)
	r.gpus += req.GPUs()
	for rName, rQuant := range req.draGpuCounts {
		r.draGpuCounts[v1.ResourceName(rName)] += rQuant
	}
	for migProfile, migCount := range req.MigResources() {
		r.BaseResource.scalarResources[migProfile] += migCount
	}
}

func (r *Resource) SubResourceRequirements(req *ResourceRequirements) {
	r.BaseResource.Sub(&req.BaseResource)
	r.gpus -= req.GPUs()
	for rName, rQuant := range req.draGpuCounts {
		r.draGpuCounts[v1.ResourceName(rName)] -= rQuant
	}
	for migProfile, migCount := range req.MigResources() {
		r.BaseResource.scalarResources[migProfile] -= migCount
	}
}

func (r *Resource) GPUs() float64 {
	return r.gpus
}

func (r *Resource) ExtendedResourceGpusAsString() string {
	return strconv.FormatFloat(r.gpus, 'g', 3, 64)
}

func (r *Resource) GetGpusQuota() float64 {
	var totalGpusQuota float64
	for _, rQuant := range r.draGpuCounts {
		totalGpusQuota += float64(rQuant)
	}
	for resourceName, quant := range r.ScalarResources() {
		if !IsMigResource(resourceName) {
			continue
		}
		gpuPortion, _, err := resources.ExtractGpuAndMemoryFromMigResourceName(resourceName.String())
		if err != nil {
			log.InfraLogger.Errorf("Failed to get device portion from %v", resourceName)
			continue
		}

		totalGpusQuota += float64(gpuPortion) * float64(quant)
	}
	totalGpusQuota += r.gpus

	return totalGpusQuota
}

func (r *Resource) SetGPUs(gpus float64) {
	r.gpus = gpus
}

func (r *Resource) AddGPUs(addGpus float64) {
	r.gpus += addGpus
}

func (r *Resource) SubGPUs(subGpus float64) {
	r.gpus -= subGpus
}

func (r *Resource) DraGpuCounts() map[v1.ResourceName]int64 {
	return r.draGpuCounts
}

func (r *Resource) MigResources() map[v1.ResourceName]int64 {
	migResources := make(map[v1.ResourceName]int64)
	for name, quant := range r.scalarResources {
		if IsMigResource(name) {
			migResources[name] = quant
		}
	}
	return migResources
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
