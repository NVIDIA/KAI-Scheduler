// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package topology

import (
	"crypto/sha256"
	"encoding/hex"
	"slices"
	"strings"

	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/node_info"
	"github.com/samber/lo"
	kueuev1alpha1 "sigs.k8s.io/kueue/apis/kueue/v1alpha1"
)

type nodeSetID = string

// Function that accepts a node set (list of nodes) and return an identifier for the node set that can
// be used as a key in a map
func getNodeSetID(nodeSet node_info.NodeSet) nodeSetID {
	nodeNames := lo.Map(nodeSet, func(node *node_info.NodeInfo, _ int) string {
		return node.Name
	})
	slices.Sort(nodeNames)
	concatenated := strings.Join(nodeNames, ",")

	hash := sha256.Sum256([]byte(concatenated))
	return nodeSetID(hex.EncodeToString(hash[:]))
}

// LowestCommonDomainID returns the lowest common domain ID for a given node set and levels. If a node is missing one of
// the levels, the function will assume it's outside the topology and ignore it.
func LowestCommonDomainID(nodeSet node_info.NodeSet, levels []kueuev1alpha1.TopologyLevel) (DomainID, DomainLevel) {
	var validNodes, invalidNodes node_info.NodeSet
	for _, node := range nodeSet {
		if IsNodePartOfTopology(node, levels) {
			validNodes = append(validNodes, node)
		} else {
			invalidNodes = append(invalidNodes, node)
		}
	}

	var domainParts []string
	for _, level := range levels {
		allMatch := true
		var value string
		for _, node := range validNodes {
			newValue := node.Node.Labels[level.NodeLabel]

			if value == "" {
				value = newValue
			}

			if newValue != value {
				allMatch = false
				break
			}
		}

		if !allMatch || value == "" {
			break
		}

		domainParts = append(domainParts, value)
	}

	if len(domainParts) == 0 {
		return rootDomainId, rootLevel
	}

	return DomainID(strings.Join(domainParts, ".")), DomainLevel(domainParts[len(domainParts)-1])
}

// For a given node to be part of the topology correctly, it must have a label for each level of the topology. TODO make this common
func IsNodePartOfTopology(nodeInfo *node_info.NodeInfo, levels []kueuev1alpha1.TopologyLevel) bool {
	for _, level := range levels {
		if _, found := nodeInfo.Node.Labels[level.NodeLabel]; !found {
			return false
		}
	}
	return true
}
