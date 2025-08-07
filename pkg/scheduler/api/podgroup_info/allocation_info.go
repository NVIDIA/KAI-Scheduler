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

func HasTasksToAllocate(podGroupInfo *PodGroupInfo, isRealAllocation bool) bool {
	for _, task := range podGroupInfo.PodInfos {
		if task.ShouldAllocate(isRealAllocation) {
			return true
		}
	}
	return false
}

func GetTasksToAllocate(
	podGroupInfo *PodGroupInfo, _ common_info.LessFn, taskOrderFn common_info.LessFn,
	isRealAllocation bool,
) []*pod_info.PodInfo {
	if podGroupInfo.tasksToAllocate != nil {
		return podGroupInfo.tasksToAllocate
	}

	var tasksToAllocate []*pod_info.PodInfo
	if len(podGroupInfo.SubGroups) > 0 {
		priorityQueueMap := getTasksPriorityQueuePerSubGroup(podGroupInfo, taskOrderFn, isRealAllocation)
		maxNumOfTasksToAllocateMap := getNumOfTasksToAllocatePerSubGroup(podGroupInfo)
		for subGroupName := range podGroupInfo.SubGroups {
			taskPriorityQueue := priorityQueueMap[subGroupName]
			maxNumOfTasksToAllocate := maxNumOfTasksToAllocateMap[subGroupName]
			subGroupTasks := getTasksFromQueue(taskPriorityQueue, maxNumOfTasksToAllocate)
			tasksToAllocate = append(tasksToAllocate, subGroupTasks...)
		}

	} else {
		taskPriorityQueue := getTasksPriorityQueue(podGroupInfo, taskOrderFn, isRealAllocation)
		maxNumOfTasksToAllocate := getNumOfTasksToAllocate(podGroupInfo)
		tasksToAllocate = getTasksFromQueue(taskPriorityQueue, maxNumOfTasksToAllocate)
	}

	podGroupInfo.tasksToAllocate = tasksToAllocate
	return tasksToAllocate
}

func getTasksFromQueue(priorityQueue *scheduler_util.PriorityQueue, maxNumTasks int) []*pod_info.PodInfo {
	var tasksToAllocate []*pod_info.PodInfo
	for !priorityQueue.Empty() && (len(tasksToAllocate) < maxNumTasks) {
		nextPod := priorityQueue.Pop().(*pod_info.PodInfo)
		tasksToAllocate = append(tasksToAllocate, nextPod)
	}
	return tasksToAllocate
}

func GetTasksToAllocateRequestedGPUs(
	podGroupInfo *PodGroupInfo, subGroupOrderFn common_info.LessFn, taskOrderFn common_info.LessFn,
	isRealAllocation bool,
) (float64, int64) {
	tasksTotalRequestedGPUs := float64(0)
	tasksTotalRequestedGpuMemory := int64(0)
	for _, task := range GetTasksToAllocate(podGroupInfo, subGroupOrderFn, taskOrderFn, isRealAllocation) {
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
	podGroupInfo *PodGroupInfo, subGroupOrderFn common_info.LessFn, taskOrderFn common_info.LessFn,
	isRealAllocation bool,
) *resource_info.Resource {
	if podGroupInfo == nil {
		return resource_info.EmptyResource()
	}
	if podGroupInfo.tasksToAllocateInitResource != nil {
		return podGroupInfo.tasksToAllocateInitResource
	}

	tasksTotalRequestedResource := resource_info.EmptyResource()
	for _, task := range GetTasksToAllocate(podGroupInfo, subGroupOrderFn, taskOrderFn, isRealAllocation) {
		if task.ShouldAllocate(isRealAllocation) {
			tasksTotalRequestedResource.AddResourceRequirements(task.ResReq)
		}
	}

	podGroupInfo.tasksToAllocateInitResource = tasksTotalRequestedResource
	return tasksTotalRequestedResource
}

func getTasksPriorityQueue(
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

func getTasksPriorityQueuePerSubGroup(
	podGroupInfo *PodGroupInfo, taskOrderFn common_info.LessFn, isRealAllocation bool,
) map[string]*scheduler_util.PriorityQueue {
	priorityQueuesMap := map[string]*scheduler_util.PriorityQueue{}
	for name, subGroup := range podGroupInfo.SubGroups {
		priorityQueue := scheduler_util.NewPriorityQueue(taskOrderFn, scheduler_util.QueueCapacityInfinite)
		for _, task := range subGroup.podInfos {
			if task.ShouldAllocate(isRealAllocation) {
				priorityQueue.Push(task)
			}
		}
		priorityQueuesMap[name] = priorityQueue
	}
	return priorityQueuesMap
}

func getNumOfTasksToAllocate(podGroupInfo *PodGroupInfo) int {
	numAllocatedTasks := podGroupInfo.GetActiveAllocatedTasksCount()
	if numAllocatedTasks >= int(podGroupInfo.MinAvailable) {
		return 1
	}
	return int(podGroupInfo.MinAvailable) - numAllocatedTasks
}

func getNumOfTasksToAllocatePerSubGroup(podGroupInfo *PodGroupInfo) map[string]int {
	maxTasksToAllocate := map[string]int{}
	for name, subGroup := range podGroupInfo.SubGroups {
		numAllocatedTasks := subGroup.GetNumActiveAllocatedTasks()
		if numAllocatedTasks >= int(subGroup.minAvailable) {
			maxTasksToAllocate[name] = int(math.Min(float64(subGroup.GetNumPendingTasks()), 1))
		} else {
			maxTasksToAllocate[name] = int(subGroup.minAvailable) - numAllocatedTasks
		}
	}
	return maxTasksToAllocate
}
