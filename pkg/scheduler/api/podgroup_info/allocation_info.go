// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package podgroup_info

import (
	"math"

	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/common_info"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/common_info/resources"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/pod_info"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/resource_info"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/log"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/scheduler_util"
)

func HasTasksToAllocate(pgi *PodGroupInfo, isRealAllocation bool) bool {
	for _, task := range pgi.PodInfos {
		if task.ShouldAllocate(isRealAllocation) {
			return true
		}
	}
	return false
}

func GetTasksToAllocate(
	pgi *PodGroupInfo, taskOrderFn common_info.LessFn, isRealAllocation bool,
) []*pod_info.PodInfo {
	if pgi.tasksToAllocate != nil {
		return pgi.tasksToAllocate
	}

	taskPriorityQueue := getTasksToAllocateQueue(pgi, taskOrderFn, isRealAllocation)
	maxNumOfTasksToAllocate := getNumOfTasksToAllocate(pgi, taskPriorityQueue.Len())

	var tasksToAllocate []*pod_info.PodInfo
	for !taskPriorityQueue.Empty() && (len(tasksToAllocate) < maxNumOfTasksToAllocate) {
		nextPod := taskPriorityQueue.Pop().(*pod_info.PodInfo)
		tasksToAllocate = append(tasksToAllocate, nextPod)
	}

	pgi.tasksToAllocate = tasksToAllocate
	return tasksToAllocate
}

func GetTasksToAllocateRequestedGPUs(
	pgi *PodGroupInfo, taskOrderFn common_info.LessFn, isRealAllocation bool,
) (float64, int64) {
	tasksTotalRequestedGPUs := float64(0)
	tasksTotalRequestedGpuMemory := int64(0)
	for _, task := range GetTasksToAllocate(pgi, taskOrderFn, isRealAllocation) {
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

func GetTasksToAllocateInitResource(
	pgi *PodGroupInfo, taskOrderFn common_info.LessFn, isRealAllocation bool,
) *resource_info.Resource {
	if pgi == nil {
		return resource_info.EmptyResource()
	}
	if pgi.tasksToAllocateInitResource != nil {
		return pgi.tasksToAllocateInitResource
	}

	tasksTotalRequestedResource := resource_info.EmptyResource()
	for _, task := range GetTasksToAllocate(pgi, taskOrderFn, isRealAllocation) {
		if task.ShouldAllocate(isRealAllocation) {
			tasksTotalRequestedResource.AddResourceRequirements(task.ResReq)
		}
	}

	pgi.tasksToAllocateInitResource = tasksTotalRequestedResource
	return tasksTotalRequestedResource
}

func getTasksToAllocateQueue(
	pgi *PodGroupInfo, taskOrderFn common_info.LessFn, isRealAllocation bool,
) *scheduler_util.PriorityQueue {
	podPriorityQueue := scheduler_util.NewPriorityQueue(taskOrderFn, scheduler_util.QueueCapacityInfinite)
	for _, task := range pgi.PodInfos {
		if task.ShouldAllocate(isRealAllocation) {
			podPriorityQueue.Push(task)
		}
	}
	return podPriorityQueue
}

func getNumOfTasksToAllocate(pgi *PodGroupInfo, numOfTasksWaitingAllocation int) int {
	allocatedTasks := int32(pgi.GetActiveAllocatedTasksCount())

	var maxTasksToAllocate int32
	if allocatedTasks >= pgi.MinAvailable {
		maxTasksToAllocate = 1
	} else {
		maxTasksToAllocate = pgi.MinAvailable
	}

	return int(math.Min(float64(maxTasksToAllocate), float64(numOfTasksWaitingAllocation)))
}
