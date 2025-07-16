// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package topology

import (
	"strings"

	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/node_info"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/resource_info"
	kueuev1alpha1 "sigs.k8s.io/kueue/apis/kueue/v1alpha1"
)

// TopologyDomainID uniquely identifies a topology domain
type TopologyDomainID string

// TopologyInfo represents a topology tree for the cluster
type TopologyInfo struct {
	// Root of the topology tree
	Root *TopologyDomainInfo

	// Map of all domains by their ID for quick lookup
	Domains map[TopologyDomainID]*TopologyDomainInfo

	// Name of this topology configuration
	Name string

	// Topology resource
	TopologyResource *kueuev1alpha1.Topology
}

// TopologyDomainInfo represents a node in the topology tree
type TopologyDomainInfo struct {
	// Unique ID of this domain
	ID TopologyDomainID

	// Name of this domain
	Name string

	// Level in the hierarchy (e.g., "datacenter", "zone", "rack", "node")
	Level string

	// Parent domain, nil for root
	Parent *TopologyDomainInfo

	// Child domains
	Children []*TopologyDomainInfo

	// Nodes that belong to this domain
	Nodes map[string]*node_info.NodeInfo

	// Total available resources in this domain
	AvailableResources *resource_info.Resource

	// Total allocated resources in this domain
	AllocatedResources *resource_info.Resource

	// Number of pods that can be allocated in this domain for the job
	AllocatablePods int

	// List of resources requested by each pod in the job this tree is built for
	RequestedResources *resource_info.ResourceRequirements

	// Depth in the tree from root (0 for root)
	Depth int
}

func NewTopologyDomainInfo(id TopologyDomainID, name, level string, depth int) *TopologyDomainInfo {
	return &TopologyDomainInfo{
		ID:                 id,
		Name:               name,
		Level:              level,
		Parent:             nil,
		Children:           []*TopologyDomainInfo{},
		Nodes:              map[string]*node_info.NodeInfo{},
		AvailableResources: resource_info.EmptyResource(),
		AllocatedResources: resource_info.EmptyResource(),
		RequestedResources: nil,
		Depth:              depth,
	}
}

func calcDomainId(leafLevelIndex int, levels []kueuev1alpha1.TopologyLevel, nodeLabels map[string]string) TopologyDomainID {
	domainsNames := make([]string, leafLevelIndex+1)
	for levelIndex := leafLevelIndex; levelIndex >= 0; levelIndex-- {
		levelLabel := levels[levelIndex].NodeLabel
		levelDomainName, foundLevelOnNode := nodeLabels[levelLabel]
		if !foundLevelOnNode {
			levelDomainName = "missing"
		}
		domainsNames[levelIndex] = levelDomainName
	}
	return TopologyDomainID(strings.Join(domainsNames, "."))
}

func calcLeafDomainId(topologyResource *kueuev1alpha1.Topology, nodeLabels map[string]string) TopologyDomainID {
	return calcDomainId(len(topologyResource.Spec.Levels)-1, topologyResource.Spec.Levels, nodeLabels)
}

func (t *TopologyDomainInfo) AddNode(nodeInfo *node_info.NodeInfo) {
	t.Nodes[nodeInfo.Node.Name] = nodeInfo
	t.AvailableResources.Add(nodeInfo.Allocatable)
	t.AllocatedResources.Add(nodeInfo.Used)
	// TODO: do not take into account fractional gpus
	t.AllocatedResources.BaseResource.ScalarResources()["pods"] =
		t.AllocatedResources.BaseResource.ScalarResources()["pods"] + int64(len(nodeInfo.PodInfos))
}
