// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package topology

import (
	"cmp"
	"fmt"
	"slices"

	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/node_info"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/pod_info"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/pod_status"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/podgroup_info"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/podgroup_info/subgroup_info"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/resource_info"
)

type jobAllocationMetaData struct {
	maxPodResources    *resource_info.ResourceRequirements
	allocationTestPods []*pod_info.PodInfo
	tasksToAllocate    []*pod_info.PodInfo
}

func (t *topologyPlugin) subSetNodesFn(
	job *podgroup_info.PodGroupInfo, subGroup *subgroup_info.SubGroupInfo, podSets map[string]*subgroup_info.PodSet,
	tasks []*pod_info.PodInfo, nodeSet node_info.NodeSet,
) ([]node_info.NodeSet, error) {
	topologyTree, found := t.getJobTopology(subGroup)
	if !found {
		job.SetJobFitError(
			podgroup_info.PodSchedulingErrors,
			fmt.Sprintf("Matching topology %s does not exist",
				subGroup.GetTopologyConstraint().Topology),
			nil)
		return []node_info.NodeSet{}, nil
	}
	if topologyTree == nil {
		return []node_info.NodeSet{nodeSet}, nil
	}

	t.treeAllocatableCleanup(topologyTree)
	maxAllocatablePods, err := t.calcTreeAllocatable(tasks, topologyTree, nodeSet)
	if err != nil {
		return nil, err
	}

	if maxAllocatablePods < len(tasks) {
		job.SetJobFitError(
			podgroup_info.PodSchedulingErrors,
			fmt.Sprintf("No relevant domains found for workload in topology tree: %s", topologyTree.Name),
			nil)
		return []node_info.NodeSet{}, nil
	}

	// Sorting the tree for both packing and closest preferred level domain scoring
	preferredLevel := DomainLevel(subGroup.GetTopologyConstraint().PreferredLevel)
	requiredLevel := DomainLevel(subGroup.GetTopologyConstraint().RequiredLevel)
	maxDepthLevel := preferredLevel
	if maxDepthLevel == "" {
		maxDepthLevel = requiredLevel
	}
	sortTree(topologyTree.DomainsByLevel[rootLevel][rootDomainId], maxDepthLevel)
	if preferredLevel != "" {
		t.subGroupNodeScores[subGroup.GetName()] = calculateNodeScores(topologyTree.DomainsByLevel[rootLevel][rootDomainId], preferredLevel)
	}

	jobAllocatableDomains, err := t.getJobAllocatableDomains(job, subGroup, podSets, len(tasks), topologyTree)
	if err != nil {
		return nil, err
	}

	jobAllocatableDomains = sortDomainInfos(topologyTree, jobAllocatableDomains)

	var domainNodeSets []node_info.NodeSet
	for _, jobAllocatableDomain := range jobAllocatableDomains {
		var domainNodeSet node_info.NodeSet
		for _, node := range jobAllocatableDomain.Nodes {
			domainNodeSet = append(domainNodeSet, node)
		}
		domainNodeSets = append(domainNodeSets, domainNodeSet)
	}

	return domainNodeSets, nil
}

func (t *topologyPlugin) getJobTopology(subGroup *subgroup_info.SubGroupInfo) (*Info, bool) {
	if subGroup.GetTopologyConstraint() == nil {
		return nil, true
	}
	jobTopologyName := subGroup.GetTopologyConstraint().Topology
	if jobTopologyName == "" {
		return nil, true
	}
	topologyTree := t.TopologyTrees[jobTopologyName]
	if topologyTree == nil {
		return nil, false
	}
	return topologyTree, true
}

func (t *topologyPlugin) calcTreeAllocatable(tasks []*pod_info.PodInfo, topologyTree *Info, nodeSet node_info.NodeSet) (int, error) {
	jobAllocationData, err := initJobAllocationMetadataStruct(tasks)
	if err != nil {
		return 0, err
	}

	nodes := map[string]bool{}
	for _, node := range nodeSet {
		nodes[node.Name] = true
	}
	return t.calcSubTreeAllocatable(jobAllocationData, topologyTree.DomainsByLevel[rootLevel][rootDomainId], nodes)
}

func initJobAllocationMetadataStruct(tasksToAllocate []*pod_info.PodInfo) (*jobAllocationMetaData, error) {
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

func (t *topologyPlugin) calcSubTreeAllocatable(
	jobAllocationData *jobAllocationMetaData, domain *DomainInfo, nodes map[string]bool,
) (int, error) {
	if domain == nil {
		return 0, nil
	}

	if len(domain.Children) == 0 {
		for _, node := range domain.Nodes {
			if _, inSubset := nodes[node.Name]; !inSubset {
				continue
			}
			domain.AllocatablePods += calcNodeAccommodation(jobAllocationData, node)
		}
		return domain.AllocatablePods, nil
	}

	for _, child := range domain.Children {
		childAllocatable, err := t.calcSubTreeAllocatable(jobAllocationData, child, nodes)
		if err != nil {
			return 0, err
		}
		domain.AllocatablePods += childAllocatable
	}
	return domain.AllocatablePods, nil
}

func calcNodeAccommodation(jobAllocationMetaData *jobAllocationMetaData, node *node_info.NodeInfo) int {
	if jobAllocationMetaData.maxPodResources.LessEqual(resource_info.EmptyResourceRequirements()) {
		return len(jobAllocationMetaData.tasksToAllocate)
	}

	allocatablePodsCount := 0
	for _, resourceRepresentorPod := range jobAllocationMetaData.allocationTestPods {
		if node.IsTaskAllocatableOnReleasingOrIdle(resourceRepresentorPod) {
			allocatablePodsCount++
		} else {
			break
		}
	}
	// Add more to jobResourcesAllocationsRepresenters until the node cannot accommodate any more pods
	if allocatablePodsCount == len(jobAllocationMetaData.allocationTestPods) {
		for i := allocatablePodsCount; ; i++ {
			latestTestPod := jobAllocationMetaData.allocationTestPods[len(jobAllocationMetaData.allocationTestPods)-1]

			iAllocationsTestPod := &pod_info.PodInfo{
				Name:   fmt.Sprintf("%d-pods-resources", allocatablePodsCount+1),
				ResReq: calcNextAllocationTestPodResources(latestTestPod.ResReq, jobAllocationMetaData.maxPodResources),
			}
			jobAllocationMetaData.allocationTestPods = append(jobAllocationMetaData.allocationTestPods, iAllocationsTestPod)
			if node.IsTaskAllocatableOnReleasingOrIdle(iAllocationsTestPod) {
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

func (t *topologyPlugin) getJobAllocatableDomains(
	job *podgroup_info.PodGroupInfo, subGroup *subgroup_info.SubGroupInfo, podSets map[string]*subgroup_info.PodSet,
	taskToAllocateCount int, topologyTree *Info,
) ([]*DomainInfo, error) {
	relevantLevels, err := t.calculateRelevantDomainLevels(subGroup, topologyTree)
	if err != nil {
		return nil, err
	}

	// Validate that the domains do not clash with the chosen domain for active pods of the job
	var relevantDomainsByLevel domainsByLevel
	if hasActiveAllocatedTasks(podSets) && hasTopologyRequiredConstraint(subGroup) {
		relevantDomainsByLevel = getRelevantDomainsWithAllocatedPods(podSets, topologyTree,
			DomainLevel(subGroup.GetTopologyConstraint().RequiredLevel))
	} else {
		relevantDomainsByLevel = topologyTree.DomainsByLevel
	}

	var domains []*DomainInfo
	for _, level := range relevantLevels {
		for _, domain := range relevantDomainsByLevel[level] {
			if domain.AllocatablePods < taskToAllocateCount { // Filter domains that cannot allocate the job
				continue
			}

			domains = append(domains, domain)
		}
	}

	if len(domains) == 0 {
		return nil, fmt.Errorf("no domains found for the job <%s/%s>, workload topology name: %s",
			job.Namespace, job.Name, topologyTree.Name)
	}

	return domains, nil
}

func hasActiveAllocatedTasks(podSets map[string]*subgroup_info.PodSet) bool {
	for _, podSet := range podSets {
		if podSet.GetNumActiveAllocatedTasks() > 0 {
			return true
		}
	}
	return false
}

func getRelevantDomainsWithAllocatedPods(
	podSets map[string]*subgroup_info.PodSet, topologyTree *Info, requiredLevel DomainLevel,
) domainsByLevel {
	relevantDomainsByLevel := domainsByLevel{}
	for _, domainAtRequiredLevel := range topologyTree.DomainsByLevel[requiredLevel] {
		if !hasActiveJobPodInDomain(podSets, domainAtRequiredLevel) {
			continue // if the domain at the top level does not have any active pods, then any domains under the subtree cannot satisfy the required constraint for both active and pending pods
		}
		addSubTreeToDomainMap(domainAtRequiredLevel, relevantDomainsByLevel)
	}
	return relevantDomainsByLevel
}

func hasActiveJobPodInDomain(podSets map[string]*subgroup_info.PodSet, domain *DomainInfo) bool {
	for _, podSet := range podSets {
		for _, pod := range podSet.GetPodInfos() {
			if pod_status.IsActiveAllocatedStatus(pod.Status) {
				podInDomain := domain.Nodes[pod.NodeName] != nil
				if podInDomain {
					return true
				}
			}
		}
	}
	return false
}

func addSubTreeToDomainMap(domain *DomainInfo, domainsMap domainsByLevel) {
	if domainsMap[domain.Level] == nil {
		domainsMap[domain.Level] = map[DomainID]*DomainInfo{}
	}
	for _, childDomain := range domain.Children {
		addSubTreeToDomainMap(childDomain, domainsMap)
	}
	domainsMap[domain.Level][domain.ID] = domain
}

func hasTopologyRequiredConstraint(subGroup *subgroup_info.SubGroupInfo) bool {
	return subGroup.GetTopologyConstraint().RequiredLevel != ""
}

func (*topologyPlugin) calculateRelevantDomainLevels(
	subGroup *subgroup_info.SubGroupInfo, topologyTree *Info,
) ([]DomainLevel, error) {
	topologyConstraint := subGroup.GetTopologyConstraint()
	requiredPlacement := DomainLevel(topologyConstraint.RequiredLevel)
	preferredPlacement := DomainLevel(topologyConstraint.PreferredLevel)
	if requiredPlacement == "" && preferredPlacement == "" {
		return nil, fmt.Errorf("no topology constraints were found for subgroup %s, with topology name %s",
			subGroup.GetName(), topologyTree.Name)
	}

	foundRequiredLevel := false
	foundPreferredLevel := false

	levels := make([]DomainLevel, len(topologyTree.TopologyResource.Spec.Levels)+1)
	levels[len(levels)-1] = rootLevel
	for i, level := range topologyTree.TopologyResource.Spec.Levels {
		levels[len(levels)-2-i] = DomainLevel(level.NodeLabel)
	}

	var relevantLevels []DomainLevel
	for _, level := range levels {
		if level == requiredPlacement {
			foundRequiredLevel = true
			relevantLevels = append(relevantLevels, level)
			break
		}
		if level == preferredPlacement {
			foundPreferredLevel = true
		}
		if foundPreferredLevel {
			relevantLevels = append(relevantLevels, level)
		}
	}

	if requiredPlacement != "" && !foundRequiredLevel {
		return nil, fmt.Errorf("topology %s doesn't have a required domain level named %s",
			topologyTree.Name, requiredPlacement)
	}
	if preferredPlacement != "" && !foundPreferredLevel {
		return nil, fmt.Errorf("topology %s doesn't have a preferred domain level named %s",
			topologyTree.Name, preferredPlacement,
		)
	}
	return relevantLevels, nil
}

func (*topologyPlugin) treeAllocatableCleanup(topologyTree *Info) {
	for _, levelDomains := range topologyTree.DomainsByLevel {
		for _, domain := range levelDomains {
			domain.AllocatablePods = 0
		}
	}
}

// sortTree recursively sorts the topology tree for bin-packing behavior.
// Domains are sorted by AllocatablePods (ascending) to prioritize filling domains
// with fewer available resources first, implementing a bin-packing strategy.
// Within domains with equal AllocatablePods, sorts by ID for deterministic ordering.
func sortTree(root *DomainInfo, maxDepthLevel DomainLevel) {
	if root == nil || maxDepthLevel == "" {
		return
	}

	slices.SortFunc(root.Children, func(i, j *DomainInfo) int {
		if c := cmp.Compare(i.AllocatablePods, j.AllocatablePods); c != 0 {
			return c
		}
		return cmp.Compare(i.ID, j.ID)
	})

	if root.Level == maxDepthLevel {
		return
	}

	for _, child := range root.Children {
		sortTree(child, maxDepthLevel)
	}
}

// sortDomainInfos orders domains according to the sorted topology tree for consistent allocation.
// Assumes the topology tree is already sorted
func sortDomainInfos(topologyTree *Info, domainInfos []*DomainInfo) []*DomainInfo {
	root := topologyTree.DomainsByLevel[rootLevel][rootDomainId]
	reverseLevelOrderedDomains := reverseLevelOrder(root)

	sortedDomainInfos := make([]*DomainInfo, 0, len(domainInfos))
	for _, domain := range reverseLevelOrderedDomains {
		for _, domainInfo := range domainInfos {
			if domain.ID == domainInfo.ID && domain.Level == domainInfo.Level {
				sortedDomainInfos = append(sortedDomainInfos, domainInfo)
			}
		}
	}

	return sortedDomainInfos
}
