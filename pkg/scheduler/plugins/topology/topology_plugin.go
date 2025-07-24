// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package topology

import (
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/common_info"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/node_info"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/pod_info"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/resource_info"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/framework"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/k8s_internal"
	kueuev1alpha1 "sigs.k8s.io/kueue/apis/kueue/v1alpha1"
)

const (
	topologyPluginName = "topology"
	noNodeName         = ""
)

type topologyPlugin struct {
	enabled            bool
	taskOrderFunc      common_info.LessFn
	sessionStateGetter k8s_internal.SessionStateProvider
	nodesInfoMap       map[string]*node_info.NodeInfo
	TopologyTrees      map[string]*TopologyInfo
}

func New(pluginArgs map[string]string) framework.Plugin {
	return &topologyPlugin{
		enabled:       true,
		TopologyTrees: map[string]*TopologyInfo{},
	}
}

func (t *topologyPlugin) Name() string {
	return topologyPluginName
}

func (t *topologyPlugin) OnSessionOpen(ssn *framework.Session) {
	topologies := ssn.Topologies
	t.taskOrderFunc = ssn.TaskOrderFn
	t.sessionStateGetter = ssn
	t.nodesInfoMap = ssn.Nodes
	t.initializeTopologyTree(topologies, ssn)

	ssn.AddEventHandler(&framework.EventHandler{
		AllocateFunc:   t.handleAllocate(ssn),
		DeallocateFunc: t.handleDeallocate(ssn),
	})

	//pre-predicate to generate the whole topology tree and store per workload
	ssn.AddPrePredicateFn(t.prePredicateFn)
	//predicate to filter nodes that are related to parts of the tree that cannot accommodate the workload - this is for "required" use only
	ssn.AddPredicateFn(t.predicateFn)
	//node order to sort the nodes according to topology nodes score - this is for "prefer" use only
	ssn.AddNodeOrderFn(t.nodeOrderFn)
}

func (*topologyPlugin) addDomainIfValidForJobAllocation(
	domain *TopologyDomainInfo, jobNextAllocationResources *resource_info.Resource,
	relevantDomains []*TopologyDomainInfo) []*TopologyDomainInfo {
	idleDomainResources := domain.AvailableResources.Clone()
	idleDomainResources.Sub(domain.AllocatedResources)
	freePodsInDomain := int(idleDomainResources.BaseResource.ScalarResources()["pods"]) - domain.AllocatablePods

	domainResourceAvalability := jobNextAllocationResources.LessEqual(idleDomainResources)
	domainPodAvalability := freePodsInDomain > 0
	if domainResourceAvalability && domainPodAvalability {
		if len(relevantDomains) == 0 || domain.Depth > relevantDomains[0].Depth {
			relevantDomains = []*TopologyDomainInfo{domain}
		} else if domain.Depth == relevantDomains[0].Depth {
			relevantDomains = append(relevantDomains, domain)
		}
	}
	return relevantDomains
}

func (t *topologyPlugin) initializeTopologyTree(topologies []*kueuev1alpha1.Topology, ssn *framework.Session) {
	for _, singleTopology := range topologies {
		topologyTree := &TopologyInfo{
			Name:             singleTopology.Name,
			Domains:          map[TopologyDomainID]*TopologyDomainInfo{},
			Root:             NewTopologyDomainInfo(TopologyDomainID("root"), "datacenter", "cluster", 0),
			TopologyResource: singleTopology,
		}

		for _, nodeInfo := range ssn.Nodes {
			t.addNodeDataToTopology(topologyTree, singleTopology, nodeInfo)
		}

		t.TopologyTrees[singleTopology.Name] = topologyTree
	}
}

func (*topologyPlugin) addNodeDataToTopology(topologyTree *TopologyInfo, singleTopology *kueuev1alpha1.Topology, nodeInfo *node_info.NodeInfo) {
	var nodeContainingChildDomain *TopologyDomainInfo
	for levelIndex := len(singleTopology.Spec.Levels) - 1; levelIndex >= 0; levelIndex-- {
		level := singleTopology.Spec.Levels[levelIndex]

		domainName, foundLevelLabel := nodeInfo.Node.Labels[level.NodeLabel]
		if !foundLevelLabel {
			continue // Skip if the node is not part of this level
		}

		domainId := calcDomainId(levelIndex, singleTopology.Spec.Levels, nodeInfo.Node.Labels)
		domainInfo, foundLevelLabel := topologyTree.Domains[domainId]
		if !foundLevelLabel {
			domainInfo = NewTopologyDomainInfo(domainId, domainName, level.NodeLabel, levelIndex+1)
			topologyTree.Domains[domainId] = domainInfo
		}
		domainInfo.AddNode(nodeInfo)

		// Connect the child domain to the current domain. The current node gives us the link
		if nodeContainingChildDomain != nil {
			nodeContainingChildDomain.Parent = domainInfo
		}
		nodeContainingChildDomain = domainInfo
	}
	nodeContainingChildDomain.Parent = topologyTree.Root
	topologyTree.Root.AddNode(nodeInfo)
}

func (t *topologyPlugin) handleAllocate(ssn *framework.Session) func(event *framework.Event) {
	return t.updateTopologyGivenPodEvent(ssn, func(domainInfo *TopologyDomainInfo, podInfo *pod_info.PodInfo) {
		domainInfo.AllocatedResources.AddResourceRequirements(podInfo.AcceptedResource)
		domainInfo.AllocatedResources.BaseResource.ScalarResources()["pods"] =
			domainInfo.AllocatedResources.BaseResource.ScalarResources()["pods"] + 1
	})
}

func (t *topologyPlugin) handleDeallocate(ssn *framework.Session) func(event *framework.Event) {
	return t.updateTopologyGivenPodEvent(ssn, func(domainInfo *TopologyDomainInfo, podInfo *pod_info.PodInfo) {
		domainInfo.AllocatedResources.SubResourceRequirements(podInfo.AcceptedResource)
		domainInfo.AllocatedResources.BaseResource.ScalarResources()["pods"] =
			domainInfo.AllocatedResources.BaseResource.ScalarResources()["pods"] - 1
	})
}

func (t *topologyPlugin) updateTopologyGivenPodEvent(
	ssn *framework.Session,
	domainUpdater func(domainInfo *TopologyDomainInfo, podInfo *pod_info.PodInfo),
) func(event *framework.Event) {
	return func(event *framework.Event) {
		pod := event.Task.Pod
		nodeName := event.Task.NodeName
		if nodeName == noNodeName {
			return
		}
		node := ssn.Nodes[nodeName].Node
		podInfo := ssn.Nodes[nodeName].PodInfos[pod_info.PodKey(pod)]

		for _, topologyTree := range t.TopologyTrees {
			leafDomainId := calcLeafDomainId(topologyTree.TopologyResource, node.Labels)
			domainInfo := topologyTree.Domains[leafDomainId]
			for domainInfo != nil {
				domainUpdater(domainInfo, podInfo)

				if domainInfo.Nodes[nodeName] != nil {
					break
				}
				domainInfo = domainInfo.Parent
			}
		}
	}
}

func (t *topologyPlugin) OnSessionClose(ssn *framework.Session) {}
