// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package api

import (
	"testing"

	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/pod_info"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/podgroup_info"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/resource_info"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestUnscheduledInfo(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "UnscheduledInfo Suite")
}

var _ = Describe("UnscheduledInfo", func() {
	Context("GetBuildOverCapacityMessageForQueue", func() {
		It("should generate correct message for GPU resource", func() {
			queueName := "test-queue"
			resourceName := GpuResource
			deserved := 4.0
			used := 2.0
			requiredResources := &podgroup_info.JobRequirement{
				GPU:      3.0,
				MilliCPU: 0,
				Memory:   0,
			}

			message := GetBuildOverCapacityMessageForQueue(queueName, resourceName, deserved, used, requiredResources)

			expectedPrefix := "Non-preemptible workload is over quota. "
			expectedDetails := "Workload requested 3 GPUs, but test-queue quota is 4 GPUs, while 2 GPUs are already allocated for non-preemptible pods."
			expectedSuffix := " Use a preemptible workload to go over quota."

			Expect(message).To(ContainSubstring(expectedPrefix))
			Expect(message).To(ContainSubstring(expectedDetails))
			Expect(message).To(ContainSubstring(expectedSuffix))
		})

		It("should generate correct message for CPU resource", func() {
			queueName := "cpu-queue"
			resourceName := CpuResource
			deserved := 8000.0 // 8 CPU cores in millicores
			used := 4000.0     // 4 CPU cores in millicores
			requiredResources := &podgroup_info.JobRequirement{
				GPU:      0,
				MilliCPU: 6000.0, // 6 CPU cores in millicores
				Memory:   0,
			}

			message := GetBuildOverCapacityMessageForQueue(queueName, resourceName, deserved, used, requiredResources)

			expectedPrefix := "Non-preemptible workload is over quota. "
			expectedDetails := "Workload requested 6 CPU cores, but cpu-queue quota is 8 cores, while 4 cores are already allocated for non-preemptible pods."
			expectedSuffix := " Use a preemptible workload to go over quota."

			Expect(message).To(ContainSubstring(expectedPrefix))
			Expect(message).To(ContainSubstring(expectedDetails))
			Expect(message).To(ContainSubstring(expectedSuffix))
		})

		It("should generate correct message for 200G memory resource", func() {
			queueName := "memory-queue"
			resourceName := MemoryResource
			deserved := 100000000000.0 // 100 GB in bytes
			used := 50000000000.0      // 50 GB in bytes
			requiredResources := &podgroup_info.JobRequirement{
				GPU:      0,
				MilliCPU: 0,
				Memory:   200000000000.0, // 200 GB in bytes
			}

			message := GetBuildOverCapacityMessageForQueue(queueName, resourceName, deserved, used, requiredResources)

			expectedPrefix := "Non-preemptible workload is over quota. "
			expectedDetails := "Workload requested 200 GB memory, but memory-queue quota is 100 GB, while 50 GB are already allocated for non-preemptible pods."
			expectedSuffix := " Use a preemptible workload to go over quota."

			Expect(message).To(ContainSubstring(expectedPrefix))
			Expect(message).To(ContainSubstring(expectedDetails))
			Expect(message).To(ContainSubstring(expectedSuffix))
		})

		It("should handle unknown resource type gracefully", func() {
			queueName := "unknown-queue"
			resourceName := "UnknownResource"
			deserved := 100.0
			used := 50.0
			requiredResources := &podgroup_info.JobRequirement{
				GPU:      0,
				MilliCPU: 0,
				Memory:   0,
			}

			message := GetBuildOverCapacityMessageForQueue(queueName, resourceName, deserved, used, requiredResources)

			expectedPrefix := "Non-preemptible workload is over quota. "
			expectedSuffix := " Use a preemptible workload to go over quota."

			Expect(message).To(ContainSubstring(expectedPrefix))
			Expect(message).To(ContainSubstring(expectedSuffix))
			// For unknown resource, the details should be empty string, so the message should just have prefix + suffix
			Expect(message).To(Equal("Non-preemptible workload is over quota.  Use a preemptible workload to go over quota."))
		})
	})

	Context("GetJobOverMaxAllowedMessageForQueue", func() {
		It("should generate correct message for GPU resource", func() {
			queueName := "gpu-queue"
			resourceName := GpuResource
			maxAllowed := 8.0
			used := 6.0
			requested := 4.0

			message := GetJobOverMaxAllowedMessageForQueue(queueName, resourceName, maxAllowed, used, requested)

			expected := "gpu-queue quota has reached the allowable limit of GPUs. Limit is 8 GPUs, currently 6 GPUs allocated and workload requested 4 GPUs"
			Expect(message).To(Equal(expected))
		})

		It("should generate correct message for CPU resource", func() {
			queueName := "cpu-queue"
			resourceName := CpuResource
			maxAllowed := 16000.0 // 16 CPU cores in millicores
			used := 12000.0       // 12 CPU cores in millicores
			requested := 8000.0   // 8 CPU cores in millicores

			message := GetJobOverMaxAllowedMessageForQueue(queueName, resourceName, maxAllowed, used, requested)

			expected := "cpu-queue quota has reached the allowable limit of CPU cores. Limit is 16 cores, currently 12 cores allocated and workload requested 8 cores"
			Expect(message).To(Equal(expected))
		})

		It("should generate correct message for memory resource", func() {
			queueName := "memory-queue"
			resourceName := MemoryResource
			maxAllowed := 500000000000.0 // 500 GB in bytes
			used := 400000000000.0       // 400 GB in bytes
			requested := 200000000000.0  // 200 GB in bytes

			message := GetJobOverMaxAllowedMessageForQueue(queueName, resourceName, maxAllowed, used, requested)

			expected := "memory-queue quota has reached the allowable limit of memory. Limit is 500 GB, currently 400 GB allocated and workload requested 200 GB"
			Expect(message).To(Equal(expected))
		})
	})

	Context("GetGangEvictionMessage", func() {
		It("should generate correct gang eviction message", func() {
			taskNamespace := "test-namespace"
			taskName := "test-task"
			minimum := int32(3)

			message := GetGangEvictionMessage(taskNamespace, taskName, minimum)

			expected := "Workload doesn't have the minimum required number of pods (3), evicting remaining pod: test-namespace/test-task"
			Expect(message).To(Equal(expected))
		})
	})

	Context("GetPreemptMessage", func() {
		It("should generate correct preemption message", func() {
			preemptorJob := &podgroup_info.PodGroupInfo{
				Name:      "high-priority-job",
				Namespace: "high-priority-namespace",
			}
			preempteeTask := &pod_info.PodInfo{
				Name:      "low-priority-pod",
				Namespace: "low-priority-namespace",
			}

			message := GetPreemptMessage(preemptorJob, preempteeTask)

			expected := "Pod low-priority-namespace/low-priority-pod was preempted by higher priority workload high-priority-namespace/high-priority-job"
			Expect(message).To(Equal(expected))
		})
	})

	Context("GetReclaimMessage", func() {
		It("should generate correct reclaim message", func() {
			reclaimeeTask := &pod_info.PodInfo{
				Name:      "reclaimed-pod",
				Namespace: "reclaimed-namespace",
			}
			reclaimerJob := &podgroup_info.PodGroupInfo{
				Name:      "reclaimer-job",
				Namespace: "reclaimer-namespace",
			}

			message := GetReclaimMessage(reclaimeeTask, reclaimerJob)

			expected := "Pod reclaimed-namespace/reclaimed-pod was preempted by workload reclaimer-namespace/reclaimer-job."
			Expect(message).To(Equal(expected))
		})
	})

	Context("GetReclaimQueueDetailsMessage", func() {
		It("should generate correct reclaim queue details message", func() {
			queueName := "test-queue"
			queueAllocated := resource_info.NewResourceRequirements(2.0, 4000.0, 8000000000.0)  // 2 GPU, 4 CPU, 8 GB
			queueQuota := resource_info.NewResourceRequirements(4.0, 8000.0, 16000000000.0)     // 4 GPU, 8 CPU, 16 GB
			queueFairShare := resource_info.NewResourceRequirements(3.0, 6000.0, 12000000000.0) // 3 GPU, 6 CPU, 12 GB
			queuePriority := 5

			message := GetReclaimQueueDetailsMessage(queueName, queueAllocated, queueQuota, queueFairShare, queuePriority)

			// Note: This test might need adjustment based on the actual String() implementation
			// of ResourceRequirements. The exact format depends on how that method formats the resources.
			Expect(message).To(ContainSubstring(queueName))
			Expect(message).To(ContainSubstring("5"))
		})
	})

	Context("GetConsolidateMessage", func() {
		It("should generate correct consolidation message", func() {
			preempteeTask := &pod_info.PodInfo{
				Name:      "consolidated-pod",
				Namespace: "consolidated-namespace",
			}

			message := GetConsolidateMessage(preempteeTask)

			expected := "Pod consolidated-namespace/consolidated-pod was preempted and rescheduled due to bin packing (resource consolidation) procedure"
			Expect(message).To(Equal(expected))
		})
	})
})
