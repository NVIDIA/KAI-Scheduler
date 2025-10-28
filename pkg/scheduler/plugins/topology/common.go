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
