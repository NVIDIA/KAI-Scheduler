// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package topology

import (
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/node_info"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/pod_info"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/framework"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/log"
	kueuev1alpha1 "sigs.k8s.io/kueue/apis/kueue/v1alpha1"
)

type topologyPlugin struct {
	enabled       bool
	TopologyTrees map[string]*TopologyInfo
}

func New(pluginArgs map[string]string) framework.Plugin {
	return &topologyPlugin{
		enabled:       true,
		TopologyTrees: map[string]*TopologyInfo{},
	}
}

func (t *topologyPlugin) Name() string {
	return "topology"
}

func (t *topologyPlugin) OnSessionOpen(ssn *framework.Session) {
	topologies, err := ssn.Cache.GetDataLister().ListTopologies()
	if err != nil {
		log.InfraLogger.Errorf("failed to list topologies", err)
		return
	}
	t.initializeTopologyTree(topologies, ssn)

	ssn.AddEventHandler(&framework.EventHandler{
		AllocateFunc:   t.handleAllocate(ssn),
		DeallocateFunc: t.handleDeallocate(ssn),
	})
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
	var previousDomainInfo *TopologyDomainInfo
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

		if previousDomainInfo != nil {
			previousDomainInfo.Parent = domainInfo
		}
		previousDomainInfo = domainInfo
	}
	previousDomainInfo.Parent = topologyTree.Root
}

func (t *topologyPlugin) handleAllocate(ssn *framework.Session) func(event *framework.Event) {
	return t.actTopologyChangesGivenpodEvent(ssn, func(domainInfo *TopologyDomainInfo, podInfo *pod_info.PodInfo) {
		domainInfo.AllocatedResources.AddResourceRequirements(podInfo.AcceptedResource)
		domainInfo.AllocatedPods = domainInfo.AllocatedPods + 1
	})
}

func (t *topologyPlugin) handleDeallocate(ssn *framework.Session) func(event *framework.Event) {
	return t.actTopologyChangesGivenpodEvent(ssn, func(domainInfo *TopologyDomainInfo, podInfo *pod_info.PodInfo) {
		domainInfo.AllocatedResources.SubResourceRequirements(podInfo.AcceptedResource)
		domainInfo.AllocatedPods = domainInfo.AllocatedPods - 1
	})
}

func (t *topologyPlugin) actTopologyChangesGivenpodEvent(
	ssn *framework.Session,
	action func(domainInfo *TopologyDomainInfo, podInfo *pod_info.PodInfo),
) func(event *framework.Event) {
	return func(event *framework.Event) {
		pod := event.Task.Pod
		nodeName := event.Task.NodeName
		node := ssn.Nodes[nodeName].Node
		podInfo := ssn.Nodes[nodeName].PodInfos[pod_info.PodKey(pod)]

		for _, topologyTree := range t.TopologyTrees {
			leafDomainId := calcLeafDomainId(topologyTree.TopologyResource, node.Labels)
			domainInfo := topologyTree.Domains[leafDomainId]
			for domainInfo != nil {
				action(domainInfo, podInfo)

				if domainInfo.Nodes[nodeName] != nil {
					break
				}
				domainInfo = domainInfo.Parent
			}
		}
	}
}

func (t *topologyPlugin) OnSessionClose(ssn *framework.Session) {}
