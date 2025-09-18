// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package topology

import (
	"fmt"
	"sort"

	k8sframework "k8s.io/kubernetes/pkg/scheduler/framework"

	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/node_info"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/pod_info"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/pod_status"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/podgroup_info"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/resource_info"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/log"
)

type topologyStateData struct {
	relevantDomains []*DomainInfo
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

func (t *topologyPlugin) subSetNodesFn(job *podgroup_info.PodGroupInfo, tasks []*pod_info.PodInfo, nodeSet node_info.NodeSet) ([]node_info.NodeSet, error) {
	topologyTree, err := t.getJobTopology(job)
	if err != nil {
		return nil, err
	}
	if topologyTree == nil {
		return []node_info.NodeSet{nodeSet}, nil
	}

	defer t.treeAllocatableCleanup(topologyTree)
	maxAllocatablePods, err := t.calcTreeAllocatable(tasks, topologyTree, nodeSet)
	if err != nil {
		return nil, err
	}

	if maxAllocatablePods < len(tasks) {
		log.InfraLogger.V(6).Infof("no relevant domains found for job %s, workload topology name: %s",
			job.PodGroup.Name, topologyTree.Name)
		return []node_info.NodeSet{}, nil
	}

	jobAllocatableDomains, err := t.getBestJobAllocatableDomains(job, len(tasks), topologyTree)
	if err != nil {
		return nil, err
	}

	result := node_info.NodeSet{}
	for _, jobAllocatableDomain := range jobAllocatableDomains {
		for _, node := range jobAllocatableDomain.Nodes {
			result = append(result, node)
		}
	}

	return []node_info.NodeSet{result}, nil
}

func (t *topologyPlugin) getJobTopology(job *podgroup_info.PodGroupInfo) (*Info, error) {
	if job.TopologyConstraint == nil {
		return nil, nil
	}
	jobTopologyName := job.TopologyConstraint.Topology
	if jobTopologyName == "" {
		return nil, nil
	}
	topologyTree := t.TopologyTrees[jobTopologyName]
	if topologyTree == nil {
		return nil, fmt.Errorf("matching topology tree haven't been found for job <%s/%s>, workload topology name: %s",
			job.Namespace, job.Name, jobTopologyName)
	}
	return topologyTree, nil
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
	allocatablePodsCount := 0
	for _, resourceRepresentorPod := range jobAllocationMetaData.allocationTestPods {
		if node.IsTaskAllocatable(resourceRepresentorPod) {
			allocatablePodsCount++
		} else {
			break
		}
	}
	// Add more to jobResourcesAllocationsRepresenters until the node cannot accommodate any more pods
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

func (t *topologyPlugin) getBestJobAllocatableDomains(job *podgroup_info.PodGroupInfo, taskToAllocateCount int, topologyTree *Info) ([]*DomainInfo, error) {
	relevantLevels, err := t.calculateRelevantDomainLevels(job, topologyTree)
	if err != nil {
		return nil, err
	}

	// Validate that the domains do not clash with the chosen domain for active pods of the job
	var relevantDomainsByLevel domainsByLevel
	if job.GetActiveAllocatedTasksCount() > 0 && jobHasTopologyRequiredConstraint(job) {
		relevantDomainsByLevel = getRelevantDomainsWithAllocatedPods(job, topologyTree, DomainLevel(job.TopologyConstraint.RequiredLevel))
	} else {
		relevantDomainsByLevel = topologyTree.DomainsByLevel
	}

	maxDepthDomains := []*DomainInfo{}
	for _, level := range relevantLevels {
		for _, domain := range relevantDomainsByLevel[level] {
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
		return nil, fmt.Errorf("no domains found for the job <%s/%s>, workload topology name: %s",
			job.Namespace, job.Name, topologyTree.Name)
	}

	if job.TopologyConstraint.PreferredLevel != "" &&
		maxDepthDomains[0].Level != DomainLevel(job.TopologyConstraint.PreferredLevel) {
		// If Preferred is defined and we couldn't find a domain on the preferred level,
		// return a children subset and not a single domain
		return t.improveChoiceForPreference(maxDepthDomains, taskToAllocateCount)
	}

	// For stage 1, return a single domain
	return []*DomainInfo{maxDepthDomains[0]}, nil
}

func getRelevantDomainsWithAllocatedPods(job *podgroup_info.PodGroupInfo, topologyTree *Info, requiredLevel DomainLevel) domainsByLevel {
	relevantDomainsByLevel := domainsByLevel{}
	for _, domainAtRequiredLevel := range topologyTree.DomainsByLevel[requiredLevel] {
		activePodsInDomain := countActiveJobPodsInDomain(job, domainAtRequiredLevel)
		if activePodsInDomain == 0 {
			continue // if the domain at the top level does not have any active pods, then any domains under the subtree cannot satisfy the required constraint for both active and pending pods
		}
		addSubTreeToDomainMap(domainAtRequiredLevel, relevantDomainsByLevel)
	}
	return relevantDomainsByLevel
}

func countActiveJobPodsInDomain(job *podgroup_info.PodGroupInfo, domain *DomainInfo) int {
	activePodsInDomain := 0
	for _, pod := range job.GetAllPodsMap() {
		if pod_status.IsActiveAllocatedStatus(pod.Status) {
			podInDomain := domain.Nodes[pod.NodeName] != nil
			if podInDomain {
				activePodsInDomain++
			}
		}
	}
	return activePodsInDomain
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

func jobHasTopologyRequiredConstraint(job *podgroup_info.PodGroupInfo) bool {
	return job.TopologyConstraint.RequiredLevel != ""
}

func (*topologyPlugin) calculateRelevantDomainLevels(
	job *podgroup_info.PodGroupInfo,
	topologyTree *Info) ([]DomainLevel, error) {
	requiredPlacement := DomainLevel(job.TopologyConstraint.RequiredLevel)
	preferredPlacement := DomainLevel(job.TopologyConstraint.PreferredLevel)
	if requiredPlacement == "" && preferredPlacement == "" {
		return nil, fmt.Errorf("no topology placement annotations found for job <%s/%s>, workload topology name: %s", job.Namespace, job.Name, topologyTree.Name)
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
		return nil, fmt.Errorf("the topology %s doesn't have a level matching the required(%s) specified for the job %s",
			topologyTree.Name, requiredPlacement, job.Name,
		)
	}
	if preferredPlacement != "" && !foundPreferredLevel {
		return nil, fmt.Errorf("the topology %s doesn't have a level matching the preferred(%s) specified for the job %s",
			topologyTree.Name, preferredPlacement, job.Name,
		)
	}
	return relevantLevels, nil
}

func (t *topologyPlugin) improveChoiceForPreference(maxDepthDomains []*DomainInfo, taskToAllocateCount int) ([]*DomainInfo, error) {
	// Look for a subgroup of children domains that allows the job to be allocated
	// and return the one with the least number of domains required for the allocation
	bestChildrenSubset := []*DomainInfo{}
	for _, domain := range maxDepthDomains {
		childDomainSubset := getJobAllocatableChildrenSubset(domain, taskToAllocateCount)
		if len(bestChildrenSubset) == 0 || len(childDomainSubset) < len(bestChildrenSubset) {
			bestChildrenSubset = childDomainSubset
		}
	}
	return bestChildrenSubset, nil
}

func getJobAllocatableChildrenSubset(domain *DomainInfo, taskToAllocateCount int) []*DomainInfo {
	children := make([]*DomainInfo, 0, len(domain.Children))
	for _, child := range domain.Children {
		children = append(children, child)
	}
	sort.SliceStable(children, func(i, j int) bool {
		return children[i].AllocatablePods > children[j].AllocatablePods
	})

	allocatablePodsSum := 0
	childDomainSubset := []*DomainInfo{}
	for _, childDomain := range children {
		allocatablePodsSum += childDomain.AllocatablePods
		childDomainSubset = append(childDomainSubset, childDomain)
		if allocatablePodsSum >= taskToAllocateCount {
			break
		}
	}
	return childDomainSubset
}

func (*topologyPlugin) treeAllocatableCleanup(topologyTree *Info) {
	for _, levelDomains := range topologyTree.DomainsByLevel {
		for _, domain := range levelDomains {
			domain.AllocatablePods = 0
		}
	}
}
