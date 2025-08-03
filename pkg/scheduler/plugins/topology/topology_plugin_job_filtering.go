// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package topology

import (
	"fmt"

	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/node_info"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/pod_info"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/podgroup_info"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/resource_info"
)

type jobAllocationMetaData struct {
	maxPodResources    *resource_info.ResourceRequirements
	allocationTestPods []*pod_info.PodInfo
	tasksToAllocate    []*pod_info.PodInfo
}

func (t *topologyPlugin) prePredicateFn(_ *pod_info.PodInfo, job *podgroup_info.PodGroupInfo) error {
	topologyTree, err := t.getJobTopology(job)
	if err != nil {
		return err
	}
	if topologyTree == nil {
		return nil
	}

	// Calc tree job allocation data
	_, err = t.calcTreeAllocatable(job, topologyTree)
	if err != nil {
		return err
	}

	// Clean allocation data from the tree
	for _, levelDomains := range topologyTree.DomainsByLevel {
		for _, domain := range levelDomains {
			domain.AllocatablePods = 0
		}
	}

	return nil
}

func (t *topologyPlugin) getJobTopology(job *podgroup_info.PodGroupInfo) (*TopologyInfo, error) {
	jobTopologyName := job.PodGroup.Spec.TopologyConstraint.Topology
	if jobTopologyName == "" {
		return nil, nil
	}
	topologyTree := t.TopologyTrees[jobTopologyName]
	if topologyTree == nil {
		return nil, fmt.Errorf("matching topology tree haven't been found for job %s, workload topology name: %s",
			job.PodGroup.Name, jobTopologyName)
	}
	return topologyTree, nil
}

func (t *topologyPlugin) calcTreeAllocatable(job *podgroup_info.PodGroupInfo, topologyTree *TopologyInfo) (int, error) {
	jobAllocationMetaData, err := initJobAllocationMetadataStruct(job, t)
	if err != nil {
		return 0, err
	}

	return t.calcSubTreeAllocatable(jobAllocationMetaData, topologyTree.Root)
}

func initJobAllocationMetadataStruct(job *podgroup_info.PodGroupInfo, t *topologyPlugin) (*jobAllocationMetaData, error) {
	tasksToAllocate := podgroup_info.GetTasksToAllocate(job, t.subGroupOrderFunc, t.taskOrderFunc, true)
	maxPodResources := resource_info.NewResourceRequirements(0, 0, 0)
	for _, podInfo := range tasksToAllocate {
		err := maxPodResources.SetMaxResource(podInfo.ResReq)
		if err != nil {
			return nil, err
		}
	}
	initialAllocationTestPods := []*pod_info.PodInfo{
		{Name: "1-pods-resources", ResReq: maxPodResources},
	}
	jobAllocationData := &jobAllocationMetaData{
		maxPodResources:    maxPodResources,
		allocationTestPods: initialAllocationTestPods,
		tasksToAllocate:    tasksToAllocate,
	}
	return jobAllocationData, nil
}

func (t *topologyPlugin) calcSubTreeAllocatable(jobAllocationData *jobAllocationMetaData, rootDomain *TopologyDomainInfo) (int, error) {
	if rootDomain == nil {
		return 0, nil
	}

	if len(rootDomain.Children) == 0 {
		for _, node := range rootDomain.Nodes {
			rootDomain.AllocatablePods += calcNodeAccomedation(jobAllocationData, node)
		}
		return rootDomain.AllocatablePods, nil
	}

	for _, child := range rootDomain.Children {
		childAllocateable, err := t.calcSubTreeAllocatable(jobAllocationData, child)
		if err != nil {
			return 0, err
		}
		rootDomain.AllocatablePods += childAllocateable
	}
	return rootDomain.AllocatablePods, nil
}

func calcNodeAccomedation(jobAllocationMetaData *jobAllocationMetaData, node *node_info.NodeInfo) int {
	allocateablePodsCount := 0
	for _, resourceRepresentorPod := range jobAllocationMetaData.allocationTestPods {
		if node.IsTaskAllocatable(resourceRepresentorPod) {
			allocateablePodsCount++
		} else {
			break
		}
	}
	// Add more to jobResourcesAllocationsRepresentors until node cannot accommodate any more pods
	if allocateablePodsCount == len(jobAllocationMetaData.allocationTestPods) {
		for i := allocateablePodsCount; i < len(jobAllocationMetaData.tasksToAllocate); i++ {
			latestTestPod := jobAllocationMetaData.allocationTestPods[len(jobAllocationMetaData.allocationTestPods)-1]

			iAllocationsTestPod := &pod_info.PodInfo{
				Name:   fmt.Sprintf("%d-pods-resources", allocateablePodsCount+1),
				ResReq: calcNextAllocationTestPodResources(latestTestPod.ResReq, jobAllocationMetaData.maxPodResources),
			}
			jobAllocationMetaData.allocationTestPods = append(jobAllocationMetaData.allocationTestPods, iAllocationsTestPod)
			if node.IsTaskAllocatable(iAllocationsTestPod) {
				allocateablePodsCount++
			} else {
				break
			}
		}
	}
	return allocateablePodsCount
}

func calcNextAllocationTestPodResources(previousTestResources, maxPodResources *resource_info.ResourceRequirements) *resource_info.ResourceRequirements {
	nPlus1Resources := previousTestResources.Clone()
	nPlus1Resources.BaseResource.Add(&maxPodResources.BaseResource)
	if len(nPlus1Resources.GpuResourceRequirement.MigResources()) > 0 {
		for migResource, quant := range maxPodResources.GpuResourceRequirement.MigResources() {
			nPlus1Resources.GpuResourceRequirement.MigResources()[migResource] += quant
		}
	} else {
		updatedGpuResource := resource_info.NewGpuResourceRequirementWithMultiFraction(
			nPlus1Resources.GetNumOfGpuDevices(),
			nPlus1Resources.GpuFractionalPortion(),
			nPlus1Resources.GpuMemory())
		nPlus1Resources.GpuResourceRequirement = *updatedGpuResource
	}
	return nPlus1Resources
}
