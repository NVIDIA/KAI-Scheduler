// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package topology

import (
	kueuev1alpha1 "sigs.k8s.io/kueue/apis/kueue/v1alpha1"

	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/node_info"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/framework"
)

const (
	topologyPluginName = "topology"
)

type topologyPlugin struct {
	enabled       bool
	nodesInfos    map[string]*node_info.NodeInfo
	TopologyTrees map[string]*TopologyInfo
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
	t.nodesInfos = ssn.Nodes
	t.initializeTopologyTree(topologies, ssn)

	ssn.AddSubsetNodesFn(t.subSetNodesFn)
}

func (t *topologyPlugin) initializeTopologyTree(topologies []*kueuev1alpha1.Topology, ssn *framework.Session) {
	for _, singleTopology := range topologies {
		topologyTree := &TopologyInfo{
			Name:             singleTopology.Name,
			DomainsByLevel:   map[string]map[TopologyDomainID]*TopologyDomainInfo{},
			Root:             NewTopologyDomainInfo(TopologyDomainID("root"), "datacenter", "cluster", 0),
			TopologyResource: singleTopology,
		}
		topologyTree.DomainsByLevel["root"] = map[TopologyDomainID]*TopologyDomainInfo{
			topologyTree.Root.ID: topologyTree.Root,
		}

		for _, nodeInfo := range ssn.Nodes {
			t.addNodeDataToTopology(topologyTree, singleTopology, nodeInfo)
		}

		t.TopologyTrees[singleTopology.Name] = topologyTree
	}
}

func (*topologyPlugin) addNodeDataToTopology(topologyTree *TopologyInfo, singleTopology *kueuev1alpha1.Topology, nodeInfo *node_info.NodeInfo) {
	// Validate that the node is part of the topology
	if !isNodePartOfTopology(nodeInfo, singleTopology) {
		return
	}

	var nodeContainingChildDomain *TopologyDomainInfo
	for levelIndex := len(singleTopology.Spec.Levels) - 1; levelIndex >= 0; levelIndex-- {
		level := singleTopology.Spec.Levels[levelIndex]

		domainName, foundLevelLabel := nodeInfo.Node.Labels[level.NodeLabel]
		if !foundLevelLabel {
			continue // Skip if the node is not part of this level
		}

		domainId := calcDomainId(levelIndex, singleTopology.Spec.Levels, nodeInfo.Node.Labels)
		domainLevel := level.NodeLabel
		domainsForLevel, foundLevelLabel := topologyTree.DomainsByLevel[domainLevel]
		if !foundLevelLabel {
			topologyTree.DomainsByLevel[level.NodeLabel] = map[TopologyDomainID]*TopologyDomainInfo{}
			domainsForLevel = topologyTree.DomainsByLevel[level.NodeLabel]
		}
		domainInfo, foundDomain := domainsForLevel[domainId]
		if !foundDomain {
			domainInfo = NewTopologyDomainInfo(domainId, domainName, level.NodeLabel, levelIndex+1)
			domainsForLevel[domainId] = domainInfo
		}
		domainInfo.AddNode(nodeInfo)

		// Connect the child domain to the current domain. The current node gives us the link
		if nodeContainingChildDomain != nil {
			connectDomainToParent(nodeContainingChildDomain, domainInfo)
		}
		nodeContainingChildDomain = domainInfo
	}
	connectDomainToParent(nodeContainingChildDomain, topologyTree.Root)
	topologyTree.Root.AddNode(nodeInfo)
}

// For a given node to be part of the topology correctly, it must have a label for each level of the topology
func isNodePartOfTopology(nodeInfo *node_info.NodeInfo, singleTopology *kueuev1alpha1.Topology) bool {
	for _, level := range singleTopology.Spec.Levels {
		if _, found := nodeInfo.Node.Labels[level.NodeLabel]; !found {
			return false
		}
	}
	return true
}

func (t *topologyPlugin) OnSessionClose(ssn *framework.Session) {}
