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

package podgroup_info

import (
	"crypto/sha256"
	"fmt"
	"time"

	"golang.org/x/exp/maps"
	"golang.org/x/exp/slices"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"

	enginev2alpha2 "github.com/NVIDIA/KAI-scheduler/pkg/apis/scheduling/v2alpha2"
	commonconstants "github.com/NVIDIA/KAI-scheduler/pkg/common/constants"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/common_info"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/pod_info"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/pod_status"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/resource_info"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/constants"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/log"
)

const (
	OverCapacity        = "OverCapacity"
	PodSchedulingErrors = "PodSchedulingErrors"
)

type JobRequirement struct {
	GPU      float64
	MilliCPU float64
	Memory   float64
}

type StalenessInfo struct {
	TimeStamp *time.Time
	Stale     bool
}

type PodGroupInfos struct {
	PodGroupInfos []*PodGroupInfo
}

type PodGroupInfo struct {
	UID common_info.PodGroupID

	Name           string
	Namespace      string
	NamespacedName string

	Queue common_info.QueueID

	Priority int32

	MinAvailable int32

	JobFitErrors   enginev2alpha2.UnschedulableExplanations
	NodesFitErrors map[common_info.PodID]*common_info.FitErrors

	// All tasks of the Job.
	PodInfos pod_info.PodsMap

	Allocated *resource_info.Resource

	CreationTimestamp  metav1.Time
	LastStartTimestamp *time.Time
	PodGroup           *enginev2alpha2.PodGroup
	PodGroupUID        types.UID
	SubGroups          map[string]*SubGroupInfo

	StalenessInfo

	schedulingConstraintsSignature common_info.SchedulingConstraintsSignature

	// inner cache
	tasksToAllocate             []*pod_info.PodInfo
	tasksToAllocateInitResource *resource_info.Resource
	PodStatusIndex              map[pod_status.PodStatus]pod_info.PodsMap
	activeAllocatedCount        *int
}

func NewPodGroupInfo(uid common_info.PodGroupID, tasks ...*pod_info.PodInfo) *PodGroupInfo {
	podGroupInfo := &PodGroupInfo{
		UID:          uid,
		MinAvailable: 0,
		Allocated:    resource_info.EmptyResource(),

		JobFitErrors:   make(enginev2alpha2.UnschedulableExplanations, 0),
		NodesFitErrors: make(map[common_info.PodID]*common_info.FitErrors),

		PodStatusIndex: map[pod_status.PodStatus]pod_info.PodsMap{},
		PodInfos:       pod_info.PodsMap{},

		StalenessInfo: StalenessInfo{
			TimeStamp: nil,
			Stale:     false,
		},

		SubGroups: map[string]*SubGroupInfo{},

		LastStartTimestamp:   nil,
		activeAllocatedCount: ptr.To(0),
	}

	for _, task := range tasks {
		podGroupInfo.AddTaskInfo(task)
	}

	return podGroupInfo
}

func (pgi *PodGroupInfo) IsPreemptibleJob(isInferencePreemptible bool) bool {
	if isInferencePreemptible {
		if pgi.Priority == constants.PriorityInferenceNumber {
			return true
		}
	}

	return pgi.Priority < constants.PriorityBuildNumber
}

func (pgi *PodGroupInfo) SetPodGroup(pg *enginev2alpha2.PodGroup) {
	pgi.Name = pg.Name
	pgi.Namespace = pg.Namespace
	pgi.NamespacedName = fmt.Sprintf("%s/%s", pgi.Namespace, pgi.Name)
	pgi.MinAvailable = max(pg.Spec.MinMember, 1)
	pgi.Queue = common_info.QueueID(pg.Spec.Queue)
	pgi.CreationTimestamp = pg.GetCreationTimestamp()
	pgi.PodGroup = pg
	pgi.PodGroupUID = pg.UID

	for _, sg := range pg.Spec.SubGroups {
		subGroupInfo := fromSubGroup(&sg)
		pgi.SubGroups[subGroupInfo.Name] = subGroupInfo
	}

	if pg.Annotations[commonconstants.StalePodgroupTimeStamp] != "" {
		staleTimeStamp, err := time.Parse(time.RFC3339, pg.Annotations[commonconstants.StalePodgroupTimeStamp])
		if err != nil {
			log.InfraLogger.V(7).Warnf("Failed to parse stale timestamp for podgroup <%s> err: %v",
				pgi.NamespacedName, err)
		} else {
			pgi.StalenessInfo.TimeStamp = &staleTimeStamp
			pgi.StalenessInfo.Stale = true
		}
	}

	if pg.Annotations[commonconstants.LastStartTimeStamp] != "" {
		startTime, err := time.Parse(time.RFC3339, pg.Annotations[commonconstants.LastStartTimeStamp])
		if err != nil {
			log.InfraLogger.V(7).Warnf("Failed to parse start timestamp for podgroup <%s> err: %v",
				pgi.NamespacedName, err)
		} else {
			pgi.LastStartTimestamp = &startTime
		}
	}

	log.InfraLogger.V(7).Infof(
		"SetPodGroup. podGroupName=<%s>, PodGroupUID=<%s> pgi.PodGroupIndex=<%d>",
		pgi.Name, pgi.PodGroupUID)
}

func (pgi *PodGroupInfo) addTaskIndex(ti *pod_info.PodInfo) {
	if _, found := pgi.PodStatusIndex[ti.Status]; !found {
		pgi.PodStatusIndex[ti.Status] = pod_info.PodsMap{}
	}

	pgi.PodStatusIndex[ti.Status][ti.UID] = ti
	if pod_status.IsActiveAllocatedStatus(ti.Status) {
		pgi.activeAllocatedCount = ptr.To(*pgi.activeAllocatedCount + 1)
	}

	pgi.invalidateTasksCache()
}

func (pgi *PodGroupInfo) AddTaskInfo(ti *pod_info.PodInfo) {
	pgi.PodInfos[ti.UID] = ti
	subGroup, found := pgi.SubGroups[ti.SubGroupName]
	if found {
		subGroup.assignTask(ti)
	}
	pgi.addTaskIndex(ti)

	if pod_status.AllocatedStatus(ti.Status) {
		pgi.Allocated.AddResourceRequirements(ti.ResReq)
	}
}

func (pgi *PodGroupInfo) UpdateTaskStatus(task *pod_info.PodInfo, status pod_status.PodStatus) error {
	// Reset the task state
	if err := pgi.resetTaskState(task); err != nil {
		return err
	}

	// Update task's status to the target status
	task.Status = status
	pgi.AddTaskInfo(task)

	return nil
}

func (pgi *PodGroupInfo) deleteTaskIndex(ti *pod_info.PodInfo) {
	if tasks, found := pgi.PodStatusIndex[ti.Status]; found {
		delete(tasks, ti.UID)
		if pod_status.IsActiveAllocatedStatus(ti.Status) {
			pgi.activeAllocatedCount = ptr.To(*pgi.activeAllocatedCount - 1)
		}

		if len(tasks) == 0 {
			delete(pgi.PodStatusIndex, ti.Status)
		}

		pgi.invalidateTasksCache()
	}
}

func (pgi *PodGroupInfo) invalidateTasksCache() {
	pgi.tasksToAllocate = nil
	pgi.tasksToAllocateInitResource = nil
}

func (pgi *PodGroupInfo) GetActiveAllocatedTasksCount() int {
	if pgi.activeAllocatedCount == nil {
		var taskCount int
		for _, task := range pgi.PodInfos {
			if pod_status.IsActiveAllocatedStatus(task.Status) {
				taskCount++
			}
		}
		pgi.activeAllocatedCount = ptr.To(taskCount)
	}
	return *pgi.activeAllocatedCount
}

func (pgi *PodGroupInfo) GetActivelyRunningTasksCount() int32 {
	tasksCount := int32(0)
	for _, task := range pgi.PodInfos {
		if pod_status.IsActiveUsedStatus(task.Status) {
			tasksCount += 1
		}
	}
	return tasksCount
}

func (pgi *PodGroupInfo) resetTaskState(ti *pod_info.PodInfo) error {
	task, found := pgi.PodInfos[ti.UID]
	if !found {
		return fmt.Errorf("failed to find task <%v/%v> in job <%v>",
			ti.Namespace, ti.Name, pgi.NamespacedName)
	}

	if pod_status.AllocatedStatus(task.Status) {
		pgi.Allocated.SubResourceRequirements(task.ResReq)
	}

	pgi.deleteTaskIndex(ti)
	return nil

}

func (pgi *PodGroupInfo) GetNumAliveTasks() int {
	numTasks := 0
	for _, task := range pgi.PodInfos {
		if pod_status.IsAliveStatus(task.Status) {
			numTasks += 1
		}
	}
	return numTasks
}

func (pgi *PodGroupInfo) GetNumActiveUsedTasks() int {
	numTasks := 0
	for _, task := range pgi.PodInfos {
		if pod_status.IsActiveUsedStatus(task.Status) {
			numTasks += 1
		}
	}
	return numTasks
}

func (pgi *PodGroupInfo) GetNumAllocatedTasks() int {
	numTasks := 0
	for _, task := range pgi.PodInfos {
		if pod_status.AllocatedStatus(task.Status) {
			numTasks++
		}
	}
	return numTasks
}

func (pgi *PodGroupInfo) GetPendingTasks() []*pod_info.PodInfo {
	var pendingTasks []*pod_info.PodInfo
	for _, task := range pgi.PodInfos {
		if task.Status == pod_status.Pending {
			pendingTasks = append(pendingTasks, task)
		}
	}
	return pendingTasks

}

func (pgi *PodGroupInfo) GetNumPendingTasks() int {
	return len(pgi.PodStatusIndex[pod_status.Pending])
}

func (pgi *PodGroupInfo) GetNumGatedTasks() int {
	return len(pgi.PodStatusIndex[pod_status.Gated])
}

func (pgi *PodGroupInfo) GetAliveTasksRequestedGPUs() float64 {
	tasksTotalRequestedGPUs := float64(0)
	for _, task := range pgi.PodInfos {
		if pod_status.IsAliveStatus(task.Status) {
			tasksTotalRequestedGPUs += task.ResReq.GPUs()
		}
	}

	return tasksTotalRequestedGPUs
}

func (pgi *PodGroupInfo) GetTasksActiveAllocatedReqResource() *resource_info.Resource {
	tasksTotalRequestedResource := resource_info.EmptyResource()
	for _, task := range pgi.PodInfos {
		if pod_status.IsActiveAllocatedStatus(task.Status) {
			tasksTotalRequestedResource.AddResourceRequirements(task.ResReq)
		}
	}

	return tasksTotalRequestedResource
}

func (pgi *PodGroupInfo) IsReadyForScheduling() bool {
	validTasks := pgi.GetNumAliveTasks() - pgi.GetNumGatedTasks()
	return int32(validTasks) >= pgi.MinAvailable
}

func (pgi *PodGroupInfo) IsElastic() bool {
	return pgi.MinAvailable < int32(len(pgi.PodInfos))
}

func (pgi *PodGroupInfo) IsStale() bool {
	if pgi.PodStatusIndex[pod_status.Succeeded] != nil {
		return false
	}

	activeUsedTasks := int32(pgi.GetNumActiveUsedTasks())
	return activeUsedTasks > 0 && activeUsedTasks < pgi.MinAvailable
}

func (pgi *PodGroupInfo) IsGangSatisfied() bool {
	numActiveTasks := pgi.GetNumActiveUsedTasks()
	return numActiveTasks >= int(pgi.MinAvailable)
}

func (pgi *PodGroupInfo) ShouldPipelineJob() bool {
	hasPipelinedTask := false
	activeAllocatedTasksCount := 0
	for _, task := range pgi.PodInfos {
		if task.Status == pod_status.Pipelined {
			log.InfraLogger.V(7).Infof("task: <%v/%v> was pipelined to node: <%v>",
				task.Namespace, task.Name, task.NodeName)
			hasPipelinedTask = true
		} else if pod_status.IsActiveAllocatedStatus(task.Status) {
			activeAllocatedTasksCount += 1
		}
	}
	// If the job has already MinAvailable tasks active allocated (but not pipelined),
	//  then we shouldn't convert non-pipelined tasks to pipeline.
	return hasPipelinedTask && activeAllocatedTasksCount < int(pgi.MinAvailable)
}

func (pgi *PodGroupInfo) Clone() *PodGroupInfo {
	return pgi.CloneWithTasks(maps.Values(pgi.PodInfos))
}

func (pgi *PodGroupInfo) CloneWithTasks(tasks []*pod_info.PodInfo) *PodGroupInfo {
	info := &PodGroupInfo{
		UID:       pgi.UID,
		Name:      pgi.Name,
		Namespace: pgi.Namespace,
		Queue:     pgi.Queue,
		Priority:  pgi.Priority,

		MinAvailable: pgi.MinAvailable,
		Allocated:    resource_info.EmptyResource(),

		JobFitErrors:   make(enginev2alpha2.UnschedulableExplanations, 0),
		NodesFitErrors: make(map[common_info.PodID]*common_info.FitErrors),

		PodGroup:    pgi.PodGroup,
		PodGroupUID: pgi.PodGroupUID,
		SubGroups:   map[string]*SubGroupInfo{},

		PodStatusIndex:       map[pod_status.PodStatus]pod_info.PodsMap{},
		PodInfos:             pod_info.PodsMap{},
		activeAllocatedCount: ptr.To(0),
	}

	pgi.CreationTimestamp.DeepCopyInto(&info.CreationTimestamp)

	for _, subGroup := range pgi.SubGroups {
		info.SubGroups[subGroup.Name] = newSubGroupInfo(subGroup.Name, subGroup.MinAvailable)
	}

	for _, task := range tasks {
		info.AddTaskInfo(task.Clone())
	}

	return info
}

func (pgi *PodGroupInfo) String() string {
	res := ""

	i := 0
	for _, task := range pgi.PodInfos {
		res = res + fmt.Sprintf("\n\t %d: %v", i, task)
		i++
	}

	return fmt.Sprintf("Job (%v): namespace %v (%v), name %v, minAvailable %d, podGroup %+v",
		pgi.UID, pgi.Namespace, pgi.Queue, pgi.Name, pgi.MinAvailable, pgi.PodGroup) + res
}

func (pgi *PodGroupInfo) SetTaskFitError(task *pod_info.PodInfo, fitErrors *common_info.FitErrors) {
	existingFitErrors, found := pgi.NodesFitErrors[task.UID]
	if found {
		existingFitErrors.AddNodeErrors(fitErrors)
	} else {
		pgi.NodesFitErrors[task.UID] = fitErrors
	}
}

func (pgi *PodGroupInfo) SetJobFitError(reason enginev2alpha2.UnschedulableReason, message string, details *enginev2alpha2.UnschedulableExplanationDetails) {
	pgi.JobFitErrors = append(pgi.JobFitErrors, enginev2alpha2.UnschedulableExplanation{
		Reason:  reason,
		Message: message,
		Details: details,
	})
}

func (pgi *PodGroupInfo) GetSchedulingConstraintsSignature() common_info.SchedulingConstraintsSignature {
	if pgi.schedulingConstraintsSignature != "" {
		return pgi.schedulingConstraintsSignature
	}

	key := pgi.generateSchedulingConstraintsSignature()

	pgi.schedulingConstraintsSignature = key
	return key
}

func (pgi *PodGroupInfo) generateSchedulingConstraintsSignature() common_info.SchedulingConstraintsSignature {
	hash := sha256.New()
	var signatures []common_info.SchedulingConstraintsSignature

	for _, pod := range pgi.PodInfos {
		if pod_status.IsActiveAllocatedStatus(pod.Status) {
			continue
		}

		key := pod.GetSchedulingConstraintsSignature()
		signatures = append(signatures, key)
	}
	slices.Sort(signatures)

	for _, signature := range signatures {
		hash.Write([]byte(signature))
	}

	return common_info.SchedulingConstraintsSignature(fmt.Sprintf("%x", hash.Sum(nil)))
}

func (jr *JobRequirement) Get(resourceName v1.ResourceName) float64 {
	switch resourceName {
	case v1.ResourceCPU:
		return jr.MilliCPU
	case v1.ResourceMemory:
		return jr.Memory
	case resource_info.GPUResourceName:
		return jr.GPU
	default:
		return 0
	}
}
