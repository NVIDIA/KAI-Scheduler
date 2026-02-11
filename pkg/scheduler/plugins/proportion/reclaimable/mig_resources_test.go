// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package reclaimable

import (
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"

	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/resource_info"
	rs "github.com/NVIDIA/KAI-scheduler/pkg/scheduler/plugins/proportion/resource_share"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("MIG resource handling in preemption", func() {
	Context("getInvolvedResourcesNames", func() {
		It("should detect GPU involvement for MIG-only resources", func() {
			// Build a Resource with MIG scalar resources but gpus=0
			migResource := resource_info.ResourceFromResourceList(v1.ResourceList{
				v1.ResourceCPU:    resource.MustParse("1"),
				v1.ResourceMemory: resource.MustParse("1Gi"),
				v1.ResourceName("nvidia.com/mig-3g.20gb"): resource.MustParse("1"),
			})

			// gpus field should be 0 (MIG is stored in scalarResources)
			Expect(migResource.GPUs()).To(Equal(float64(0)))

			// But GetSumGPUs should return non-zero (3 GPU portions for mig-3g)
			Expect(migResource.GetSumGPUs()).To(Equal(float64(3)))

			involved := getInvolvedResourcesNames([]*resource_info.Resource{migResource})
			Expect(involved).To(HaveKey(rs.GpuResource))
			Expect(involved).To(HaveKey(rs.CpuResource))
			Expect(involved).To(HaveKey(rs.MemoryResource))
		})

		It("should detect GPU involvement for regular GPU resources", func() {
			gpuResource := resource_info.NewResource(1000, 1024*1024*1024, 2)

			involved := getInvolvedResourcesNames([]*resource_info.Resource{gpuResource})
			Expect(involved).To(HaveKey(rs.GpuResource))
			Expect(involved).To(HaveKey(rs.CpuResource))
			Expect(involved).To(HaveKey(rs.MemoryResource))
		})

		It("should detect GPU involvement for mixed MIG and regular GPU resources", func() {
			mixedResource := resource_info.ResourceFromResourceList(v1.ResourceList{
				v1.ResourceCPU:                           resource.MustParse("1"),
				v1.ResourceMemory:                        resource.MustParse("1Gi"),
				v1.ResourceName("nvidia.com/gpu"):        resource.MustParse("1"),
				v1.ResourceName("nvidia.com/mig-1g.5gb"): resource.MustParse("2"),
			})

			Expect(mixedResource.GPUs()).To(Equal(float64(1)))
			Expect(mixedResource.GetSumGPUs()).To(Equal(float64(3))) // 1 whole + 2*1g

			involved := getInvolvedResourcesNames([]*resource_info.Resource{mixedResource})
			Expect(involved).To(HaveKey(rs.GpuResource))
		})

		It("should not include GPU for CPU/memory-only resources", func() {
			cpuOnlyResource := resource_info.NewResource(1000, 1024*1024*1024, 0)

			involved := getInvolvedResourcesNames([]*resource_info.Resource{cpuOnlyResource})
			Expect(involved).NotTo(HaveKey(rs.GpuResource))
			Expect(involved).To(HaveKey(rs.CpuResource))
			Expect(involved).To(HaveKey(rs.MemoryResource))
		})
	})
})
