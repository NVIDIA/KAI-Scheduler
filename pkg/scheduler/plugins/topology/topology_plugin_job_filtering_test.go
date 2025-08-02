// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package topology

import (
	"testing"

	"k8s.io/utils/ptr"
	kueuev1alpha1 "sigs.k8s.io/kueue/apis/kueue/v1alpha1"

	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/common_info"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/node_info"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/pod_status"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/framework"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/test_utils/jobs_fake"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/test_utils/nodes_fake"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/test_utils/tasks_fake"
)

func TestTopologyPlugin_calcTreeAllocatable(t *testing.T) {
	tests := []struct {
		name                       string
		job                        *jobs_fake.TestJobBasic
		allocatedPodGroups         []*jobs_fake.TestJobBasic
		nodes                      map[string]nodes_fake.TestNodeBasic
		nodesToDomains             map[string]TopologyDomainID
		setupTopologyTree          func() *TopologyInfo
		expectedMaxAllocatablePods int
		expectedDomains            map[TopologyDomainID]*TopologyDomainInfo
	}{
		{
			name: "two level topology - parent takes child values when children can allocate full job",
			job: &jobs_fake.TestJobBasic{
				Name:                "test-job",
				RequiredCPUsPerTask: 500,
				Tasks: []*tasks_fake.TestTaskBasic{
					{State: pod_status.Pending},
					{State: pod_status.Pending},
				},
			},
			nodes: map[string]nodes_fake.TestNodeBasic{
				"node-1": {
					CPUMillis:  1000,
					GPUs:       6,
					MaxTaskNum: ptr.To(100),
				},
				"node-2": {
					CPUMillis:  1000,
					GPUs:       6,
					MaxTaskNum: ptr.To(100),
				},
			},
			nodesToDomains: map[string]TopologyDomainID{
				"node-1": "rack1.zone1",
				"node-2": "rack2.zone1",
			},
			setupTopologyTree: func() *TopologyInfo {
				tree := &TopologyInfo{
					Name: "test-topology",
					TopologyResource: &kueuev1alpha1.Topology{
						Spec: kueuev1alpha1.TopologySpec{
							Levels: []kueuev1alpha1.TopologyLevel{
								{NodeLabel: "rack"},
								{NodeLabel: "zone"},
							},
						},
					},
					DomainsByLevel: map[string]map[TopologyDomainID]*TopologyDomainInfo{
						"rack": {
							"rack1.zone1": {
								ID:    "rack1.zone1",
								Name:  "rack1",
								Level: "rack",
								Nodes: map[string]*node_info.NodeInfo{},
							},
							"rack2.zone1": {
								ID:    "rack2.zone1",
								Name:  "rack2",
								Level: "rack",
								Nodes: map[string]*node_info.NodeInfo{},
							},
						},
						"zone": {
							"zone1": {
								ID:    "zone1",
								Name:  "zone1",
								Level: "zone",
								Nodes: map[string]*node_info.NodeInfo{},
							},
						},
					},
				}

				tree.Root = tree.DomainsByLevel["zone"]["zone1"]

				// Set parent relationships
				tree.DomainsByLevel["zone"]["zone1"].Children = []*TopologyDomainInfo{
					tree.DomainsByLevel["rack"]["rack1.zone1"],
					tree.DomainsByLevel["rack"]["rack2.zone1"],
				}
				tree.DomainsByLevel["rack"]["rack1.zone1"].Parent = tree.DomainsByLevel["zone"]["zone1"]
				tree.DomainsByLevel["rack"]["rack2.zone1"].Parent = tree.DomainsByLevel["zone"]["zone1"]

				return tree
			},
			expectedMaxAllocatablePods: 4,
			expectedDomains: map[TopologyDomainID]*TopologyDomainInfo{
				"rack1.zone1": {
					ID:              "rack1.zone1",
					Name:            "rack1",
					Level:           "rack",
					AllocatablePods: 2,
				},
				"rack2.zone1": {
					ID:              "rack2.zone1",
					Name:            "rack2",
					Level:           "rack",
					AllocatablePods: 2,
				},
				"zone1": {
					ID:              "zone1",
					Name:            "zone1",
					Level:           "zone",
					AllocatablePods: 4,
				},
			},
		},
		{
			name: "children cannot allocate full job individually - parent sums allocations",
			job: &jobs_fake.TestJobBasic{
				Name:                "test-job",
				RequiredCPUsPerTask: 800, // Each node can only fit 1 pod (1000/800 = 1)
				Tasks: []*tasks_fake.TestTaskBasic{
					{State: pod_status.Pending},
					{State: pod_status.Pending},
				},
			},
			nodes: map[string]nodes_fake.TestNodeBasic{
				"node-1": {
					CPUMillis:  1000,
					GPUs:       6,
					MaxTaskNum: ptr.To(100),
				},
				"node-2": {
					CPUMillis:  1000,
					GPUs:       6,
					MaxTaskNum: ptr.To(100),
				},
			},
			nodesToDomains: map[string]TopologyDomainID{
				"node-1": "rack1.zone1",
				"node-2": "rack2.zone1",
			},
			setupTopologyTree: func() *TopologyInfo {
				tree := &TopologyInfo{
					Name: "test-topology",
					TopologyResource: &kueuev1alpha1.Topology{
						Spec: kueuev1alpha1.TopologySpec{
							Levels: []kueuev1alpha1.TopologyLevel{
								{NodeLabel: "rack"},
								{NodeLabel: "zone"},
							},
						},
					},
					DomainsByLevel: map[string]map[TopologyDomainID]*TopologyDomainInfo{
						"rack": {
							"rack1.zone1": {
								ID:    "rack1.zone1",
								Name:  "rack1",
								Level: "rack",
								Nodes: map[string]*node_info.NodeInfo{},
							},
							"rack2.zone1": {
								ID:    "rack2.zone1",
								Name:  "rack2",
								Level: "rack",
								Nodes: map[string]*node_info.NodeInfo{},
							},
						},
						"zone": {
							"zone1": {
								ID:    "zone1",
								Name:  "zone1",
								Level: "zone",
								Nodes: map[string]*node_info.NodeInfo{},
							},
						},
					},
				}

				tree.Root = tree.DomainsByLevel["zone"]["zone1"]

				// Set parent relationships
				tree.DomainsByLevel["zone"]["zone1"].Children = []*TopologyDomainInfo{
					tree.DomainsByLevel["rack"]["rack1.zone1"],
					tree.DomainsByLevel["rack"]["rack2.zone1"],
				}
				tree.DomainsByLevel["rack"]["rack1.zone1"].Parent = tree.DomainsByLevel["zone"]["zone1"]
				tree.DomainsByLevel["rack"]["rack2.zone1"].Parent = tree.DomainsByLevel["zone"]["zone1"]

				return tree
			},
			expectedMaxAllocatablePods: 2,
			expectedDomains: map[TopologyDomainID]*TopologyDomainInfo{
				"rack1.zone1": {
					ID:              "rack1.zone1",
					Name:            "rack1",
					Level:           "rack",
					AllocatablePods: 1, // Can only fit 1 pod
				},
				"rack2.zone1": {
					ID:              "rack2.zone1",
					Name:            "rack2",
					Level:           "rack",
					AllocatablePods: 1, // Can only fit 1 pod
				},
				"zone1": {
					ID:              "zone1",
					Name:            "zone1",
					Level:           "zone",
					AllocatablePods: 2, // Sum of children allocations: 1 + 1
				},
			},
		},
		{
			name: "mixed distances - parent takes minimum distance",
			job: &jobs_fake.TestJobBasic{
				Name:                "test-job",
				RequiredCPUsPerTask: 500,
				Tasks: []*tasks_fake.TestTaskBasic{
					{State: pod_status.Pending},
					{State: pod_status.Pending},
				},
			},
			nodes: map[string]nodes_fake.TestNodeBasic{
				"node-1": {
					CPUMillis:  500,
					GPUs:       6,
					MaxTaskNum: ptr.To(100),
				},
				"node-2": {
					CPUMillis:  500,
					GPUs:       6,
					MaxTaskNum: ptr.To(100),
				},
				"node-3": {
					CPUMillis:  1000,
					GPUs:       6,
					MaxTaskNum: ptr.To(100),
				},
			},
			nodesToDomains: map[string]TopologyDomainID{
				"node-1": "rack1.zone1",
				"node-2": "rack1.zone1",
				"node-3": "rack2.zone1",
			},
			setupTopologyTree: func() *TopologyInfo {
				tree := &TopologyInfo{
					Name: "test-topology",
					TopologyResource: &kueuev1alpha1.Topology{
						Spec: kueuev1alpha1.TopologySpec{
							Levels: []kueuev1alpha1.TopologyLevel{
								{NodeLabel: "rack"},
								{NodeLabel: "zone"},
							},
						},
					},
					DomainsByLevel: map[string]map[TopologyDomainID]*TopologyDomainInfo{
						"rack": {
							"rack1.zone1": {
								ID:    "rack1.zone1",
								Name:  "rack1",
								Level: "rack",
								Nodes: map[string]*node_info.NodeInfo{},
							},
							"rack2.zone1": {
								ID:    "rack2.zone1",
								Name:  "rack2",
								Level: "rack",
								Nodes: map[string]*node_info.NodeInfo{},
							},
						},
						"zone": {
							"zone1": {
								ID:    "zone1",
								Name:  "zone1",
								Level: "zone",
								Nodes: map[string]*node_info.NodeInfo{},
							},
						},
					},
				}

				tree.Root = tree.DomainsByLevel["zone"]["zone1"]

				// Set parent relationships
				tree.DomainsByLevel["zone"]["zone1"].Children = []*TopologyDomainInfo{
					tree.DomainsByLevel["rack"]["rack1.zone1"],
					tree.DomainsByLevel["rack"]["rack2.zone1"],
				}
				tree.DomainsByLevel["rack"]["rack1.zone1"].Parent = tree.DomainsByLevel["zone"]["zone1"]
				tree.DomainsByLevel["rack"]["rack2.zone1"].Parent = tree.DomainsByLevel["zone"]["zone1"]

				return tree
			},
			expectedMaxAllocatablePods: 4,
			expectedDomains: map[TopologyDomainID]*TopologyDomainInfo{
				"rack1.zone1": {
					ID:              "rack1.zone1",
					Name:            "rack1",
					Level:           "rack",
					AllocatablePods: 2,
				},
				"rack2.zone1": {
					ID:              "rack2.zone1",
					Name:            "rack2",
					Level:           "rack",
					AllocatablePods: 2,
				},
				"zone1": {
					ID:              "zone1",
					Name:            "zone1",
					Level:           "zone",
					AllocatablePods: 4,
				},
			},
		},
		{
			name: "no leaf domains - no allocateable domains",
			job: &jobs_fake.TestJobBasic{
				Name:                "test-job",
				RequiredCPUsPerTask: 2000, // Too much for any node
				Tasks: []*tasks_fake.TestTaskBasic{
					{State: pod_status.Pending},
				},
			},
			nodes: map[string]nodes_fake.TestNodeBasic{
				"node-1": {
					CPUMillis:  1000,
					GPUs:       6,
					MaxTaskNum: ptr.To(100),
				},
			},
			nodesToDomains: map[string]TopologyDomainID{
				"node-1": "zone1",
			},
			setupTopologyTree: func() *TopologyInfo {
				tree := &TopologyInfo{
					Name: "test-topology",
					TopologyResource: &kueuev1alpha1.Topology{
						Spec: kueuev1alpha1.TopologySpec{
							Levels: []kueuev1alpha1.TopologyLevel{
								{NodeLabel: "zone"},
							},
						},
					},
					DomainsByLevel: map[string]map[TopologyDomainID]*TopologyDomainInfo{
						"zone": {
							"zone1": {
								ID:    "zone1",
								Name:  "zone1",
								Level: "zone",
								Nodes: map[string]*node_info.NodeInfo{},
							},
						},
					},
				}

				tree.Root = tree.DomainsByLevel["zone"]["zone1"]

				return tree
			},
			expectedMaxAllocatablePods: 0,
			expectedDomains:            map[TopologyDomainID]*TopologyDomainInfo{
				// No domains should have allocations since no nodes can accommodate the job
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			jobName := tt.job.Name
			clusterPodGroups := append(tt.allocatedPodGroups, tt.job)
			jobsInfoMap, tasksToNodeMap, _ := jobs_fake.BuildJobsAndTasksMaps(clusterPodGroups)
			nodesInfoMap := nodes_fake.BuildNodesInfoMap(tt.nodes, tasksToNodeMap)
			job := jobsInfoMap[common_info.PodGroupID(jobName)]

			topologyTree := tt.setupTopologyTree()
			for nodeName, domainId := range tt.nodesToDomains {
				nodeInfo := nodesInfoMap[nodeName]
				domain := topologyTree.DomainsByLevel[topologyTree.TopologyResource.Spec.Levels[0].NodeLabel][domainId]
				for domain != nil {
					if nodeInfo.Node.Labels == nil {
						nodeInfo.Node.Labels = map[string]string{
							domain.Level: domain.Name,
						}
					} else {
						nodeInfo.Node.Labels[domain.Level] = domain.Name
					}
					domain.AddNode(nodeInfo)
					domain = domain.Parent
				}
			}

			session := &framework.Session{
				Nodes:         nodesInfoMap,
				PodGroupInfos: jobsInfoMap,
				Topologies:    []*kueuev1alpha1.Topology{topologyTree.TopologyResource},
			}
			plugin := &topologyPlugin{
				sessionStateGetter: session,
				nodesInfos:         nodesInfoMap,
			}

			// Call the function under test
			maxAllocatablePods, err := plugin.calcTreeAllocatable(job, topologyTree)
			if err != nil {
				t.Errorf("failed to calc tree allocatable. job: %s, error: %v", job.PodGroup.Name, err)
			}

			// Assert
			if maxAllocatablePods != tt.expectedMaxAllocatablePods {
				t.Errorf("expected max allocatable pods %d, got %d", tt.expectedMaxAllocatablePods, maxAllocatablePods)
			}

			if len(tt.expectedDomains) == 0 {
				// Check that no domains have allocations
				for _, levelDomains := range topologyTree.DomainsByLevel {
					for _, domain := range levelDomains {
						if domain.AllocatablePods != 0 {
							t.Errorf("expected domain %s to have 0 AllocatablePods, got %d",
								domain.ID, domain.AllocatablePods)
						}
					}
				}
				return
			}

			for domainID, expectedDomain := range tt.expectedDomains {
				actualDomain, exists := topologyTree.DomainsByLevel[expectedDomain.Level][domainID]
				if !exists {
					t.Errorf("expected domain %s not found", domainID)
					continue
				}
				if actualDomain.AllocatablePods != expectedDomain.AllocatablePods {
					t.Errorf("domain %s: expected AllocatablePods %d, got %d",
						domainID, expectedDomain.AllocatablePods, actualDomain.AllocatablePods)
				}
			}
		})
	}
}
