// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package topology

import (
	"errors"
	"fmt"

	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/node_info"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/pod_info"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/podgroup_info"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/resource_info"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/log"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/plugins/scores"
	"k8s.io/apimachinery/pkg/types"
	k8sframework "k8s.io/kubernetes/pkg/scheduler/framework"
)

const (
	topologyAnnotationKey                   = "kai.scheduler/topology"
	topologyRequiredPlacementAnnotationKey  = "kai.scheduler/topology-required-placement"
	topologyPreferredPlacementAnnotationKey = "kai.scheduler/topology-preferred-placement"
)

type topologyStateData struct {
	relevantDomains []*TopologyDomainInfo
}

func (t *topologyStateData) Clone() k8sframework.StateData {
	return &topologyStateData{
		relevantDomains: t.relevantDomains,
	}
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
	t.calcTreeAlocationData(job, topologyTree)

	// Get best domains for the job
	jobAllocateableDomain, err := t.getBestjobAllocateableDomains(job, topologyTree)
	if err != nil {
		return err
	}

	// Clean allocation data from the tree
	for _, domain := range topologyTree.Domains {
		domain.AllocatablePods = 0
		domain.Distance = 0
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
	jobTopologyName := job.PodGroup.Annotations[topologyAnnotationKey]
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

func (t *topologyPlugin) calcTreeAlocationData(job *podgroup_info.PodGroupInfo, topologyTree *TopologyInfo) {
	taskToAllocateCount, leafDomains := t.calcAllocationsForLeafDomains(job, topologyTree)

	// Run from leafs to root updating distances and allocated pods
	latestUpdatedDomains := leafDomains
	currentlyUpdatedDomains := map[TopologyDomainID]*TopologyDomainInfo{}
	for len(latestUpdatedDomains) > 0 {
		for _, domain := range latestUpdatedDomains {
			if domain.Parent != nil {
				// TODO: subsetsum on allocateablePods, from them find min distance
				if domain.Parent.AllocatablePods < taskToAllocateCount {
					// If the parent domain has less allocateable pods than the tasks to allocate,
					// we need to update the parent domain with the current domain's allocateable pods
					if domain.AllocatablePods < taskToAllocateCount {
						domain.Parent.Distance += domain.Distance
						domain.Parent.AllocatablePods += domain.AllocatablePods
					} else {
						domain.Parent.Distance = domain.Distance
						domain.Parent.AllocatablePods = domain.AllocatablePods
					}
				} else if domain.Parent.Distance > domain.Distance {
					domain.Parent.Distance = domain.Distance
					domain.Parent.AllocatablePods = domain.AllocatablePods
				}
				currentlyUpdatedDomains[domain.Parent.ID] = domain.Parent
			}
		}
		latestUpdatedDomains = currentlyUpdatedDomains
		currentlyUpdatedDomains = map[TopologyDomainID]*TopologyDomainInfo{}
	}
}

func (t *topologyPlugin) getBestjobAllocateableDomains(job *podgroup_info.PodGroupInfo, topologyTree *TopologyInfo) ([]*TopologyDomainInfo, error) {
	relevantLevels, err := t.calculateRelevantDomainLevels(job, topologyTree.Name, topologyTree)
	if err != nil {
		return nil, err
	}
	taskToAllocateCount := len(podgroup_info.GetTasksToAllocate(job, t.taskOrderFunc, true))

	var jobAllocateableDomain []*TopologyDomainInfo

	minDistanceDomains := []*TopologyDomainInfo{}
	for _, domain := range topologyTree.Domains {
		// Filter domains above preffered and below required levels
		if _, relevantDomainLevel := relevantLevels[domain.Level]; !relevantDomainLevel {
			continue
		}

		// Filter domains that cannot allocate the job
		if domain.AllocatablePods < taskToAllocateCount {
			continue
		}

		if len(minDistanceDomains) == 0 || domain.Distance < minDistanceDomains[0].Distance {
			minDistanceDomains = []*TopologyDomainInfo{domain}
		} else if domain.Distance == minDistanceDomains[0].Distance {
			minDistanceDomains = append(minDistanceDomains, domain)
		}
	}

	// Calc domain with min/max free resources
	jobAllocateableDomain = minDistanceDomains
	return jobAllocateableDomain, nil
}

func (t *topologyPlugin) calcAllocationsForLeafDomains(job *podgroup_info.PodGroupInfo, topologyTree *TopologyInfo) (int, map[TopologyDomainID]*TopologyDomainInfo) {
	maxPodResources := resource_info.NewResourceRequirements(0, 0, 0)
	for _, podInfo := range job.PodInfos {
		maxPodResources.SetMaxResource(podInfo.ResReq)
	}
	taskToAllocateCount := len(podgroup_info.GetTasksToAllocate(job, t.taskOrderFunc, true))

	allocationTestPods := []*pod_info.PodInfo{
		{Name: "1-pods-resources", ResReq: maxPodResources},
	}
	leafDomains := map[TopologyDomainID]*TopologyDomainInfo{}
	for _, node := range t.nodesInfoMap {
		allocateablePodsCount := calcNodeAccomedation(maxPodResources, allocationTestPods, node, taskToAllocateCount)

		leafDomainId := calcLeafDomainId(topologyTree.TopologyResource, node.Node.Labels)
		domainInfo := topologyTree.Domains[leafDomainId]
		if domainInfo != nil {
			if allocateablePodsCount > 0 && domainInfo.AllocatablePods < taskToAllocateCount {
				domainInfo.Distance += 1
				domainInfo.AllocatablePods = min(domainInfo.AllocatablePods+allocateablePodsCount, taskToAllocateCount)
			}
			leafDomains[leafDomainId] = domainInfo
		}
	}
	return taskToAllocateCount, leafDomains
}

func calcNodeAccomedation(
	maxPodResources *resource_info.ResourceRequirements, allocationTestPods []*pod_info.PodInfo,
	node *node_info.NodeInfo, taskToAllocateCount int) int {
	allocateablePodsCount := 0
	for _, resourceRepresentorPod := range allocationTestPods {
		if node.IsTaskAllocatable(resourceRepresentorPod) {
			allocateablePodsCount++
		} else {
			break
		}
	}
	// Add more to jobResourcesAllocationsRepresentors until node cannot accommodate any more pods
	if allocateablePodsCount == len(allocationTestPods) {
		for i := allocateablePodsCount; i < taskToAllocateCount; i++ {
			latestTestPod := allocationTestPods[len(allocationTestPods)-1]

			iAllocationsTestPod := &pod_info.PodInfo{
				Name:   fmt.Sprintf("%d-pods-resources", allocateablePodsCount+1),
				ResReq: calcNextAllocationTestPodResources(latestTestPod.ResReq, maxPodResources),
			}
			allocationTestPods = append(allocationTestPods, iAllocationsTestPod)
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
	topologyTree *TopologyInfo) (map[string]bool, error) {
	requiredPlacement := job.PodGroup.Annotations[topologyRequiredPlacementAnnotationKey]
	preferredPlacement := job.PodGroup.Annotations[topologyPreferredPlacementAnnotationKey]
	if requiredPlacement == "" && preferredPlacement == "" {
		return nil, fmt.Errorf("no topology placement annotations found for job %s, workload topology name: %s", job.PodGroup.Name, jobTopologyName)
	}

	foundRequiredLevel := false
	foundPreferredLevel := false
	relevantLevels := map[string]bool{}
	abovePreferredLevel := preferredPlacement == ""
	for _, level := range topologyTree.TopologyResource.Spec.Levels {
		if preferredPlacement != "" && preferredPlacement == level.NodeLabel {
			foundPreferredLevel = true
			abovePreferredLevel = true
		}

		if !abovePreferredLevel {
			continue
		}
		relevantLevels[level.NodeLabel] = true

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
