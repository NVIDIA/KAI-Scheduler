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
	for _, task := range podGroupInfo.GetAllPodsMap() {
		if task.ShouldAllocate(isRealAllocation) {
			return true
		}
	}
	return false
}

func GetTasksToAllocate(
	podGroupInfo *PodGroupInfo, subGroupOrderFn common_info.LessFn, taskOrderFn common_info.LessFn,
	isRealAllocation bool,
) []*pod_info.PodInfo {
	if podGroupInfo.tasksToAllocate != nil {
		return podGroupInfo.tasksToAllocate
	}

	var tasksToAllocate []*pod_info.PodInfo
	if len(podGroupInfo.GetActiveSubGroupInfos()) > 0 {
		priorityQueueMap := getTasksPriorityQueuePerSubGroup(podGroupInfo, taskOrderFn, isRealAllocation)
		maxNumOfTasksToAllocateMap := getNumTasksToAllocatePerSubGroup(podGroupInfo, isRealAllocation)

		subGroupPriorityQueue := getSubGroupsPriorityQueue(podGroupInfo.GetActiveSubGroupInfos(), subGroupOrderFn)
		maxNumOfSubGroups := getNumOfSubGroupsToAllocate(podGroupInfo)
		numAllocatedSubGroups := 0

		for !subGroupPriorityQueue.Empty() && (numAllocatedSubGroups < maxNumOfSubGroups) {
			nextSubGroup := subGroupPriorityQueue.Pop().(*SubGroupInfo)
			taskPriorityQueue := priorityQueueMap[nextSubGroup.GetName()]
			maxNumOfTasksToAllocate := maxNumOfTasksToAllocateMap[nextSubGroup.GetName()]
			subGroupTasks := getTasksFromQueue(taskPriorityQueue, maxNumOfTasksToAllocate)
			tasksToAllocate = append(tasksToAllocate, subGroupTasks...)
			numAllocatedSubGroups += 1
		}

	} else {
		taskPriorityQueue := getTasksPriorityQueue(podGroupInfo, taskOrderFn, isRealAllocation)
		maxNumOfTasksToAllocate := getNumTasksToAllocate(podGroupInfo)
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
	for _, task := range podGroupInfo.GetAllPodsMap() {
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
	for name, subGroup := range podGroupInfo.GetActiveSubGroupInfos() {
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

func getSubGroupsPriorityQueue(subGroups map[string]*SubGroupInfo,
	subGroupOrderFn common_info.LessFn) *scheduler_util.PriorityQueue {
	priorityQueue := scheduler_util.NewPriorityQueue(subGroupOrderFn, scheduler_util.QueueCapacityInfinite)
	for _, subGroup := range subGroups {
		priorityQueue.Push(subGroup)
	}
	return priorityQueue
}

func getNumTasksToAllocate(podGroupInfo *PodGroupInfo) int {
	numAllocatedTasks := podGroupInfo.GetActiveAllocatedTasksCount()
	if numAllocatedTasks >= int(podGroupInfo.GetDefaultMinAvailable()) {
		return 1
	}
	return int(podGroupInfo.GetDefaultMinAvailable()) - numAllocatedTasks
}

func getNumTasksToAllocatePerSubGroup(podGroupInfo *PodGroupInfo, isRealAllocation bool) map[string]int {
	maxTasksToAllocate := map[string]int{}
	for name, subGroup := range podGroupInfo.GetActiveSubGroupInfos() {
		numAllocatedTasks := subGroup.GetNumActiveAllocatedTasks()
		if numAllocatedTasks >= int(subGroup.minAvailable) {
			numTasksToAllocate := getNumAllocatableTasks(subGroup, isRealAllocation)
			maxTasksToAllocate[name] = int(math.Min(float64(numTasksToAllocate), 1))
		} else {
			maxTasksToAllocate[name] = int(subGroup.minAvailable) - numAllocatedTasks
		}
	}
	return maxTasksToAllocate
}

func getNumAllocatableTasks(subGroup *SubGroupInfo, isRealAllocation bool) int {
	numTasksToAllocate := 0
	for _, task := range subGroup.GetPodInfos() {
		if task.ShouldAllocate(isRealAllocation) {
			numTasksToAllocate += 1
		}
	}
	return numTasksToAllocate
}

func getNumOfSubGroupsToAllocate(podGroupInfo *PodGroupInfo) int {
	for _, subGroup := range podGroupInfo.GetActiveSubGroupInfos() {
		allocatedTasks := subGroup.GetNumActiveAllocatedTasks()
		if allocatedTasks >= int(subGroup.GetMinAvailable()) {
			return 1
		}
	}
	return len(podGroupInfo.SubGroups)
}
