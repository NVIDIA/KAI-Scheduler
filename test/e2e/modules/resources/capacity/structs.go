/*
Copyright 2025 NVIDIA CORPORATION
SPDX-License-Identifier: Apache-2.0
*/
package capacity

import (
	"fmt"

	v1 "k8s.io/api/core/v1"

	"k8s.io/apimachinery/pkg/api/resource"
)

type ResourceList struct {
	Cpu            resource.Quantity
	Memory         resource.Quantity
	Gpu            resource.Quantity
	GpuMemory      resource.Quantity
	PodCount       int
	OtherResources map[v1.ResourceName]resource.Quantity
}

func initEmptyResourcesList() *ResourceList {
	return &ResourceList{
		Cpu:            resource.MustParse("0"),
		Memory:         resource.MustParse("0"),
		Gpu:            resource.MustParse("0"),
		GpuMemory:      resource.MustParse("0"),
		PodCount:       0,
		OtherResources: map[v1.ResourceName]resource.Quantity{},
	}
}

func FromK8sResourceList(list v1.ResourceList) *ResourceList {
	return &ResourceList{
		Cpu:       *list.Cpu(),
		Memory:    *list.Memory(),
		Gpu:       list[v1.ResourceName("nvidia.com/gpu")],
		GpuMemory: resource.Quantity{},
	}
}

func (rl *ResourceList) Add(toAddRl *ResourceList) {
	rl.Cpu.Add(toAddRl.Cpu)
	rl.Memory.Add(toAddRl.Memory)
	rl.Gpu.Add(toAddRl.Gpu)
	rl.GpuMemory.Add(toAddRl.GpuMemory)
	rl.PodCount += toAddRl.PodCount
	for resourceName, quantity := range toAddRl.OtherResources {
		if rl.OtherResources == nil {
			rl.OtherResources = map[v1.ResourceName]resource.Quantity{}
		}
		if _, exists := rl.OtherResources[resourceName]; !exists {
			rl.OtherResources[resourceName] = resource.MustParse("0")
		}
		res := rl.OtherResources[resourceName]
		res.Add(quantity)
		rl.OtherResources[resourceName] = res
	}
}

func (rl *ResourceList) Sub(toSubRl *ResourceList) {
	rl.Cpu.Sub(toSubRl.Cpu)
	rl.Memory.Sub(toSubRl.Memory)
	rl.Gpu.Sub(toSubRl.Gpu)
	rl.GpuMemory.Sub(toSubRl.GpuMemory)
	rl.PodCount -= toSubRl.PodCount
	for resourceName, quantity := range toSubRl.OtherResources {
		if rl.OtherResources == nil {
			rl.OtherResources = map[v1.ResourceName]resource.Quantity{}
		}
		if _, exists := rl.OtherResources[resourceName]; !exists {
			rl.OtherResources[resourceName] = resource.MustParse("0")
		}
		res := rl.OtherResources[resourceName]
		res.Sub(quantity)
		rl.OtherResources[resourceName] = res
	}
}

func (rl *ResourceList) LessOrEqual(toCmpRl *ResourceList) bool {
	if regularResource := !(rl.Cpu.Cmp(toCmpRl.Cpu) > 0) &&
		!(rl.Memory.Cmp(toCmpRl.Memory) > 0) &&
		!(rl.Gpu.Cmp(toCmpRl.Gpu) > 0) &&
		!(rl.GpuMemory.Cmp(toCmpRl.GpuMemory) > 0) &&
		!(rl.PodCount > toCmpRl.PodCount); !regularResource {
		return false
	}

	otherResources := true
	for resourceName, quantity := range rl.OtherResources {
		res := resource.MustParse("0")
		if _, exists := toCmpRl.OtherResources[resourceName]; exists {
			res = toCmpRl.OtherResources[resourceName]
		}
		otherResources = otherResources && !(quantity.Cmp(res) > 0)
	}
	return otherResources
}

func (rl ResourceList) String() string {
	return fmt.Sprintf("{Cpu: %v, Memory: %v, gpus: %v, GpuMemory: %v, PodCount: %d, OtherResources: %v}",
		rl.Cpu.String(), rl.Memory.String(), rl.Gpu.String(), rl.GpuMemory.String(), rl.PodCount, rl.OtherResources)
}
