// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package topology

import (
	"errors"
	"fmt"
	"slices"
	"sort"

	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/node_info"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/pod_info"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/podgroup_info"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/resource_info"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/log"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/plugins/scores"
	"k8s.io/apimachinery/pkg/types"
	k8sframework "k8s.io/kubernetes/pkg/scheduler/framework"
)

type topologyStateData struct {
	relevantDomains []*TopologyDomainInfo
}

func (t *topologyStateData) Clone() k8sframework.StateData {
	return &topologyStateData{
		relevantDomains: t.relevantDomains,
	}
}

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

	//Check cycle cache to see if the calculation has already been done
	jobAllocateableDomains, _ := t.loadAllocateableDomainsFromCache(types.UID(job.PodGroupUID))
	if jobAllocateableDomains != nil {
		return nil
	}

	// Calc tree job allocation data
	maxAllocatablePods, err := t.calcTreeAllocatable(job, topologyTree)
	if err != nil {
		return err
	}

	// Get best domains for the job
	var jobAllocateableDomain []*TopologyDomainInfo
	if maxAllocatablePods >= len(podgroup_info.GetTasksToAllocate(job, t.taskOrderFunc, true)) {
		jobAllocateableDomain, err = t.getBestjobAllocateableDomains(job, topologyTree)
		if err != nil {
			return err
		}
	}

	// Clean allocation data from the tree
	for _, levelDomains := range topologyTree.DomainsByLevel {
		for _, domain := range levelDomains {
			domain.AllocatablePods = 0
		}
	}

	if len(jobAllocateableDomain) == 0 {
		log.InfraLogger.V(6).Infof("no relevant domains found for job %s, workload topology name: %s",
			job.PodGroup.Name, topologyTree.Name)
	}

	//Save results to cycle cache
	cycleJobState := (*k8sframework.CycleState)(t.sessionStateGetter.GetK8sStateForPod(job.PodGroupUID))
	cycleJobState.Write(
		k8sframework.StateKey(topologyPluginName),
		&topologyStateData{relevantDomains: jobAllocateableDomain},
	)

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
	tasksToAllocate := podgroup_info.GetTasksToAllocate(job, t.taskOrderFunc, true)
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

func (t *topologyPlugin) getBestjobAllocateableDomains(job *podgroup_info.PodGroupInfo, topologyTree *TopologyInfo) ([]*TopologyDomainInfo, error) {
	relevantLevels, err := t.calculateRelevantDomainLevels(job, topologyTree.Name, topologyTree)
	if err != nil {
		return nil, err
	}
	taskToAllocateCount := len(podgroup_info.GetTasksToAllocate(job, t.taskOrderFunc, true))

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

	if job.PodGroup.Spec.TopologyConstraint.PreferredTopologyLevel != "" {
		return t.improveChoiseForPreference(maxDepthDomains, job)
	}

	return maxDepthDomains, nil
}

func (t *topologyPlugin) improveChoiseForPreference(maxDepthDomains []*TopologyDomainInfo, job *podgroup_info.PodGroupInfo) ([]*TopologyDomainInfo, error) {
	// if Preferred is defined and we found a domain on the prefered level, return it
	taskToAllocateCount := len(podgroup_info.GetTasksToAllocate(job, t.taskOrderFunc, true))
	if maxDepthDomains[0].Level == job.PodGroup.Spec.TopologyConstraint.PreferredTopologyLevel {
		return []*TopologyDomainInfo{maxDepthDomains[0]}, nil
	}

	// else, look for a subgroup of children domains that allows the job to be allocated
	bestChildrenSubset := []*TopologyDomainInfo{}
	for _, domain := range maxDepthDomains {
		childDomainSubset := getJobAllocateableChildrenSubset(domain, taskToAllocateCount)
		if len(bestChildrenSubset) == 0 || len(childDomainSubset) < len(bestChildrenSubset) {
			bestChildrenSubset = childDomainSubset
		}
	}
	return bestChildrenSubset, nil
}

func getJobAllocateableChildrenSubset(domain *TopologyDomainInfo, taskToAllocateCount int) []*TopologyDomainInfo {
	children := slices.Clone(domain.Children)
	sort.SliceStable(children, func(i, j int) bool {
		return children[i].AllocatablePods > children[j].AllocatablePods
	})

	allocateablePodsSum := 0
	childDomainSubset := []*TopologyDomainInfo{}
	for _, childDomain := range children {
		allocateablePodsSum += childDomain.AllocatablePods
		childDomainSubset = append(childDomainSubset, childDomain)
		if allocateablePodsSum >= taskToAllocateCount {
			break
		}
	}
	return childDomainSubset
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
		return nil, fmt.Errorf("the topology %s doesn't have a level matching the required(%s) spesified for the job %s",
			jobTopologyName, requiredPlacement, job.Name,
		)
	}
	if preferredPlacement != "" && !foundPreferredLevel {
		return nil, fmt.Errorf("the topology %s doesn't have a level matching the preffered(%s) spesified for the job %s",
			jobTopologyName, preferredPlacement, job.Name,
		)
	}
	return relevantLevels, nil
}

func (t *topologyPlugin) predicateFn(pod *pod_info.PodInfo, job *podgroup_info.PodGroupInfo, node *node_info.NodeInfo) error {
	jobAllocateableDomains, err := t.loadAllocateableDomainsFromCache(job.PodGroupUID)
	if err != nil {
		return err
	}

	// For topology stage 1 - choose one domain, accept nodes allocation only if they are part of the chosen domain
	if len(jobAllocateableDomains) > 0 {
		chosenDomain := jobAllocateableDomains[0]
		if chosenDomain.Nodes[node.Node.Name] == nil {
			return fmt.Errorf("the node %s is not part of the chosen topology domain for the job %s. The chosen domain is %s",
				node.Node.Name, job.PodGroup.Name, chosenDomain.Name)
		}
	}

	return nil
}

func (t *topologyPlugin) nodeOrderFn(pod *pod_info.PodInfo, node *node_info.NodeInfo) (float64, error) {
	score := 0.0

	jobAllocateableDomains, err := t.loadAllocateableDomainsFromCache(types.UID(pod.Job))
	if err != nil {
		return score, err
	}

	// For topology stage 1 - choose one domain, accept nodes allocation only if they are part of the chosen domain
	if len(jobAllocateableDomains) > 0 {
		chosenDomain := jobAllocateableDomains[0]
		if chosenDomain.Nodes[node.Node.Name] != nil {
			score = scores.Topology
		}
	}

	return score, nil
}

func (t *topologyPlugin) loadAllocateableDomainsFromCache(podGroupUID types.UID) ([]*TopologyDomainInfo, error) {
	cycleJobState := (*k8sframework.CycleState)(t.sessionStateGetter.GetK8sStateForPod(podGroupUID))
	if cycleJobState == nil {
		return nil, nil
	}
	jobTopologyStateData, err := cycleJobState.Read(k8sframework.StateKey(topologyPluginName))
	if err != nil {
		if errors.Is(err, k8sframework.ErrNotFound) {
			return nil, nil
		}
		return nil, err
	}
	jobAllocateableDomains := jobTopologyStateData.(*topologyStateData).relevantDomains
	return jobAllocateableDomains, nil
}
