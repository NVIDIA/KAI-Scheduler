// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package podgroup_info

import (
	"math"

	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/common_info"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/common_info/resources"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/pod_info"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/pod_status"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/resource_info"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/log"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/scheduler_util"
)

func HasTasksToAllocate(podGroupInfo *PodGroupInfo, isRealAllocation bool) bool {
	for _, task := range podGroupInfo.PodInfos {
		if task.ShouldAllocate(isRealAllocation) {
			return true
		}
	}
	return false
}

func GetTasksToAllocate(
	podGroupInfo *PodGroupInfo, taskOrderFn common_info.LessFn, isRealAllocation bool,
) []*pod_info.PodInfo {
	taskPriorityQueue := getTasksToAllocateQueue(podGroupInfo, taskOrderFn, isRealAllocation)
	maxNumOfTasksToAllocate := getNumOfTasksToAllocate(podGroupInfo, taskPriorityQueue.Len())

	var tasksToAllocate []*pod_info.PodInfo
	for !taskPriorityQueue.Empty() && (len(tasksToAllocate) < maxNumOfTasksToAllocate) {
		nextPod := taskPriorityQueue.Pop().(*pod_info.PodInfo)
		tasksToAllocate = append(tasksToAllocate, nextPod)
	}

	return tasksToAllocate
}

func GetTasksToAllocateRequestedGPUs(
	podGroupInfo *PodGroupInfo, taskOrderFn common_info.LessFn, isRealAllocation bool,
) (float64, int64) {
	tasksTotalRequestedGPUs := float64(0)
	tasksTotalRequestedGpuMemory := int64(0)
	for _, task := range GetTasksToAllocate(podGroupInfo, taskOrderFn, isRealAllocation) {
		tasksTotalRequestedGPUs += task.ResReq.GPUs()
		tasksTotalRequestedGpuMemory += task.ResReq.GpuMemory()

		for migResource, quant := range task.ResReq.MigResources() {
			gpuPortion, mem, err := resources.ExtractGpuAndMemoryFromMigResourceName(migResource.String())
			if err != nil {
				log.InfraLogger.Errorf("failed to evaluate device portion for resource %v: %v", migResource, err)
				continue
			}
			tasksTotalRequestedGPUs += float64(int64(gpuPortion) * quant)
			tasksTotalRequestedGpuMemory += int64(mem) * quant
		}
	}

	return tasksTotalRequestedGPUs, tasksTotalRequestedGpuMemory
}

func GetJobsToAllocateInitResource(
	podGroupInfos []*PodGroupInfo, taskOrderFn common_info.LessFn, isRealAllocation bool,
) *resource_info.Resource {
	tasksTotalRequestedResource := resource_info.EmptyResource()
	for _, podGroupInfo := range podGroupInfos {
		pgInitResource := GetTasksToAllocateInitResource(podGroupInfo, taskOrderFn, isRealAllocation)
		tasksTotalRequestedResource.Add(pgInitResource)
	}

	return tasksTotalRequestedResource
}

func GetTasksToAllocateInitResource(
	podGroupInfo *PodGroupInfo, taskOrderFn common_info.LessFn, isRealAllocation bool,
) *resource_info.Resource {
	tasksTotalRequestedResource := resource_info.EmptyResource()

	if podGroupInfo == nil {
		return tasksTotalRequestedResource
	}

	for _, task := range GetTasksToAllocate(podGroupInfo, taskOrderFn, isRealAllocation) {
		if task.ShouldAllocate(isRealAllocation) {
			tasksTotalRequestedResource.AddResourceRequirements(task.ResReq)
		}
	}

	return tasksTotalRequestedResource
}

func getTasksToAllocateQueue(
	podGroupInfo *PodGroupInfo, taskOrderFn common_info.LessFn, isRealAllocation bool,
) *scheduler_util.PriorityQueue {
	podPriorityQueue := scheduler_util.NewPriorityQueue(taskOrderFn, scheduler_util.QueueCapacityInfinite)
	for _, task := range podGroupInfo.PodInfos {
		if task.ShouldAllocate(isRealAllocation) {
			podPriorityQueue.Push(task)
		}
	}
	return podPriorityQueue
}

func getNumOfTasksToAllocate(podGroupInfo *PodGroupInfo, numOfTasksWaitingAllocation int) int {
	allocatedTasks := int32(getNumOfAllocatedTasks(podGroupInfo))

	var maxTasksToAllocate int32
	if allocatedTasks >= podGroupInfo.MinAvailable {
		maxTasksToAllocate = 1
	} else {
		maxTasksToAllocate = podGroupInfo.MinAvailable
	}

	return int(math.Min(float64(maxTasksToAllocate), float64(numOfTasksWaitingAllocation)))
}

func getNumOfAllocatedTasks(podGroupInfo *PodGroupInfo) int {
	allocatedTasks := 0
	for _, task := range podGroupInfo.PodInfos {
		if pod_status.IsActiveAllocatedStatus(task.Status) {
			allocatedTasks += 1
		}
	}
	return allocatedTasks
}
