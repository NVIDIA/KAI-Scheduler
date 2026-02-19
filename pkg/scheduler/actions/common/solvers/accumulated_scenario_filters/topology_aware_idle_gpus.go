// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package accumulated_scenario_filters

import (
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/actions/common/solvers/scenario"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/node_info"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/podgroup_info/subgroup_info"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/topology_info"
)

const (
	topologyAwareIdleGpuFilterName = "TopologyAwareIdleGpus"
)

type TopologyDomainKey struct {
	topologyName string
	level        string // Level within the topology (e.g., "rack", "zone")
	value        string // Specific domain value (e.g., "rack-1", "zone-a")
}

// TopologyAwareIdleGpus validates scenarios considering topology constraints
type TopologyAwareIdleGpus struct {
	domainCapacity        map[TopologyDomainKey]float64
	subgroupsWithRequired []*subgroup_info.SubGroupSet
	nodeToTopologyDomains map[string][]TopologyDomainKey
}

func NewTopologyAwareIdleGpusFilter(
	scenario *scenario.ByNodeScenario,
	nodeInfosMap map[string]*node_info.NodeInfo,
) *TopologyAwareIdleGpus {
	subgroupsWithRequired := extractRequiredTopologyConstraints(scenario)
	if len(subgroupsWithRequired) == 0 {
		return nil
	}

	domainCapacity, nodeToTopologyDomains := buildDomainCapacity(nodeInfosMap, subgroupsWithRequired)

	return &TopologyAwareIdleGpus{
		domainCapacity:        domainCapacity,
		subgroupsWithRequired: subgroupsWithRequired,
		nodeToTopologyDomains: nodeToTopologyDomains,
	}
}

func (taf *TopologyAwareIdleGpus) Name() string {
	return topologyAwareIdleGpuFilterName
}

func (taf *TopologyAwareIdleGpus) Filter(scenario *scenario.ByNodeScenario) (bool, error) {
	taf.updateDomainCapacityWithVictims(scenario)
	return taf.requiredTopologyCapacityExists(), nil
}

// updateDomainCapacityWithVictims updates domain capacities when new victims are added
func (taf *TopologyAwareIdleGpus) updateDomainCapacityWithVictims(scenario *scenario.ByNodeScenario) {
	// Get new potential victims since last call
	for _, victimTask := range scenario.PotentialVictimsTasks() {
		if victimTask.NodeName == "" {
			continue
		}

		// Get freed GPUs from this victim
		freedGpus := victimTask.AcceptedResource.GPUs()

		// Update all topology domains that include this node
		if domains, ok := taf.nodeToTopologyDomains[victimTask.NodeName]; ok {
			for _, domainKey := range domains {
				taf.domainCapacity[domainKey] += freedGpus
			}
		}
	}
}

// requiredTopologyCapacityExists checks if at least one domain can fit each SubGroupSet's tasks
func (taf *TopologyAwareIdleGpus) requiredTopologyCapacityExists() bool {
	for _, subgroup := range taf.subgroupsWithRequired {
		topologyConstraint := subgroup.GetTopologyConstraint()
		if topologyConstraint == nil || topologyConstraint.RequiredLevel == "" {
			continue
		}
		totalGpusNeeded := sumGpuRequirements(subgroup)
		if !taf.anyDomainHasCapacity(topologyConstraint, totalGpusNeeded) {
			return false
		}
	}
	return true
}

func sumGpuRequirements(subgroup *subgroup_info.SubGroupSet) float64 {
	podSets := subgroup.GetAllPodSets()
	totalGpusNeeded := 0.0
	for _, podSet := range podSets {
		for _, task := range podSet.GetPodInfos() {
			if task.ResReq != nil {
				totalGpusNeeded += task.ResReq.GPUs() + float64(task.ResReq.GetDraGpusCount())
			}
		}
	}
	return totalGpusNeeded
}

// anyDomainHasCapacity checks if at least one domain can fit the GPU requirements
func (taf *TopologyAwareIdleGpus) anyDomainHasCapacity(
	constraint *topology_info.TopologyConstraintInfo, totalGpusNeeded float64,
) bool {
	for domainKey, totalIdleGpus := range taf.domainCapacity {
		if domainKey.topologyName == constraint.Topology &&
			domainKey.level == constraint.RequiredLevel {
			if totalIdleGpus >= totalGpusNeeded {
				return true
			}
		}
	}
	return false
}

func extractRequiredTopologyConstraints(scenario *scenario.ByNodeScenario) []*subgroup_info.SubGroupSet {
	if scenario == nil {
		return nil
	}
	pendingJob := scenario.GetPreemptor()
	if pendingJob == nil || pendingJob.RootSubGroupSet == nil {
		return nil
	}
	return getSubgroupsWithRequiredConstraints(pendingJob.RootSubGroupSet, nil)
}

func getSubgroupsWithRequiredConstraints(
	jobSubGroup *subgroup_info.SubGroupSet,
	out []*subgroup_info.SubGroupSet,
) []*subgroup_info.SubGroupSet {
	if jobSubGroup == nil {
		return out
	}
	if jobSubGroup.GetTopologyConstraint() != nil && len(jobSubGroup.GetTopologyConstraint().RequiredLevel) > 0 {
		out = append(out, jobSubGroup)
	}
	for _, childGroup := range jobSubGroup.GetChildGroups() {
		out = getSubgroupsWithRequiredConstraints(childGroup, out)
	}
	return out
}

func buildDomainCapacity(
	nodeInfosMap map[string]*node_info.NodeInfo,
	subgroupsWithRequired []*subgroup_info.SubGroupSet,
) (map[TopologyDomainKey]float64, map[string][]TopologyDomainKey) {
	domainCapacity := make(map[TopologyDomainKey]float64)
	nodeToTopologyDomains := make(map[string][]TopologyDomainKey)

	for _, nodeInfo := range nodeInfosMap {
		nodeIdleGpus, _ := nodeInfo.GetSumOfIdleGPUs()
		nodeReleasingGpus, _ := nodeInfo.GetSumOfReleasingGPUs()
		totalIdleGpus := nodeIdleGpus + nodeReleasingGpus

		for _, subgroup := range subgroupsWithRequired {
			constraint := subgroup.GetTopologyConstraint()
			if constraint == nil {
				continue
			}
			domainValue := nodeInfo.Node.Labels[constraint.RequiredLevel]
			if len(domainValue) == 0 {
				continue
			}
			key := TopologyDomainKey{
				topologyName: constraint.Topology,
				level:        constraint.RequiredLevel,
				value:        domainValue,
			}
			domainCapacity[key] += totalIdleGpus
			nodeToTopologyDomains[nodeInfo.Name] = append(nodeToTopologyDomains[nodeInfo.Name], key)
		}
	}
	return domainCapacity, nodeToTopologyDomains
}
