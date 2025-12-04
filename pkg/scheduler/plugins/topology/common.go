// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package topology

import (
	"crypto/sha256"
	"encoding/hex"
	"slices"

	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/node_info"
	"github.com/samber/lo"
)

type nodeSetID = string

// Function that accepts a node set (list of nodes) and return an identifier for the node set that can
// be used as a key in a map
func getNodeSetID(nodeSet node_info.NodeSet) nodeSetID {
	if len(nodeSet.ID) > 0 {
		return nodeSet.ID
	}

	nodeNames := lo.Map(nodeSet.Nodes, func(node *node_info.NodeInfo, _ int) string {
		return node.Name
	})
	slices.Sort(nodeNames)

	hasher := sha256.New()
	for _, nodeName := range nodeNames {
		hasher.Write([]byte(nodeName))
	}
	hash := hasher.Sum(nil)
	nodeSetID := hex.EncodeToString(hash)

	nodeSet.ID = nodeSetID // cache id as part of the NodeSet object to try to avoid recalculating it
	return nodeSet.ID
}
