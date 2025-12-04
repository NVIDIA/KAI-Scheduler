// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package topology

import (
	"math/rand"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/node_info"
)

func TestGetNodeSetID_ConsistencyWithLargeNodeSet(t *testing.T) {
	const numNodes = 10000

	nodeNames := make([]string, numNodes)
	for i := 0; i < numNodes; i++ {
		nodeNames[i] = "node-" + strconv.Itoa(i)
	}

	createNodeSet := func(names []string) node_info.NodeSet {
		nodeSet := make(node_info.NodeSet, len(names))
		for i, name := range names {
			nodeSet[i] = &node_info.NodeInfo{Name: name}
		}
		return nodeSet
	}

	expectedID := getNodeSetID(createNodeSet(nodeNames))

	// Verify consistency on repeated calls
	for i := 0; i < 10; i++ {
		assert.Equal(t, expectedID, getNodeSetID(createNodeSet(nodeNames)))
	}

	// Verify order independence with shuffled node sets
	for i := 0; i < 10; i++ {
		shuffled := make([]string, len(nodeNames))
		copy(shuffled, nodeNames)
		rand.New(rand.NewSource(int64(i))).Shuffle(len(shuffled), func(i, j int) {
			shuffled[i], shuffled[j] = shuffled[j], shuffled[i]
		})
		assert.Equal(t, expectedID, getNodeSetID(createNodeSet(shuffled)))
	}

	// Verify different node sets produce different IDs
	assert.NotEqual(t, expectedID, getNodeSetID(createNodeSet(nodeNames[:numNodes-1])))
}
