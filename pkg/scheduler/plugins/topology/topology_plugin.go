// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package topology

import (
	kueuev1alpha1 "sigs.k8s.io/kueue/apis/kueue/v1alpha1"

	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/common_info"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/node_info"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/framework"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/k8s_internal"
)

const (
	topologyPluginName = "topology"
)

type topologyPlugin struct {
	enabled            bool
	taskOrderFunc      common_info.LessFn
	subGroupOrderFunc  common_info.LessFn
	sessionStateGetter k8s_internal.SessionStateProvider
	nodesInfos         map[string]*node_info.NodeInfo
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
	t.subGroupOrderFunc = ssn.SubGroupOrderFn
	t.sessionStateGetter = ssn
	t.nodesInfos = ssn.Nodes
	t.initializeTopologyTree(topologies, ssn)

	//pre-predicate to generate the whole topology tree and store per workload
	ssn.AddPrePredicateFn(t.prePredicateFn)
	//predicate to filter nodes that are related to parts of the tree that cannot accommodate the workload - this is for "required" use only
	ssn.AddPredicateFn(t.predicateFn)
	//node order to sort the nodes according to topology nodes score - this is for "prefer" use only
	ssn.AddNodeOrderFn(t.nodeOrderFn)
	//clean cycle cache after an allocation attempt for a job
	ssn.AddCleanAllocationAttemptCacheFn(t.cleanAllocationAttemptCache)
}

func (t *topologyPlugin) initializeTopologyTree(topologies []*kueuev1alpha1.Topology, ssn *framework.Session) {
	for _, singleTopology := range topologies {
		topologyTree := &TopologyInfo{
			Name:             singleTopology.Name,
			DomainsByLevel:   map[string]map[TopologyDomainID]*TopologyDomainInfo{},
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
			nodeContainingChildDomain.Parent = domainInfo
		}
		nodeContainingChildDomain = domainInfo
	}
	nodeContainingChildDomain.Parent = topologyTree.Root
	topologyTree.Root.AddNode(nodeInfo)
}

func (t *topologyPlugin) OnSessionClose(ssn *framework.Session) {}
