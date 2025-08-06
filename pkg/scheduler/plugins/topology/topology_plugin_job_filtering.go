// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package topology

import (
	"fmt"
	"slices"
	"sort"

	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/node_info"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/pod_info"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/podgroup_info"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/resource_info"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/log"
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

	defer t.treeAllocatableCleanup(topologyTree)
	maxAllocatablePods, err := t.calcTreeAllocatable(job, topologyTree)
	if err != nil {
		return err
	}

	if maxAllocatablePods < len(podgroup_info.GetTasksToAllocate(job, t.subGroupOrderFunc, t.taskOrderFunc, true)) {
		log.InfraLogger.V(6).Infof("no relevant domains found for job %s, workload topology name: %s",
			job.PodGroup.Name, topologyTree.Name)
		return nil
	}

	_, err = t.getBestJobAllocatableDomains(job, topologyTree)
	if err != nil {
		return err
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
			rootDomain.AllocatablePods += calcNodeAccommodation(jobAllocationData, node)
		}
		return rootDomain.AllocatablePods, nil
	}

	for _, child := range rootDomain.Children {
		childAllocatable, err := t.calcSubTreeAllocatable(jobAllocationData, child)
		if err != nil {
			return 0, err
		}
		rootDomain.AllocatablePods += childAllocatable
	}
	return rootDomain.AllocatablePods, nil
}

func calcNodeAccommodation(jobAllocationMetaData *jobAllocationMetaData, node *node_info.NodeInfo) int {
	allocatablePodsCount := 0
	for _, resourceRepresentorPod := range jobAllocationMetaData.allocationTestPods {
		if node.IsTaskAllocatable(resourceRepresentorPod) {
			allocatablePodsCount++
		} else {
			break
		}
	}
	// Add more to jobResourcesAllocationsRepresentors until node cannot accommodate any more pods
	if allocatablePodsCount == len(jobAllocationMetaData.allocationTestPods) {
		for i := allocatablePodsCount; i < len(jobAllocationMetaData.tasksToAllocate); i++ {
			latestTestPod := jobAllocationMetaData.allocationTestPods[len(jobAllocationMetaData.allocationTestPods)-1]

			iAllocationsTestPod := &pod_info.PodInfo{
				Name:   fmt.Sprintf("%d-pods-resources", allocatablePodsCount+1),
				ResReq: calcNextAllocationTestPodResources(latestTestPod.ResReq, jobAllocationMetaData.maxPodResources),
			}
			jobAllocationMetaData.allocationTestPods = append(jobAllocationMetaData.allocationTestPods, iAllocationsTestPod)
			if node.IsTaskAllocatable(iAllocationsTestPod) {
				allocatablePodsCount++
			} else {
				break
			}
		}
	}
	return allocatablePodsCount
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
			nPlus1Resources.GetNumOfGpuDevices()+maxPodResources.GetNumOfGpuDevices(),
			nPlus1Resources.GpuFractionalPortion(),
			nPlus1Resources.GpuMemory())
		nPlus1Resources.GpuResourceRequirement = *updatedGpuResource
	}
	return nPlus1Resources
}

func (t *topologyPlugin) getBestJobAllocatableDomains(job *podgroup_info.PodGroupInfo, topologyTree *TopologyInfo) ([]*TopologyDomainInfo, error) {
	relevantLevels, err := t.calculateRelevantDomainLevels(job, topologyTree.Name, topologyTree)
	if err != nil {
		return nil, err
	}
	taskToAllocateCount := len(podgroup_info.GetTasksToAllocate(job, t.subGroupOrderFunc, t.taskOrderFunc, true))

	maxDepthDomains := []*TopologyDomainInfo{}
	for _, level := range relevantLevels {
		for _, domain := range topologyTree.DomainsByLevel[level] {
			if domain.AllocatablePods < taskToAllocateCount { // Filter domains that cannot allocate the job
				continue
			}

			maxDepthDomains = append(maxDepthDomains, domain)
		}
		if len(maxDepthDomains) > 0 {
			break
		}
	}

	if len(maxDepthDomains) == 0 {
		return nil, fmt.Errorf("no domains found for the job %s, workload topology name: %s",
			job.PodGroup.Name, topologyTree.Name)
	}

	if job.PodGroup.Spec.TopologyConstraint.PreferredTopologyLevel != "" &&
		maxDepthDomains[0].Level != job.PodGroup.Spec.TopologyConstraint.PreferredTopologyLevel {
		// If Preferred is defined and we couldn't find a domain on the preferred level,
		// return a children subset and not a single domain
		return t.improveChoiceForPreference(maxDepthDomains, job)
	}

	// For stage 1, return a single domain
	return []*TopologyDomainInfo{maxDepthDomains[0]}, nil
}

func (*topologyPlugin) calculateRelevantDomainLevels(
	job *podgroup_info.PodGroupInfo, jobTopologyName string,
	topologyTree *TopologyInfo) ([]string, error) {
	requiredPlacement := job.PodGroup.Spec.TopologyConstraint.RequiredTopologyLevel
	preferredPlacement := job.PodGroup.Spec.TopologyConstraint.PreferredTopologyLevel
	if requiredPlacement == "" && preferredPlacement == "" {
		return nil, fmt.Errorf("no topology placement annotations found for job %s, workload topology name: %s", job.PodGroup.Name, jobTopologyName)
	}

	foundRequiredLevel := false
	foundPreferredLevel := false
	relevantLevels := []string{}
	abovePreferredLevel := preferredPlacement == ""
	for _, level := range topologyTree.TopologyResource.Spec.Levels {
		if preferredPlacement != "" && preferredPlacement == level.NodeLabel {
			foundPreferredLevel = true
			abovePreferredLevel = true
		}

		if !abovePreferredLevel {
			continue
		}
		relevantLevels = append(relevantLevels, level.NodeLabel)

		if requiredPlacement != "" && requiredPlacement == level.NodeLabel {
			foundRequiredLevel = true
			break // Next level won't fulfill the required placement
		}
	}
	if requiredPlacement != "" && !foundRequiredLevel {
		return nil, fmt.Errorf("the topology %s doesn't have a level matching the required(%s) specified for the job %s",
			jobTopologyName, requiredPlacement, job.Name,
		)
	}
	if preferredPlacement != "" && !foundPreferredLevel {
		return nil, fmt.Errorf("the topology %s doesn't have a level matching the preferred(%s) specified for the job %s",
			jobTopologyName, preferredPlacement, job.Name,
		)
	}
	return relevantLevels, nil
}

func (t *topologyPlugin) improveChoiceForPreference(maxDepthDomains []*TopologyDomainInfo, job *podgroup_info.PodGroupInfo) ([]*TopologyDomainInfo, error) {
	taskToAllocateCount := len(podgroup_info.GetTasksToAllocate(job, t.subGroupOrderFunc, t.taskOrderFunc, true))
	// Look for a subgroup of children domains that allows the job to be allocated
	// and return the one with the least number of domains required for the allocation
	bestChildrenSubset := []*TopologyDomainInfo{}
	for _, domain := range maxDepthDomains {
		childDomainSubset := getJobAllocatableChildrenSubset(domain, taskToAllocateCount)
		if len(bestChildrenSubset) == 0 || len(childDomainSubset) < len(bestChildrenSubset) {
			bestChildrenSubset = childDomainSubset
		}
	}
	return bestChildrenSubset, nil
}

func getJobAllocatableChildrenSubset(domain *TopologyDomainInfo, taskToAllocateCount int) []*TopologyDomainInfo {
	children := slices.Clone(domain.Children)
	sort.SliceStable(children, func(i, j int) bool {
		return children[i].AllocatablePods > children[j].AllocatablePods
	})

	allocatablePodsSum := 0
	childDomainSubset := []*TopologyDomainInfo{}
	for _, childDomain := range children {
		allocatablePodsSum += childDomain.AllocatablePods
		childDomainSubset = append(childDomainSubset, childDomain)
		if allocatablePodsSum >= taskToAllocateCount {
			break
		}
	}
	return childDomainSubset
}

func (*topologyPlugin) treeAllocatableCleanup(topologyTree *TopologyInfo) {
	for _, levelDomains := range topologyTree.DomainsByLevel {
		for _, domain := range levelDomains {
			domain.AllocatablePods = 0
		}
	}
}
