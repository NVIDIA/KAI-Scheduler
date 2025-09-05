/*
Copyright 2023 NVIDIA CORPORATION
SPDX-License-Identifier: Apache-2.0
*/

package framework

import (
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/node_info"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/podgroup_info"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/pod_info"
)

func TestMutateBindRequestAnnotations(t *testing.T) {
	tests := []struct {
		name                string
		mutateFns           []api.BindRequestMutateFn
		expectedAnnotations map[string]string
	}{
		{
			name:                "no mutate functions",
			mutateFns:           []api.BindRequestMutateFn{},
			expectedAnnotations: map[string]string{},
		},
		{
			name: "single mutate function",
			mutateFns: []api.BindRequestMutateFn{
				func(pod *pod_info.PodInfo, nodeName string) map[string]string {
					return map[string]string{"key1": "value1"}
				},
			},
			expectedAnnotations: map[string]string{"key1": "value1"},
		},
		{
			name: "multiple mutate functions with different keys",
			mutateFns: []api.BindRequestMutateFn{
				func(pod *pod_info.PodInfo, nodeName string) map[string]string {
					return map[string]string{"key1": "value1"}
				},
				func(pod *pod_info.PodInfo, nodeName string) map[string]string {
					return map[string]string{"key2": "value2"}
				},
			},
			expectedAnnotations: map[string]string{"key1": "value1", "key2": "value2"},
		},
		{
			name: "multiple mutate functions with overlapping keys - later should override",
			mutateFns: []api.BindRequestMutateFn{
				func(pod *pod_info.PodInfo, nodeName string) map[string]string {
					return map[string]string{"key1": "value1", "common": "first"}
				},
				func(pod *pod_info.PodInfo, nodeName string) map[string]string {
					return map[string]string{"key2": "value2", "common": "second"}
				},
			},
			expectedAnnotations: map[string]string{"key1": "value1", "key2": "value2", "common": "second"},
		},
		{
			name: "mutate function returns nil map",
			mutateFns: []api.BindRequestMutateFn{
				func(pod *pod_info.PodInfo, nodeName string) map[string]string {
					return map[string]string{"key1": "value1"}
				},
				func(pod *pod_info.PodInfo, nodeName string) map[string]string {
					return nil
				},
			},
			expectedAnnotations: map[string]string{"key1": "value1"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ssn := &Session{
				BindRequestMutateFns: tt.mutateFns,
			}
			pod := &pod_info.PodInfo{
				Name: "test-pod",
			}
			nodeName := "test-node"
			annotations := ssn.MutateBindRequestAnnotations(pod, nodeName)
			assert.Equal(t, tt.expectedAnnotations, annotations)
		})
	}
}

func TestPartitionMultiImplementation(t *testing.T) {
	nodes := []*node_info.NodeInfo{
		{
			Name: "node0ing",
		},
		{
			Name: "node1ing",
		},
		{
			Name: "node2",
		},
		{
			Name: "node3ing",
		},
		{
			Name: "node4ing",
		},
		{
			Name: "node5",
		},
	}

	evenPartition := func(job *podgroup_info.PodGroupInfo, _ []*pod_info.PodInfo, partition api.NodePartition) ([]api.NodePartition, error) {
		partition1 := []*node_info.NodeInfo{}
		partition2 := []*node_info.NodeInfo{}
		for i, node := range partition {
			if i%2 == 0 {
				partition1 = append(partition1, node)
			} else {
				partition2 = append(partition2, node)
			}
		}
		return []api.NodePartition{partition1, partition2}, nil
	}

	ingPartition := func(job *podgroup_info.PodGroupInfo, _ []*pod_info.PodInfo, partition api.NodePartition) ([]api.NodePartition, error) {
		partition1 := []*node_info.NodeInfo{}
		partition2 := []*node_info.NodeInfo{}
		for _, node := range partition {
			if strings.Contains(node.Name, "ing") {
				partition1 = append(partition1, node)
			} else {
				partition2 = append(partition2, node)
			}
		}
		return []api.NodePartition{partition1, partition2}, nil
	}

	ssn := &Session{}

	ssn.AddNodePartitionFn(evenPartition)
	ssn.AddNodePartitionFn(ingPartition)

	partitions, _ := ssn.NodePartitionFn(nil, nil, nodes)

	assert.Equal(t, 4, len(partitions))

	assert.Equal(t, len(partitions[0]), 2)
	assert.Equal(t, partitions[0][0].Name, "node0ing")
	assert.Equal(t, partitions[0][1].Name, "node4ing")

	assert.Equal(t, len(partitions[1]), 1)
	assert.Equal(t, partitions[1][0].Name, "node2")

	assert.Equal(t, len(partitions[2]), 2)
	assert.Equal(t, partitions[2][0].Name, "node1ing")
	assert.Equal(t, partitions[2][1].Name, "node3ing")

	assert.Equal(t, len(partitions[3]), 1)
	assert.Equal(t, partitions[3][0].Name, "node5")
}
