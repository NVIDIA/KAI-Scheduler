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
	rootLevel          = "root"
	rootDomainId       = rootLevel
)

type topologyPlugin struct {
	TopologyTrees map[string]*Info
}

func New(_ map[string]string) framework.Plugin {
	return &topologyPlugin{
		TopologyTrees: map[string]*Info{},
	}
}

func (t *topologyPlugin) Name() string {
	return topologyPluginName
}

func (t *topologyPlugin) OnSessionOpen(ssn *framework.Session) {
	t.initializeTopologyTree(ssn.Topologies, ssn.Nodes)

	ssn.AddSubsetNodesFn(t.subSetNodesFn)
}

func (t *topologyPlugin) initializeTopologyTree(topologies []*kueuev1alpha1.Topology, nodes map[string]*node_info.NodeInfo) {
	for _, topology := range topologies {
		topologyTree := &Info{
			Name: topology.Name,
			DomainsByLevel: map[DomainLevel]LevelDomainInfos{
				rootLevel: {
					rootDomainId: NewDomainInfo(rootDomainId, rootLevel),
				},
			},
			TopologyResource: topology,
		}

		for _, nodeInfo := range nodes {
			t.addNodeDataToTopology(topologyTree, topology, nodeInfo)
		}

		t.TopologyTrees[topology.Name] = topologyTree
	}
}

func (*topologyPlugin) addNodeDataToTopology(topologyTree *Info, topology *kueuev1alpha1.Topology, nodeInfo *node_info.NodeInfo) {
	// Validate that the node is part of the topology
	if !isNodePartOfTopology(nodeInfo, topology) {
		return
	}

	var nodeContainingChildDomain *DomainInfo
	for levelIndex := len(topology.Spec.Levels) - 1; levelIndex >= 0; levelIndex-- {
		level := topology.Spec.Levels[levelIndex]

		domainId := calcDomainId(levelIndex, topology.Spec.Levels, nodeInfo.Node.Labels)
		domainLevel := DomainLevel(level.NodeLabel)
		domainsForLevel, foundLevelLabel := topologyTree.DomainsByLevel[domainLevel]
		if !foundLevelLabel {
			topologyTree.DomainsByLevel[domainLevel] = map[DomainID]*DomainInfo{}
			domainsForLevel = topologyTree.DomainsByLevel[domainLevel]
		}
		domainInfo, foundDomain := domainsForLevel[domainId]
		if !foundDomain {
			domainInfo = NewDomainInfo(domainId, domainLevel)
			domainsForLevel[domainId] = domainInfo
		}
		domainInfo.AddNode(nodeInfo)

		// Connect the child domain to the current domain. The current node gives us the link
		if nodeContainingChildDomain != nil {
			domainInfo.Children[nodeContainingChildDomain.ID] = nodeContainingChildDomain
		}
		nodeContainingChildDomain = domainInfo
	}

	topologyTree.DomainsByLevel[rootLevel][rootDomainId].Children[nodeContainingChildDomain.ID] = nodeContainingChildDomain
	topologyTree.DomainsByLevel[rootLevel][rootDomainId].AddNode(nodeInfo)
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

func (t *topologyPlugin) OnSessionClose(_ *framework.Session) {}
