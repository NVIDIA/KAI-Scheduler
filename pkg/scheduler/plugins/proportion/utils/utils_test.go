// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package utils

import (
	"testing"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"

	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/resource_info"
	rs "github.com/NVIDIA/KAI-scheduler/pkg/scheduler/plugins/proportion/resource_share"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestUtils(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Proportion Utils suite")
}

var _ = Describe("QuantifyResource", func() {
	It("should count MIG resources as GPUs", func() {
		migResource := resource_info.ResourceFromResourceList(v1.ResourceList{
			v1.ResourceCPU:    resource.MustParse("2"),
			v1.ResourceMemory: resource.MustParse("4Gi"),
			v1.ResourceName("nvidia.com/mig-3g.20gb"): resource.MustParse("1"),
		})

		quantities := QuantifyResource(migResource)
		Expect(quantities[rs.GpuResource]).To(Equal(float64(3)))
		Expect(quantities[rs.CpuResource]).To(Equal(float64(2000))) // 2 cores = 2000 milliCPU
	})

	It("should count regular GPUs", func() {
		gpuResource := resource_info.NewResource(1000, 1024*1024*1024, 2)

		quantities := QuantifyResource(gpuResource)
		Expect(quantities[rs.GpuResource]).To(Equal(float64(2)))
	})

	It("should count mixed MIG and regular GPUs", func() {
		mixedResource := resource_info.ResourceFromResourceList(v1.ResourceList{
			v1.ResourceCPU:                           resource.MustParse("1"),
			v1.ResourceMemory:                        resource.MustParse("1Gi"),
			v1.ResourceName("nvidia.com/gpu"):        resource.MustParse("1"),
			v1.ResourceName("nvidia.com/mig-1g.5gb"): resource.MustParse("2"),
		})

		quantities := QuantifyResource(mixedResource)
		Expect(quantities[rs.GpuResource]).To(Equal(float64(3))) // 1 whole + 2*1g
	})

	It("should return zero GPU for CPU/memory-only resources", func() {
		cpuResource := resource_info.NewResource(1000, 1024*1024*1024, 0)

		quantities := QuantifyResource(cpuResource)
		Expect(quantities[rs.GpuResource]).To(Equal(float64(0)))
	})
})
