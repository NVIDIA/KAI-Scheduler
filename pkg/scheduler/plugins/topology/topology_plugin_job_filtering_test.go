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
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/resource_info"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/framework"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/test_utils/jobs_fake"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/test_utils/nodes_fake"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/test_utils/tasks_fake"
)

func TestTopologyPlugin_calcAllocationsForLeafDomains(t *testing.T) {
	tests := []struct {
		name               string
		job                *jobs_fake.TestJobBasic
		allocatedPodGroups []*jobs_fake.TestJobBasic
		nodes              map[string]nodes_fake.TestNodeBasic
		nodesToDomains     map[string]TopologyDomainID
		setupTopologyTree  func() *TopologyInfo
		expectedCount      int
		expectedDomains    map[TopologyDomainID]*TopologyDomainInfo
	}{
		{
			name: "single node with single pod job allocateable",
			job: &jobs_fake.TestJobBasic{
				Name:                "test-job",
				RequiredCPUsPerTask: 500,
				Tasks: []*tasks_fake.TestTaskBasic{
					{State: pod_status.Pending},
				},
			},
			allocatedPodGroups: []*jobs_fake.TestJobBasic{{
				Name:                "test-job",
				RequiredCPUsPerTask: 500,
				Tasks: []*tasks_fake.TestTaskBasic{
					{State: pod_status.Running, NodeName: "node-1"},
				},
			}},
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
				return &TopologyInfo{
					Name: "test-topology",
					TopologyResource: &kueuev1alpha1.Topology{
						Spec: kueuev1alpha1.TopologySpec{
							Levels: []kueuev1alpha1.TopologyLevel{
								{NodeLabel: "zone"},
							},
						},
					},
					Domains: map[TopologyDomainID]*TopologyDomainInfo{
						"zone1": {
							ID:                 "zone1",
							Name:               "zone1",
							Level:              "zone",
							AvailableResources: resource_info.NewResource(0, 0, 0),
							AllocatedResources: resource_info.NewResource(0, 0, 0),
							Nodes:              map[string]*node_info.NodeInfo{},
						},
					},
				}
			},
			expectedCount: 1,
			expectedDomains: map[TopologyDomainID]*TopologyDomainInfo{
				"zone1": {
					ID:              "zone1",
					Name:            "zone1",
					Level:           "zone",
					Distance:        0, // Single node domain has distance 0
					AllocatablePods: 1,
				},
			},
		},
		{
			name: "3 allocateable domains, 1 not allocateable.",
			job: &jobs_fake.TestJobBasic{
				Name:                "test-job",
				RequiredCPUsPerTask: 1000,
				Tasks: []*tasks_fake.TestTaskBasic{
					{State: pod_status.Pending},
					{State: pod_status.Pending},
				},
			},
			nodes: map[string]nodes_fake.TestNodeBasic{
				"node-1": {
					CPUMillis:  2000,
					GPUs:       6,
					MaxTaskNum: ptr.To(100),
				},
				"node-2": {
					CPUMillis:  2000,
					GPUs:       6,
					MaxTaskNum: ptr.To(100),
				},
				"node-3": {
					CPUMillis:  2000,
					GPUs:       6,
					MaxTaskNum: ptr.To(100),
				},
			},
			nodesToDomains: map[string]TopologyDomainID{
				"node-1": "rack1.zone1",
				"node-2": "rack2.zone2",
				"node-3": "rack3.zone2",
			},
			allocatedPodGroups: []*jobs_fake.TestJobBasic{
				{
					Name:                "test-job-2",
					RequiredCPUsPerTask: 1000,
					Tasks: []*tasks_fake.TestTaskBasic{
						{State: pod_status.Running, NodeName: "node-2"},
					},
				},
				{
					Name:                "test-job-3",
					RequiredCPUsPerTask: 1000,
					Tasks: []*tasks_fake.TestTaskBasic{
						{State: pod_status.Running, NodeName: "node-3"},
					},
				},
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
					Domains: map[TopologyDomainID]*TopologyDomainInfo{
						"rack1.zone1": {
							ID:                 "rack1.zone1",
							Name:               "rack1",
							Level:              "rack",
							AvailableResources: resource_info.NewResource(0, 0, 0),
							AllocatedResources: resource_info.NewResource(0, 0, 0),
							Nodes:              map[string]*node_info.NodeInfo{},
						},
						"rack2.zone2": {
							ID:                 "rack2.zone2",
							Name:               "rack2",
							Level:              "rack",
							AvailableResources: resource_info.NewResource(0, 0, 0),
							AllocatedResources: resource_info.NewResource(0, 0, 0),
							Nodes:              map[string]*node_info.NodeInfo{},
						},
						"rack3.zone2": {
							ID:                 "rack3.zone2",
							Name:               "rack3",
							Level:              "rack",
							AvailableResources: resource_info.NewResource(0, 0, 0),
							AllocatedResources: resource_info.NewResource(0, 0, 0),
							Nodes:              map[string]*node_info.NodeInfo{},
						},
						"zone1": {
							ID:                 "zone1",
							Name:               "zone1",
							Level:              "zone",
							AvailableResources: resource_info.NewResource(0, 0, 0),
							AllocatedResources: resource_info.NewResource(0, 0, 0),
							Nodes:              map[string]*node_info.NodeInfo{},
						},
						"zone2": {
							ID:                 "zone2",
							Name:               "zone2",
							Level:              "zone",
							AvailableResources: resource_info.NewResource(0, 0, 0),
							AllocatedResources: resource_info.NewResource(0, 0, 0),
							Nodes:              map[string]*node_info.NodeInfo{},
						},
					},
				}

				tree.Domains["rack1.zone1"].Parent = tree.Domains["zone1"]
				tree.Domains["rack2.zone2"].Parent = tree.Domains["zone2"]
				tree.Domains["rack3.zone2"].Parent = tree.Domains["zone2"]

				return tree
			},
			expectedCount: 2,
			expectedDomains: map[TopologyDomainID]*TopologyDomainInfo{
				"rack1.zone1": {
					ID:              "rack1.zone1",
					Name:            "rack1",
					Level:           "rack",
					Distance:        0,
					AllocatablePods: 2,
				},
				"rack2.zone2": {
					ID:              "rack2.zone2",
					Name:            "rack2",
					Level:           "rack",
					Distance:        0,
					AllocatablePods: 1,
				},
				"rack3.zone2": {
					ID:              "rack3.zone2",
					Name:            "rack3",
					Level:           "rack",
					Distance:        0,
					AllocatablePods: 1,
				},
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
				domain := topologyTree.Domains[domainId]
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
				nodesInfoMap:       nodesInfoMap,
			}

			count, domains := plugin.calcAllocationsForLeafDomains(job, topologyTree)

			// Assert
			if count != tt.expectedCount {
				t.Errorf("expected count %d, got %d", tt.expectedCount, count)
			}

			if len(domains) != len(tt.expectedDomains) {
				t.Errorf("expected %d domains, got %d", len(tt.expectedDomains), len(domains))
			}

			for domainID, expectedDomain := range tt.expectedDomains {
				actualDomain, exists := domains[domainID]
				if !exists {
					t.Errorf("expected domain %s not found", domainID)
					continue
				}

				if actualDomain.Distance != expectedDomain.Distance {
					t.Errorf("domain %s: expected Distance %d, got %d",
						domainID, expectedDomain.Distance, actualDomain.Distance)
				}
				if actualDomain.AllocatablePods != expectedDomain.AllocatablePods {
					t.Errorf("domain %s: expected AllocatablePods %d, got %d",
						domainID, expectedDomain.AllocatablePods, actualDomain.AllocatablePods)
				}
			}
		})
	}
}
