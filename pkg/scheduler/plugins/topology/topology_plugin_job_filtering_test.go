// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package topology

import (
	"testing"

	"k8s.io/utils/ptr"
	kueuev1alpha1 "sigs.k8s.io/kueue/apis/kueue/v1alpha1"

	enginev2alpha2 "github.com/NVIDIA/KAI-scheduler/pkg/apis/scheduling/v2alpha2"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/common_info"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/node_info"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/pod_status"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/podgroup_info"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/resource_info"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/framework"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/test_utils/jobs_fake"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/test_utils/nodes_fake"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/test_utils/tasks_fake"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
				"node-2": {
					CPUMillis:  1000,
					GPUs:       6,
					MaxTaskNum: ptr.To(100),
				},
			},
			nodesToDomains: map[string]TopologyDomainID{
				"node-1": "zone1",
				"node-2": "zone1",
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
					Distance:        1,
					AllocatablePods: 1,
				},
			},
		},
		{
			name: "two nodes with two pods job allocateable, single domain",
			job: &jobs_fake.TestJobBasic{
				Name:                "test-job",
				RequiredCPUsPerTask: 500,
				Tasks: []*tasks_fake.TestTaskBasic{
					{State: pod_status.Pending},
					{State: pod_status.Pending},
				},
			},
			allocatedPodGroups: []*jobs_fake.TestJobBasic{},
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
			},
			nodesToDomains: map[string]TopologyDomainID{
				"node-1": "zone1",
				"node-2": "zone1",
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
			expectedCount: 2,
			expectedDomains: map[TopologyDomainID]*TopologyDomainInfo{
				"zone1": {
					ID:              "zone1",
					Name:            "zone1",
					Level:           "zone",
					Distance:        2,
					AllocatablePods: 2,
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
					Distance:        1,
					AllocatablePods: 2,
				},
				"rack2.zone2": {
					ID:              "rack2.zone2",
					Name:            "rack2",
					Level:           "rack",
					Distance:        1,
					AllocatablePods: 1,
				},
				"rack3.zone2": {
					ID:              "rack3.zone2",
					Name:            "rack3",
					Level:           "rack",
					Distance:        1,
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

func TestTopologyPlugin_calculateRelevantDomainLevels(t *testing.T) {
	tests := []struct {
		name            string
		job             *podgroup_info.PodGroupInfo
		jobTopologyName string
		topologyTree    *TopologyInfo
		expectedLevels  map[string]bool
		expectedError   string
	}{
		{
			name: "both required and preferred placement specified",
			job: &podgroup_info.PodGroupInfo{
				Name: "test-job",
				PodGroup: &enginev2alpha2.PodGroup{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-job",
						Annotations: map[string]string{
							topologyRequiredPlacementAnnotationKey:  "zone",
							topologyPreferredPlacementAnnotationKey: "rack",
						},
					},
				},
			},
			jobTopologyName: "test-topology",
			topologyTree: &TopologyInfo{
				Name: "test-topology",
				TopologyResource: &kueuev1alpha1.Topology{
					Spec: kueuev1alpha1.TopologySpec{
						Levels: []kueuev1alpha1.TopologyLevel{
							{NodeLabel: "rack"},
							{NodeLabel: "zone"},
							{NodeLabel: "datacenter"},
						},
					},
				},
			},
			expectedLevels: map[string]bool{
				"rack": true,
				"zone": true,
			},
			expectedError: "",
		},
		{
			name: "only required placement specified",
			job: &podgroup_info.PodGroupInfo{
				Name: "test-job",
				PodGroup: &enginev2alpha2.PodGroup{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-job",
						Annotations: map[string]string{
							topologyRequiredPlacementAnnotationKey: "zone",
						},
					},
				},
			},
			jobTopologyName: "test-topology",
			topologyTree: &TopologyInfo{
				Name: "test-topology",
				TopologyResource: &kueuev1alpha1.Topology{
					Spec: kueuev1alpha1.TopologySpec{
						Levels: []kueuev1alpha1.TopologyLevel{
							{NodeLabel: "rack"},
							{NodeLabel: "zone"},
							{NodeLabel: "datacenter"},
						},
					},
				},
			},
			expectedLevels: map[string]bool{
				"rack": true,
				"zone": true,
			},
			expectedError: "",
		},
		{
			name: "only preferred placement specified",
			job: &podgroup_info.PodGroupInfo{
				Name: "test-job",
				PodGroup: &enginev2alpha2.PodGroup{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-job",
						Annotations: map[string]string{
							topologyPreferredPlacementAnnotationKey: "rack",
						},
					},
				},
			},
			jobTopologyName: "test-topology",
			topologyTree: &TopologyInfo{
				Name: "test-topology",
				TopologyResource: &kueuev1alpha1.Topology{
					Spec: kueuev1alpha1.TopologySpec{
						Levels: []kueuev1alpha1.TopologyLevel{
							{NodeLabel: "rack"},
							{NodeLabel: "zone"},
							{NodeLabel: "datacenter"},
						},
					},
				},
			},
			expectedLevels: map[string]bool{
				"rack":       true,
				"zone":       true,
				"datacenter": true,
			},
			expectedError: "",
		},
		{
			name: "no placement annotations specified",
			job: &podgroup_info.PodGroupInfo{
				Name: "test-job",
				PodGroup: &enginev2alpha2.PodGroup{
					ObjectMeta: metav1.ObjectMeta{
						Name:        "test-job",
						Annotations: map[string]string{},
					},
				},
			},
			jobTopologyName: "test-topology",
			topologyTree: &TopologyInfo{
				Name: "test-topology",
				TopologyResource: &kueuev1alpha1.Topology{
					Spec: kueuev1alpha1.TopologySpec{
						Levels: []kueuev1alpha1.TopologyLevel{
							{NodeLabel: "rack"},
							{NodeLabel: "zone"},
						},
					},
				},
			},
			expectedLevels: nil,
			expectedError:  "no topology placement annotations found for job test-job, workload topology name: test-topology",
		},
		{
			name: "required placement not found in topology",
			job: &podgroup_info.PodGroupInfo{
				Name: "test-job",
				PodGroup: &enginev2alpha2.PodGroup{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-job",
						Annotations: map[string]string{
							topologyRequiredPlacementAnnotationKey: "nonexistent",
						},
					},
				},
			},
			jobTopologyName: "test-topology",
			topologyTree: &TopologyInfo{
				Name: "test-topology",
				TopologyResource: &kueuev1alpha1.Topology{
					Spec: kueuev1alpha1.TopologySpec{
						Levels: []kueuev1alpha1.TopologyLevel{
							{NodeLabel: "rack"},
							{NodeLabel: "zone"},
						},
					},
				},
			},
			expectedLevels: nil,
			expectedError:  "the topology test-topology doesn't have a level matching the required(nonexistent) spesified for the job test-job",
		},
		{
			name: "preferred placement not found in topology",
			job: &podgroup_info.PodGroupInfo{
				Name: "test-job",
				PodGroup: &enginev2alpha2.PodGroup{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-job",
						Annotations: map[string]string{
							topologyPreferredPlacementAnnotationKey: "nonexistent",
						},
					},
				},
			},
			jobTopologyName: "test-topology",
			topologyTree: &TopologyInfo{
				Name: "test-topology",
				TopologyResource: &kueuev1alpha1.Topology{
					Spec: kueuev1alpha1.TopologySpec{
						Levels: []kueuev1alpha1.TopologyLevel{
							{NodeLabel: "rack"},
							{NodeLabel: "zone"},
						},
					},
				},
			},
			expectedLevels: nil,
			expectedError:  "the topology test-topology doesn't have a level matching the preffered(nonexistent) spesified for the job test-job",
		},
		{
			name: "required placement at first level",
			job: &podgroup_info.PodGroupInfo{
				Name: "test-job",
				PodGroup: &enginev2alpha2.PodGroup{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-job",
						Annotations: map[string]string{
							topologyRequiredPlacementAnnotationKey: "rack",
						},
					},
				},
			},
			jobTopologyName: "test-topology",
			topologyTree: &TopologyInfo{
				Name: "test-topology",
				TopologyResource: &kueuev1alpha1.Topology{
					Spec: kueuev1alpha1.TopologySpec{
						Levels: []kueuev1alpha1.TopologyLevel{
							{NodeLabel: "rack"},
							{NodeLabel: "zone"},
							{NodeLabel: "datacenter"},
						},
					},
				},
			},
			expectedLevels: map[string]bool{
				"rack": true,
			},
			expectedError: "",
		},
		{
			name: "preferred placement at first level",
			job: &podgroup_info.PodGroupInfo{
				Name: "test-job",
				PodGroup: &enginev2alpha2.PodGroup{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-job",
						Annotations: map[string]string{
							topologyPreferredPlacementAnnotationKey: "rack",
						},
					},
				},
			},
			jobTopologyName: "test-topology",
			topologyTree: &TopologyInfo{
				Name: "test-topology",
				TopologyResource: &kueuev1alpha1.Topology{
					Spec: kueuev1alpha1.TopologySpec{
						Levels: []kueuev1alpha1.TopologyLevel{
							{NodeLabel: "rack"},
							{NodeLabel: "zone"},
							{NodeLabel: "datacenter"},
						},
					},
				},
			},
			expectedLevels: map[string]bool{
				"rack":       true,
				"zone":       true,
				"datacenter": true,
			},
			expectedError: "",
		},
		{
			name: "preferred placement at middle level",
			job: &podgroup_info.PodGroupInfo{
				Name: "test-job",
				PodGroup: &enginev2alpha2.PodGroup{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-job",
						Annotations: map[string]string{
							topologyPreferredPlacementAnnotationKey: "zone",
						},
					},
				},
			},
			jobTopologyName: "test-topology",
			topologyTree: &TopologyInfo{
				Name: "test-topology",
				TopologyResource: &kueuev1alpha1.Topology{
					Spec: kueuev1alpha1.TopologySpec{
						Levels: []kueuev1alpha1.TopologyLevel{
							{NodeLabel: "rack"},
							{NodeLabel: "zone"},
							{NodeLabel: "datacenter"},
						},
					},
				},
			},
			expectedLevels: map[string]bool{
				"zone":       true,
				"datacenter": true,
			},
			expectedError: "",
		},
		{
			name: "single level topology with preferred placement",
			job: &podgroup_info.PodGroupInfo{
				Name: "test-job",
				PodGroup: &enginev2alpha2.PodGroup{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-job",
						Annotations: map[string]string{
							topologyPreferredPlacementAnnotationKey: "zone",
						},
					},
				},
			},
			jobTopologyName: "test-topology",
			topologyTree: &TopologyInfo{
				Name: "test-topology",
				TopologyResource: &kueuev1alpha1.Topology{
					Spec: kueuev1alpha1.TopologySpec{
						Levels: []kueuev1alpha1.TopologyLevel{
							{NodeLabel: "zone"},
						},
					},
				},
			},
			expectedLevels: map[string]bool{
				"zone": true,
			},
			expectedError: "",
		},
		{
			name: "complex topology with multiple levels",
			job: &podgroup_info.PodGroupInfo{
				Name: "test-job",
				PodGroup: &enginev2alpha2.PodGroup{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-job",
						Annotations: map[string]string{
							topologyRequiredPlacementAnnotationKey:  "region",
							topologyPreferredPlacementAnnotationKey: "zone",
						},
					},
				},
			},
			jobTopologyName: "test-topology",
			topologyTree: &TopologyInfo{
				Name: "test-topology",
				TopologyResource: &kueuev1alpha1.Topology{
					Spec: kueuev1alpha1.TopologySpec{
						Levels: []kueuev1alpha1.TopologyLevel{
							{NodeLabel: "rack"},
							{NodeLabel: "zone"},
							{NodeLabel: "region"},
							{NodeLabel: "datacenter"},
						},
					},
				},
			},
			expectedLevels: map[string]bool{
				"zone":   true,
				"region": true,
			},
			expectedError: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			plugin := &topologyPlugin{}

			result, err := plugin.calculateRelevantDomainLevels(tt.job, tt.jobTopologyName, tt.topologyTree)

			// Check error
			if tt.expectedError != "" {
				if err == nil {
					t.Errorf("expected error '%s', but got nil", tt.expectedError)
					return
				}
				if err.Error() != tt.expectedError {
					t.Errorf("expected error '%s', but got '%s'", tt.expectedError, err.Error())
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			// Check result
			if tt.expectedLevels == nil {
				if result != nil {
					t.Errorf("expected nil result, but got %v", result)
				}
				return
			}

			if result == nil {
				t.Errorf("expected result %v, but got nil", tt.expectedLevels)
				return
			}

			// Compare maps
			if len(result) != len(tt.expectedLevels) {
				t.Errorf("expected %d levels, but got %d", len(tt.expectedLevels), len(result))
			}

			for level, expected := range tt.expectedLevels {
				if actual, exists := result[level]; !exists {
					t.Errorf("expected level '%s' not found in result", level)
				} else if actual != expected {
					t.Errorf("for level '%s': expected %v, but got %v", level, expected, actual)
				}
			}

			// Check for unexpected levels in result
			for level := range result {
				if _, exists := tt.expectedLevels[level]; !exists {
					t.Errorf("unexpected level '%s' found in result", level)
				}
			}
		})
	}
}

func TestTopologyPlugin_calcTreeAlocationData(t *testing.T) {
	tests := []struct {
		name               string
		job                *jobs_fake.TestJobBasic
		allocatedPodGroups []*jobs_fake.TestJobBasic
		nodes              map[string]nodes_fake.TestNodeBasic
		nodesToDomains     map[string]TopologyDomainID
		setupTopologyTree  func() *TopologyInfo
		expectedDomains    map[TopologyDomainID]*TopologyDomainInfo
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
					Domains: map[TopologyDomainID]*TopologyDomainInfo{
						"rack1.zone1": {
							ID:                 "rack1.zone1",
							Name:               "rack1",
							Level:              "rack",
							AvailableResources: resource_info.NewResource(0, 0, 0),
							AllocatedResources: resource_info.NewResource(0, 0, 0),
							Nodes:              map[string]*node_info.NodeInfo{},
						},
						"rack2.zone1": {
							ID:                 "rack2.zone1",
							Name:               "rack2",
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
					},
				}

				// Set parent relationships
				tree.Domains["rack1.zone1"].Parent = tree.Domains["zone1"]
				tree.Domains["rack2.zone1"].Parent = tree.Domains["zone1"]

				return tree
			},
			expectedDomains: map[TopologyDomainID]*TopologyDomainInfo{
				"rack1.zone1": {
					ID:              "rack1.zone1",
					Name:            "rack1",
					Level:           "rack",
					Distance:        1,
					AllocatablePods: 2,
				},
				"rack2.zone1": {
					ID:              "rack2.zone1",
					Name:            "rack2",
					Level:           "rack",
					Distance:        1,
					AllocatablePods: 2,
				},
				"zone1": {
					ID:              "zone1",
					Name:            "zone1",
					Level:           "zone",
					Distance:        1,
					AllocatablePods: 2, // Takes child's AllocatablePods, up to full job
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
					Domains: map[TopologyDomainID]*TopologyDomainInfo{
						"rack1.zone1": {
							ID:                 "rack1.zone1",
							Name:               "rack1",
							Level:              "rack",
							AvailableResources: resource_info.NewResource(0, 0, 0),
							AllocatedResources: resource_info.NewResource(0, 0, 0),
							Nodes:              map[string]*node_info.NodeInfo{},
						},
						"rack2.zone1": {
							ID:                 "rack2.zone1",
							Name:               "rack2",
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
					},
				}

				// Set parent relationships
				tree.Domains["rack1.zone1"].Parent = tree.Domains["zone1"]
				tree.Domains["rack2.zone1"].Parent = tree.Domains["zone1"]

				return tree
			},
			expectedDomains: map[TopologyDomainID]*TopologyDomainInfo{
				"rack1.zone1": {
					ID:              "rack1.zone1",
					Name:            "rack1",
					Level:           "rack",
					Distance:        1,
					AllocatablePods: 1, // Can only fit 1 pod
				},
				"rack2.zone1": {
					ID:              "rack2.zone1",
					Name:            "rack2",
					Level:           "rack",
					Distance:        1,
					AllocatablePods: 1, // Can only fit 1 pod
				},
				"zone1": {
					ID:              "zone1",
					Name:            "zone1",
					Level:           "zone",
					Distance:        2,
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
					Domains: map[TopologyDomainID]*TopologyDomainInfo{
						"rack1.zone1": {
							ID:                 "rack1.zone1",
							Name:               "rack1",
							Level:              "rack",
							AvailableResources: resource_info.NewResource(0, 0, 0),
							AllocatedResources: resource_info.NewResource(0, 0, 0),
							Nodes:              map[string]*node_info.NodeInfo{},
						},
						"rack2.zone1": {
							ID:                 "rack2.zone1",
							Name:               "rack2",
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
					},
				}

				// Set parent relationships
				tree.Domains["rack1.zone1"].Parent = tree.Domains["zone1"]
				tree.Domains["rack2.zone1"].Parent = tree.Domains["zone1"]

				return tree
			},
			expectedDomains: map[TopologyDomainID]*TopologyDomainInfo{
				"rack1.zone1": {
					ID:              "rack1.zone1",
					Name:            "rack1",
					Level:           "rack",
					Distance:        2,
					AllocatablePods: 2,
				},
				"rack2.zone1": {
					ID:              "rack2.zone1",
					Name:            "rack2",
					Level:           "rack",
					Distance:        1,
					AllocatablePods: 2,
				},
				"zone1": {
					ID:              "zone1",
					Name:            "zone1",
					Level:           "zone",
					Distance:        1, // Takes minimum distance from children (0 from rack2)
					AllocatablePods: 2,
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
			expectedDomains: map[TopologyDomainID]*TopologyDomainInfo{
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

			// Call the function under test
			plugin.calcTreeAlocationData(job, topologyTree)

			// Assert
			if len(tt.expectedDomains) == 0 {
				// Check that no domains have allocations
				for _, domain := range topologyTree.Domains {
					if domain.AllocatablePods != 0 {
						t.Errorf("expected domain %s to have 0 AllocatablePods, got %d",
							domain.ID, domain.AllocatablePods)
					}
					if domain.Distance != 0 {
						t.Errorf("expected domain %s to have 0 Distance, got %d",
							domain.ID, domain.Distance)
					}
				}
				return
			}

			for domainID, expectedDomain := range tt.expectedDomains {
				actualDomain, exists := topologyTree.Domains[domainID]
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
