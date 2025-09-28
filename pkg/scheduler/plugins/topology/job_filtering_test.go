// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package topology

import (
	"slices"
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
	"golang.org/x/exp/maps"
	v1 "k8s.io/api/core/v1"
	k8sframework "k8s.io/kubernetes/pkg/scheduler/framework"
	"k8s.io/utils/ptr"
	kueuev1alpha1 "sigs.k8s.io/kueue/apis/kueue/v1alpha1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	enginev2alpha2 "github.com/NVIDIA/KAI-scheduler/pkg/apis/scheduling/v2alpha2"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/common_info"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/node_info"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/pod_info"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/pod_status"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/podgroup_info"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/podgroup_info/subgroup_info"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/framework"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/k8s_internal"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/test_utils/jobs_fake"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/test_utils/nodes_fake"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/test_utils/tasks_fake"
)

// Mock session state provider for testing
type mockSessionStateProvider struct {
	states                   map[types.UID]*k8sframework.CycleState
	GetSessionStateCallCount int
}

func newMockSessionStateProvider() *mockSessionStateProvider {
	return &mockSessionStateProvider{
		states:                   make(map[types.UID]*k8sframework.CycleState),
		GetSessionStateCallCount: 0,
	}
}

func (m *mockSessionStateProvider) GetSessionStateForResource(uid types.UID) k8s_internal.SessionState {
	m.GetSessionStateCallCount++
	if state, exists := m.states[uid]; exists {
		return k8s_internal.SessionState(state)
	}
	state := k8sframework.NewCycleState()
	m.states[uid] = state
	return k8s_internal.SessionState(state)
}

func TestTopologyPlugin_subsetNodesFn(t *testing.T) {
	tests := []struct {
		name                  string
		job                   *jobs_fake.TestJobBasic
		jobTopologyConstraint *enginev2alpha2.TopologyConstraint
		allocatedPodGroups    []*jobs_fake.TestJobBasic
		nodes                 map[string]nodes_fake.TestNodeBasic
		nodesToDomains        map[string]DomainID
		setupTopologyTree     func() *Info
		setupSessionState     func(provider *mockSessionStateProvider, jobUID types.UID)
		domainParent          map[DomainID]DomainID
		domainLevel           map[DomainID]DomainLevel
		expectedError         string
		expectedNodes         map[string]bool
	}{
		{
			name: "successful topology allocation - right nodes",
			job: &jobs_fake.TestJobBasic{
				Name:                "test-job",
				RequiredCPUsPerTask: 500,
				Tasks: []*tasks_fake.TestTaskBasic{
					{State: pod_status.Pending},
					{State: pod_status.Pending},
				},
			},
			jobTopologyConstraint: &enginev2alpha2.TopologyConstraint{
				Topology:               "test-topology",
				RequiredTopologyLevel:  "zone",
				PreferredTopologyLevel: "rack",
			},
			nodes: map[string]nodes_fake.TestNodeBasic{
				"node-1": {
					CPUMillis:  1000,
					GPUs:       6,
					MaxTaskNum: ptr.To(100),
				},
				"node-2": {
					CPUMillis:  400,
					GPUs:       6,
					MaxTaskNum: ptr.To(100),
				},
			},
			nodesToDomains: map[string]DomainID{
				"node-1": "rack1.zone1",
				"node-2": "rack2.zone1",
			},
			setupTopologyTree: func() *Info {
				tree := &Info{
					Name: "test-topology",
					TopologyResource: &kueuev1alpha1.Topology{
						Spec: kueuev1alpha1.TopologySpec{
							Levels: []kueuev1alpha1.TopologyLevel{
								{NodeLabel: "zone"},
								{NodeLabel: "rack"},
							},
						},
					},
					DomainsByLevel: map[DomainLevel]LevelDomainInfos{
						"rack": {
							"rack1.zone1": {
								ID:    "rack1.zone1",
								Level: "rack",
								Nodes: map[string]*node_info.NodeInfo{},
							},
							"rack2.zone1": {
								ID:    "rack2.zone1",
								Level: "rack",
								Nodes: map[string]*node_info.NodeInfo{},
							},
						},
						"zone": {
							"zone1": {
								ID:    "zone1",
								Level: "zone",
								Nodes: map[string]*node_info.NodeInfo{},
							},
						},
					},
				}

				tree.DomainsByLevel[rootLevel] = map[DomainID]*DomainInfo{
					rootDomainId: tree.DomainsByLevel["zone"]["zone1"],
				}

				// Set parent relationships
				tree.DomainsByLevel["zone"]["zone1"].Children = map[DomainID]*DomainInfo{
					"rack1.zone1": tree.DomainsByLevel["rack"]["rack1.zone1"],
					"rack2.zone1": tree.DomainsByLevel["rack"]["rack2.zone1"],
				}

				return tree
			},
			domainParent: map[DomainID]DomainID{
				"rack1.zone1": "zone1",
				"rack2.zone1": "zone1",
			},
			domainLevel: map[DomainID]DomainLevel{
				"zone1": "zone",
			},
			expectedError: "",
			expectedNodes: map[string]bool{
				"node-1": true,
			},
		},
		{
			name: "no topology constraint - early return",
			job: &jobs_fake.TestJobBasic{
				Name:                "test-job",
				RequiredCPUsPerTask: 500,
				Tasks: []*tasks_fake.TestTaskBasic{
					{State: pod_status.Pending},
				},
			},
			jobTopologyConstraint: &enginev2alpha2.TopologyConstraint{
				Topology: "test-topology",
			},
			nodes: map[string]nodes_fake.TestNodeBasic{
				"node-1": {
					CPUMillis:  1000,
					GPUs:       6,
					MaxTaskNum: ptr.To(100),
				},
			},
			setupTopologyTree: func() *Info {
				return &Info{
					Name: "test-topology",
					TopologyResource: &kueuev1alpha1.Topology{
						Spec: kueuev1alpha1.TopologySpec{
							Levels: []kueuev1alpha1.TopologyLevel{
								{NodeLabel: "zone"},
							},
						},
					},
				}
			},
			expectedError: "",
		},
		{
			name: "topology not found - error",
			job: &jobs_fake.TestJobBasic{
				Name:                "test-job",
				Namespace:           "test-namespace",
				RequiredCPUsPerTask: 500,
				Tasks: []*tasks_fake.TestTaskBasic{
					{State: pod_status.Pending},
				},
			},
			jobTopologyConstraint: &enginev2alpha2.TopologyConstraint{
				Topology: "nonexistent-topology",
			},
			nodes: map[string]nodes_fake.TestNodeBasic{
				"node-1": {
					CPUMillis:  1000,
					GPUs:       6,
					MaxTaskNum: ptr.To(100),
				},
			},
			setupTopologyTree: func() *Info {
				return &Info{
					Name: "test-topology",
					TopologyResource: &kueuev1alpha1.Topology{
						Spec: kueuev1alpha1.TopologySpec{
							Levels: []kueuev1alpha1.TopologyLevel{
								{NodeLabel: "zone"},
							},
						},
					},
				}
			},
			expectedError: "matching topology tree haven't been found for job <test-namespace/test-job>, workload topology name: nonexistent-topology",
		},
		{
			name: "cache already populated - early return",
			job: &jobs_fake.TestJobBasic{
				Name:                "test-job",
				RequiredCPUsPerTask: 500,
				Tasks: []*tasks_fake.TestTaskBasic{
					{State: pod_status.Pending},
				},
			},
			jobTopologyConstraint: &enginev2alpha2.TopologyConstraint{
				Topology: "test-topology",
			},
			nodes: map[string]nodes_fake.TestNodeBasic{
				"node-1": {
					CPUMillis:  1000,
					GPUs:       6,
					MaxTaskNum: ptr.To(100),
				},
			},
			setupTopologyTree: func() *Info {
				return &Info{
					Name: "test-topology",
					TopologyResource: &kueuev1alpha1.Topology{
						Spec: kueuev1alpha1.TopologySpec{
							Levels: []kueuev1alpha1.TopologyLevel{
								{NodeLabel: "zone"},
							},
						},
					},
				}
			},
			setupSessionState: func(provider *mockSessionStateProvider, jobUID types.UID) {
				state := provider.GetSessionStateForResource(jobUID)
				(*k8sframework.CycleState)(state).Write(
					k8sframework.StateKey(topologyPluginName),
					&topologyStateData{
						relevantDomains: []*DomainInfo{
							{
								ID:              "zone1",
								Level:           "zone",
								AllocatablePods: 1,
							},
						},
					},
				)
				provider.GetSessionStateCallCount = 0 // Do not count this test data initialization as a call
			},
			expectedError: "",
		},
		{
			name: "insufficient allocatable pods - no domains found",
			job: &jobs_fake.TestJobBasic{
				Name:                "test-job",
				RequiredCPUsPerTask: 2000, // Too much for any node
				Tasks: []*tasks_fake.TestTaskBasic{
					{State: pod_status.Pending},
				},
			},
			jobTopologyConstraint: &enginev2alpha2.TopologyConstraint{
				Topology: "test-topology",
			},
			nodes: map[string]nodes_fake.TestNodeBasic{
				"node-1": {
					CPUMillis:  1000,
					GPUs:       6,
					MaxTaskNum: ptr.To(100),
				},
			},
			nodesToDomains: map[string]DomainID{
				"node-1": "zone1",
			},
			setupTopologyTree: func() *Info {
				tree := &Info{
					Name: "test-topology",
					TopologyResource: &kueuev1alpha1.Topology{
						Spec: kueuev1alpha1.TopologySpec{
							Levels: []kueuev1alpha1.TopologyLevel{
								{NodeLabel: "zone"},
							},
						},
					},
					DomainsByLevel: map[DomainLevel]LevelDomainInfos{
						"zone": {
							"zone1": {
								ID:    "zone1",
								Level: "zone",
								Nodes: map[string]*node_info.NodeInfo{},
							},
						},
					},
				}

				tree.DomainsByLevel[rootLevel] = map[DomainID]*DomainInfo{
					rootDomainId: tree.DomainsByLevel["zone"]["zone1"],
				}

				return tree
			},
			expectedError: "",
		},
		{
			name: "getJobAllocatableDomains constrain definition error",
			job: &jobs_fake.TestJobBasic{
				Name:                "test-job",
				RequiredCPUsPerTask: 500,
				Tasks: []*tasks_fake.TestTaskBasic{
					{State: pod_status.Pending},
				},
			},
			jobTopologyConstraint: &enginev2alpha2.TopologyConstraint{
				Topology:               "test-topology",
				RequiredTopologyLevel:  "nonexistent-level",
				PreferredTopologyLevel: "rack",
			},
			nodes: map[string]nodes_fake.TestNodeBasic{
				"node-1": {
					CPUMillis:  1000,
					GPUs:       6,
					MaxTaskNum: ptr.To(100),
				},
			},
			nodesToDomains: map[string]DomainID{
				"node-1": "rack1.zone1",
			},
			setupTopologyTree: func() *Info {
				tree := &Info{
					Name: "test-topology",
					TopologyResource: &kueuev1alpha1.Topology{
						Spec: kueuev1alpha1.TopologySpec{
							Levels: []kueuev1alpha1.TopologyLevel{
								{NodeLabel: "zone"},
								{NodeLabel: "rack"},
							},
						},
					},
					DomainsByLevel: map[DomainLevel]LevelDomainInfos{
						"rack": {
							"rack1.zone1": {
								ID:    "rack1.zone1",
								Level: "rack",
								Nodes: map[string]*node_info.NodeInfo{},
							},
						},
						"zone": {
							"zone1": {
								ID:    "zone1",
								Level: "zone",
								Nodes: map[string]*node_info.NodeInfo{},
							},
						},
					},
				}

				tree.DomainsByLevel[rootLevel] = map[DomainID]*DomainInfo{
					rootDomainId: tree.DomainsByLevel["zone"]["zone1"],
				}

				// Set parent relationships
				tree.DomainsByLevel["zone"]["zone1"].Children = map[DomainID]*DomainInfo{
					"rack1.zone1": tree.DomainsByLevel["rack"]["rack1.zone1"],
				}

				return tree
			},
			domainParent: map[DomainID]DomainID{
				"rack1.zone1": "zone1",
			},
			domainLevel: map[DomainID]DomainLevel{
				"zone1": "zone",
			},
			expectedError: "the topology test-topology doesn't have a level matching the required(nonexistent-level) specified for the job test-job",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup test data
			jobName := tt.job.Name
			clusterPodGroups := append(tt.allocatedPodGroups, tt.job)
			jobsInfoMap, tasksToNodeMap, _ := jobs_fake.BuildJobsAndTasksMaps(clusterPodGroups)
			nodesInfoMap := nodes_fake.BuildNodesInfoMap(tt.nodes, tasksToNodeMap, nil)
			job := jobsInfoMap[common_info.PodGroupID(jobName)]

			// Setup topology tree
			topologyTree := tt.setupTopologyTree()
			if tt.nodesToDomains != nil {
				for nodeName, domainId := range tt.nodesToDomains {
					nodeInfo := nodesInfoMap[nodeName]
					leafLevel := len(topologyTree.TopologyResource.Spec.Levels) - 1
					domain := topologyTree.DomainsByLevel[DomainLevel(topologyTree.TopologyResource.Spec.Levels[leafLevel].NodeLabel)][domainId]
					for domain != nil {
						domain.AddNode(nodeInfo)
						parentDomainId := tt.domainParent[domain.ID]
						parentDomainLevel := tt.domainLevel[parentDomainId]
						domain = topologyTree.DomainsByLevel[parentDomainLevel][parentDomainId]
					}
				}
			}

			// Setup session state provider
			sessionStateProvider := newMockSessionStateProvider()
			if tt.setupSessionState != nil {
				tt.setupSessionState(sessionStateProvider, types.UID(job.PodGroupUID))
			}

			// Setup plugin
			plugin := &topologyPlugin{
				TopologyTrees: map[string]*Info{
					"test-topology": topologyTree,
				}}

			// Update job with topology constraints based on test case
			if tt.jobTopologyConstraint != nil {
				job.TopologyConstraint = &podgroup_info.TopologyConstraintInfo{
					Topology:       tt.jobTopologyConstraint.Topology,
					RequiredLevel:  tt.jobTopologyConstraint.RequiredTopologyLevel,
					PreferredLevel: tt.jobTopologyConstraint.PreferredTopologyLevel,
				}
			}

			// Call the function under test
			subsets, err := plugin.subSetNodesFn(job, podgroup_info.GetTasksToAllocate(job, nil, nil, true), maps.Values(nodesInfoMap))

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

			// Verify nodes
			if tt.expectedNodes != nil {
				if subsets == nil {
					t.Errorf("expected subsets to be filled with %v, but got nil", tt.expectedNodes)
				}
				for _, node := range subsets[0] {
					_, ok := tt.expectedNodes[node.Name]
					assert.True(t, ok, "expected node %s to be in subset %v", node.Name, tt.expectedNodes)
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
		topologyTree    *Info
		expectedLevels  []DomainLevel
		expectedError   string
	}{
		{
			name: "both required and preferred placement specified",
			job: &podgroup_info.PodGroupInfo{
				Name: "test-job",
				TopologyConstraint: &podgroup_info.TopologyConstraintInfo{
					RequiredLevel:  "zone",
					PreferredLevel: "rack",
				},
			},
			jobTopologyName: "test-topology",
			topologyTree: &Info{
				Name: "test-topology",
				TopologyResource: &kueuev1alpha1.Topology{
					Spec: kueuev1alpha1.TopologySpec{
						Levels: []kueuev1alpha1.TopologyLevel{
							{NodeLabel: "datacenter"},
							{NodeLabel: "zone"},
							{NodeLabel: "rack"},
						},
					},
				},
			},
			expectedLevels: []DomainLevel{
				"rack",
				"zone",
			},
			expectedError: "",
		},
		{
			name: "only required placement specified",
			job: &podgroup_info.PodGroupInfo{
				Name: "test-job",
				TopologyConstraint: &podgroup_info.TopologyConstraintInfo{
					RequiredLevel: "zone",
				},
			},
			jobTopologyName: "test-topology",
			topologyTree: &Info{
				Name: "test-topology",
				TopologyResource: &kueuev1alpha1.Topology{
					Spec: kueuev1alpha1.TopologySpec{
						Levels: []kueuev1alpha1.TopologyLevel{
							{NodeLabel: "datacenter"},
							{NodeLabel: "zone"},
							{NodeLabel: "rack"},
						},
					},
				},
			},
			expectedLevels: []DomainLevel{
				"zone",
			},
			expectedError: "",
		},
		{
			name: "only preferred placement specified",
			job: &podgroup_info.PodGroupInfo{
				Name:      "test-job",
				Namespace: "test-namespace",
				TopologyConstraint: &podgroup_info.TopologyConstraintInfo{
					PreferredLevel: "rack",
				},
			},
			jobTopologyName: "test-topology",
			topologyTree: &Info{
				Name: "test-topology",
				TopologyResource: &kueuev1alpha1.Topology{
					Spec: kueuev1alpha1.TopologySpec{
						Levels: []kueuev1alpha1.TopologyLevel{
							{NodeLabel: "datacenter"},
							{NodeLabel: "zone"},
							{NodeLabel: "rack"},
						},
					},
				},
			},
			expectedLevels: []DomainLevel{
				"rack",
				"zone",
				"datacenter",
				"root",
			},
			expectedError: "",
		},
		{
			name: "no placement annotations specified",
			job: &podgroup_info.PodGroupInfo{
				Name:               "test-job",
				Namespace:          "test-namespace",
				TopologyConstraint: &podgroup_info.TopologyConstraintInfo{},
			},
			jobTopologyName: "test-topology",
			topologyTree: &Info{
				Name: "test-topology",
				TopologyResource: &kueuev1alpha1.Topology{
					Spec: kueuev1alpha1.TopologySpec{
						Levels: []kueuev1alpha1.TopologyLevel{
							{NodeLabel: "zone"},
							{NodeLabel: "rack"},
						},
					},
				},
			},
			expectedLevels: nil,
			expectedError:  "no topology placement annotations found for job <test-namespace/test-job>, workload topology name: test-topology",
		},
		{
			name: "required placement not found in topology",
			job: &podgroup_info.PodGroupInfo{
				Name: "test-job",
				TopologyConstraint: &podgroup_info.TopologyConstraintInfo{
					RequiredLevel: "nonexistent",
				},
			},
			jobTopologyName: "test-topology",
			topologyTree: &Info{
				Name: "test-topology",
				TopologyResource: &kueuev1alpha1.Topology{
					Spec: kueuev1alpha1.TopologySpec{
						Levels: []kueuev1alpha1.TopologyLevel{
							{NodeLabel: "zone"},
							{NodeLabel: "rack"},
						},
					},
				},
			},
			expectedLevels: nil,
			expectedError:  "the topology test-topology doesn't have a level matching the required(nonexistent) specified for the job test-job",
		},
		{
			name: "preferred placement not found in topology",
			job: &podgroup_info.PodGroupInfo{
				Name: "test-job",
				TopologyConstraint: &podgroup_info.TopologyConstraintInfo{
					PreferredLevel: "nonexistent",
				},
			},
			jobTopologyName: "test-topology",
			topologyTree: &Info{
				Name: "test-topology",
				TopologyResource: &kueuev1alpha1.Topology{
					Spec: kueuev1alpha1.TopologySpec{
						Levels: []kueuev1alpha1.TopologyLevel{
							{NodeLabel: "zone"},
							{NodeLabel: "rack"},
						},
					},
				},
			},
			expectedLevels: nil,
			expectedError:  "the topology test-topology doesn't have a level matching the preferred(nonexistent) specified for the job test-job",
		},
		{
			name: "required placement at first level",
			job: &podgroup_info.PodGroupInfo{
				Name: "test-job",
				TopologyConstraint: &podgroup_info.TopologyConstraintInfo{
					RequiredLevel: "rack",
				},
			},
			jobTopologyName: "test-topology",
			topologyTree: &Info{
				Name: "test-topology",
				TopologyResource: &kueuev1alpha1.Topology{
					Spec: kueuev1alpha1.TopologySpec{
						Levels: []kueuev1alpha1.TopologyLevel{
							{NodeLabel: "datacenter"},
							{NodeLabel: "zone"},
							{NodeLabel: "rack"},
						},
					},
				},
			},
			expectedLevels: []DomainLevel{
				"rack",
			},
			expectedError: "",
		},
		{
			name: "preferred placement at first level",
			job: &podgroup_info.PodGroupInfo{
				Name: "test-job",
				TopologyConstraint: &podgroup_info.TopologyConstraintInfo{
					PreferredLevel: "rack",
				},
			},
			jobTopologyName: "test-topology",
			topologyTree: &Info{
				Name: "test-topology",
				TopologyResource: &kueuev1alpha1.Topology{
					Spec: kueuev1alpha1.TopologySpec{
						Levels: []kueuev1alpha1.TopologyLevel{
							{NodeLabel: "datacenter"},
							{NodeLabel: "zone"},
							{NodeLabel: "rack"},
						},
					},
				},
			},
			expectedLevels: []DomainLevel{
				"rack",
				"zone",
				"datacenter",
				"root",
			},
			expectedError: "",
		},
		{
			name: "preferred placement at middle level",
			job: &podgroup_info.PodGroupInfo{
				Name: "test-job",
				TopologyConstraint: &podgroup_info.TopologyConstraintInfo{
					PreferredLevel: "zone",
				},
			},
			jobTopologyName: "test-topology",
			topologyTree: &Info{
				Name: "test-topology",
				TopologyResource: &kueuev1alpha1.Topology{
					Spec: kueuev1alpha1.TopologySpec{
						Levels: []kueuev1alpha1.TopologyLevel{
							{NodeLabel: "datacenter"},
							{NodeLabel: "zone"},
							{NodeLabel: "rack"},
						},
					},
				},
			},
			expectedLevels: []DomainLevel{
				"zone",
				"datacenter",
				"root",
			},
			expectedError: "",
		},
		{
			name: "single level topology with preferred placement",
			job: &podgroup_info.PodGroupInfo{
				Name: "test-job",
				TopologyConstraint: &podgroup_info.TopologyConstraintInfo{
					PreferredLevel: "zone",
				},
			},
			jobTopologyName: "test-topology",
			topologyTree: &Info{
				Name: "test-topology",
				TopologyResource: &kueuev1alpha1.Topology{
					Spec: kueuev1alpha1.TopologySpec{
						Levels: []kueuev1alpha1.TopologyLevel{
							{NodeLabel: "zone"},
						},
					},
				},
			},
			expectedLevels: []DomainLevel{
				"zone",
				"root",
			},
			expectedError: "",
		},
		{
			name: "complex topology with multiple levels",
			job: &podgroup_info.PodGroupInfo{
				Name: "test-job",
				TopologyConstraint: &podgroup_info.TopologyConstraintInfo{
					RequiredLevel:  "region",
					PreferredLevel: "zone",
				},
			},
			jobTopologyName: "test-topology",
			topologyTree: &Info{
				Name: "test-topology",
				TopologyResource: &kueuev1alpha1.Topology{
					Spec: kueuev1alpha1.TopologySpec{
						Levels: []kueuev1alpha1.TopologyLevel{
							{NodeLabel: "datacenter"},
							{NodeLabel: "region"},
							{NodeLabel: "zone"},
							{NodeLabel: "rack"},
						},
					},
				},
			},
			expectedLevels: []DomainLevel{
				"zone",
				"region",
			},
			expectedError: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			plugin := &topologyPlugin{}

			result, err := plugin.calculateRelevantDomainLevels(tt.job, tt.topologyTree)

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

			if !slices.Equal(result, tt.expectedLevels) {
				t.Errorf("expected %v, but got %v", tt.expectedLevels, result)
			}
		})
	}
}

func TestTopologyPlugin_calcTreeAllocatable(t *testing.T) {
	tests := []struct {
		name                       string
		job                        *jobs_fake.TestJobBasic
		allocatedPodGroups         []*jobs_fake.TestJobBasic
		nodes                      map[string]nodes_fake.TestNodeBasic
		nodesToDomains             map[string]DomainID
		setupTopologyTree          func() *Info
		domainParent               map[DomainID]DomainID
		domainLevel                map[DomainID]DomainLevel
		expectedMaxAllocatablePods int
		expectedDomains            map[DomainID]*DomainInfo
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
			nodesToDomains: map[string]DomainID{
				"node-1": "rack1.zone1",
				"node-2": "rack2.zone1",
			},
			setupTopologyTree: func() *Info {
				tree := &Info{
					Name: "test-topology",
					TopologyResource: &kueuev1alpha1.Topology{
						Spec: kueuev1alpha1.TopologySpec{
							Levels: []kueuev1alpha1.TopologyLevel{
								{NodeLabel: "zone"},
								{NodeLabel: "rack"},
							},
						},
					},
					DomainsByLevel: map[DomainLevel]LevelDomainInfos{
						"rack": {
							"rack1.zone1": {
								ID:    "rack1.zone1",
								Level: "rack",
								Nodes: map[string]*node_info.NodeInfo{},
							},
							"rack2.zone1": {
								ID:    "rack2.zone1",
								Level: "rack",
								Nodes: map[string]*node_info.NodeInfo{},
							},
						},
						"zone": {
							"zone1": {
								ID:    "zone1",
								Level: "zone",
								Nodes: map[string]*node_info.NodeInfo{},
							},
						},
					},
				}

				tree.DomainsByLevel[rootLevel] = map[DomainID]*DomainInfo{
					rootDomainId: tree.DomainsByLevel["zone"]["zone1"],
				}

				// Set parent relationships
				tree.DomainsByLevel["zone"]["zone1"].Children = map[DomainID]*DomainInfo{
					"rack1.zone1": tree.DomainsByLevel["rack"]["rack1.zone1"],
					"rack2.zone1": tree.DomainsByLevel["rack"]["rack2.zone1"],
				}

				return tree
			},
			domainParent: map[DomainID]DomainID{
				"rack1.zone1": "zone1",
				"rack2.zone1": "zone1",
			},
			domainLevel: map[DomainID]DomainLevel{
				"zone1": "zone",
			},
			expectedMaxAllocatablePods: 4,
			expectedDomains: map[DomainID]*DomainInfo{
				"rack1.zone1": {
					ID:              "rack1.zone1",
					Level:           "rack",
					AllocatablePods: 2,
				},
				"rack2.zone1": {
					ID:              "rack2.zone1",
					Level:           "rack",
					AllocatablePods: 2,
				},
				"zone1": {
					ID:              "zone1",
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
			nodesToDomains: map[string]DomainID{
				"node-1": "rack1.zone1",
				"node-2": "rack2.zone1",
			},
			setupTopologyTree: func() *Info {
				tree := &Info{
					Name: "test-topology",
					TopologyResource: &kueuev1alpha1.Topology{
						Spec: kueuev1alpha1.TopologySpec{
							Levels: []kueuev1alpha1.TopologyLevel{
								{NodeLabel: "zone"},
								{NodeLabel: "rack"},
							},
						},
					},
					DomainsByLevel: map[DomainLevel]LevelDomainInfos{
						"rack": {
							"rack1.zone1": {
								ID:    "rack1.zone1",
								Level: "rack",
								Nodes: map[string]*node_info.NodeInfo{},
							},
							"rack2.zone1": {
								ID:    "rack2.zone1",
								Level: "rack",
								Nodes: map[string]*node_info.NodeInfo{},
							},
						},
						"zone": {
							"zone1": {
								ID:    "zone1",
								Level: "zone",
								Nodes: map[string]*node_info.NodeInfo{},
							},
						},
					},
				}

				tree.DomainsByLevel[rootLevel] = map[DomainID]*DomainInfo{
					rootDomainId: tree.DomainsByLevel["zone"]["zone1"],
				}

				// Set parent relationships
				tree.DomainsByLevel["zone"]["zone1"].Children = map[DomainID]*DomainInfo{
					"rack1.zone1": tree.DomainsByLevel["rack"]["rack1.zone1"],
					"rack2.zone1": tree.DomainsByLevel["rack"]["rack2.zone1"],
				}

				return tree
			},
			domainParent: map[DomainID]DomainID{
				"rack1.zone1": "zone1",
				"rack2.zone1": "zone1",
			},
			domainLevel: map[DomainID]DomainLevel{
				"zone1": "zone",
			},
			expectedMaxAllocatablePods: 2,
			expectedDomains: map[DomainID]*DomainInfo{
				"rack1.zone1": {
					ID:              "rack1.zone1",
					Level:           "rack",
					AllocatablePods: 1, // Can only fit 1 pod
				},
				"rack2.zone1": {
					ID:              "rack2.zone1",
					Level:           "rack",
					AllocatablePods: 1, // Can only fit 1 pod
				},
				"zone1": {
					ID:              "zone1",
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
			nodesToDomains: map[string]DomainID{
				"node-1": "rack1.zone1",
				"node-2": "rack1.zone1",
				"node-3": "rack2.zone1",
			},
			setupTopologyTree: func() *Info {
				tree := &Info{
					Name: "test-topology",
					TopologyResource: &kueuev1alpha1.Topology{
						Spec: kueuev1alpha1.TopologySpec{
							Levels: []kueuev1alpha1.TopologyLevel{
								{NodeLabel: "zone"},
								{NodeLabel: "rack"},
							},
						},
					},
					DomainsByLevel: map[DomainLevel]LevelDomainInfos{
						"rack": {
							"rack1.zone1": {
								ID:    "rack1.zone1",
								Level: "rack",
								Nodes: map[string]*node_info.NodeInfo{},
							},
							"rack2.zone1": {
								ID:    "rack2.zone1",
								Level: "rack",
								Nodes: map[string]*node_info.NodeInfo{},
							},
						},
						"zone": {
							"zone1": {
								ID:    "zone1",
								Level: "zone",
								Nodes: map[string]*node_info.NodeInfo{},
							},
						},
					},
				}

				tree.DomainsByLevel[rootLevel] = map[DomainID]*DomainInfo{
					rootDomainId: tree.DomainsByLevel["zone"]["zone1"],
				}

				// Set parent relationships
				tree.DomainsByLevel["zone"]["zone1"].Children = map[DomainID]*DomainInfo{
					"rack1.zone1": tree.DomainsByLevel["rack"]["rack1.zone1"],
					"rack2.zone1": tree.DomainsByLevel["rack"]["rack2.zone1"],
				}

				return tree
			},
			domainParent: map[DomainID]DomainID{
				"rack1.zone1": "zone1",
				"rack2.zone1": "zone1",
			},
			domainLevel: map[DomainID]DomainLevel{
				"zone1": "zone",
			},
			expectedMaxAllocatablePods: 4,
			expectedDomains: map[DomainID]*DomainInfo{
				"rack1.zone1": {
					ID:              "rack1.zone1",
					Level:           "rack",
					AllocatablePods: 2,
				},
				"rack2.zone1": {
					ID:              "rack2.zone1",
					Level:           "rack",
					AllocatablePods: 2,
				},
				"zone1": {
					ID:              "zone1",
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
			nodesToDomains: map[string]DomainID{
				"node-1": "zone1",
			},
			setupTopologyTree: func() *Info {
				tree := &Info{
					Name: "test-topology",
					TopologyResource: &kueuev1alpha1.Topology{
						Spec: kueuev1alpha1.TopologySpec{
							Levels: []kueuev1alpha1.TopologyLevel{
								{NodeLabel: "zone"},
							},
						},
					},
					DomainsByLevel: map[DomainLevel]LevelDomainInfos{
						"zone": {
							"zone1": {
								ID:    "zone1",
								Level: "zone",
								Nodes: map[string]*node_info.NodeInfo{},
							},
						},
					},
				}

				tree.DomainsByLevel[rootLevel] = map[DomainID]*DomainInfo{
					rootDomainId: tree.DomainsByLevel["zone"]["zone1"],
				}

				return tree
			},
			expectedMaxAllocatablePods: 0,
			expectedDomains:            map[DomainID]*DomainInfo{
				// No domains should have allocations since no nodes can accommodate the job
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			jobName := tt.job.Name
			clusterPodGroups := append(tt.allocatedPodGroups, tt.job)
			jobsInfoMap, tasksToNodeMap, _ := jobs_fake.BuildJobsAndTasksMaps(clusterPodGroups)
			nodesInfoMap := nodes_fake.BuildNodesInfoMap(tt.nodes, tasksToNodeMap, nil)
			job := jobsInfoMap[common_info.PodGroupID(jobName)]

			topologyTree := tt.setupTopologyTree()
			for nodeName, domainId := range tt.nodesToDomains {
				nodeInfo := nodesInfoMap[nodeName]
				leafLevel := len(topologyTree.TopologyResource.Spec.Levels) - 1
				domain := topologyTree.DomainsByLevel[DomainLevel(topologyTree.TopologyResource.Spec.Levels[leafLevel].NodeLabel)][domainId]
				for domain != nil {
					domain.AddNode(nodeInfo)
					parentDomainId := tt.domainParent[domain.ID]
					parentDomainLevel := tt.domainLevel[parentDomainId]
					domain = topologyTree.DomainsByLevel[parentDomainLevel][parentDomainId]
				}
			}

			session := &framework.Session{
				Nodes:         nodesInfoMap,
				PodGroupInfos: jobsInfoMap,
				Topologies:    []*kueuev1alpha1.Topology{topologyTree.TopologyResource},
			}
			plugin := &topologyPlugin{}

			// Call the function under test
			maxAllocatablePods, err := plugin.calcTreeAllocatable(podgroup_info.GetTasksToAllocate(job, nil, nil, true), topologyTree, maps.Values(session.Nodes))
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

func TestTopologyPlugin_getJobAllocatableDomains(t *testing.T) {
	tests := []struct {
		name            string
		job             *podgroup_info.PodGroupInfo
		topologyTree    *Info
		taskOrderFunc   common_info.LessFn
		expectedDomains []*DomainInfo
		expectedError   string
	}{
		{
			name: "return multi domain",
			job: &podgroup_info.PodGroupInfo{
				Name: "test-job",
				SubGroups: map[string]*subgroup_info.SubGroupInfo{
					podgroup_info.DefaultSubGroup: subgroup_info.NewSubGroupInfo(podgroup_info.DefaultSubGroup, 2).WithPodInfos(map[common_info.PodID]*pod_info.PodInfo{
						"pod1": {UID: "pod1", Name: "pod1", Status: pod_status.Pending},
						"pod2": {UID: "pod2", Name: "pod2", Status: pod_status.Pending},
					}),
				},
				TopologyConstraint: &podgroup_info.TopologyConstraintInfo{
					RequiredLevel:  "zone",
					PreferredLevel: "rack",
				},
			},
			topologyTree: &Info{
				Name: "test-topology",
				TopologyResource: &kueuev1alpha1.Topology{
					Spec: kueuev1alpha1.TopologySpec{
						Levels: []kueuev1alpha1.TopologyLevel{
							{NodeLabel: "zone"},
							{NodeLabel: "rack"},
						},
					},
				},
				DomainsByLevel: map[DomainLevel]LevelDomainInfos{
					"rack": {
						"rack1.zone1": {
							ID:              "rack1.zone1",
							Level:           "rack",
							AllocatablePods: 2,
						},
						"rack2.zone1": {
							ID:              "rack2.zone1",
							Level:           "rack",
							AllocatablePods: 1,
						},
					},
					"zone": {
						"zone1": {
							ID:              "zone1",
							Level:           "zone",
							AllocatablePods: 3,
						},
					},
				},
			},
			taskOrderFunc: func(l, r interface{}) bool {
				return l.(*pod_info.PodInfo).Name < r.(*pod_info.PodInfo).Name
			},
			expectedDomains: []*DomainInfo{
				{
					ID:              "rack1.zone1",
					Level:           "rack",
					AllocatablePods: 2,
				},
				{
					ID:              "zone1",
					Level:           "zone",
					AllocatablePods: 3,
				},
			},
			expectedError: "",
		},
		{
			name: "no domains can allocate the job",
			job: &podgroup_info.PodGroupInfo{
				Name:      "test-job",
				Namespace: "test-namespace",
				SubGroups: map[string]*subgroup_info.SubGroupInfo{
					podgroup_info.DefaultSubGroup: subgroup_info.NewSubGroupInfo(podgroup_info.DefaultSubGroup, 2).WithPodInfos(map[common_info.PodID]*pod_info.PodInfo{
						"pod1": {UID: "pod1", Name: "pod1", Status: pod_status.Pending},
						"pod2": {UID: "pod2", Name: "pod2", Status: pod_status.Pending},
					}),
				},
				TopologyConstraint: &podgroup_info.TopologyConstraintInfo{
					RequiredLevel: "zone",
				},
			},
			topologyTree: &Info{
				Name: "test-topology",
				TopologyResource: &kueuev1alpha1.Topology{
					Spec: kueuev1alpha1.TopologySpec{
						Levels: []kueuev1alpha1.TopologyLevel{
							{NodeLabel: "zone"},
							{NodeLabel: "rack"},
						},
					},
				},
				DomainsByLevel: map[DomainLevel]LevelDomainInfos{
					"rack": {
						"rack1.zone1": {
							ID:              "rack1.zone1",
							Level:           "rack",
							AllocatablePods: 1, // Can only fit 1 pod, job needs 2
						},
					},
					"zone": {
						"zone1": {
							ID:              "zone1",
							Level:           "zone",
							AllocatablePods: 1, // Can only fit 1 pod, job needs 2
						},
					},
				},
			},
			taskOrderFunc: func(l, r interface{}) bool {
				return l.(*pod_info.PodInfo).Name < r.(*pod_info.PodInfo).Name
			},
			expectedDomains: []*DomainInfo{},
			expectedError:   "no domains found for the job <test-namespace/test-job>, workload topology name: test-topology",
		},
		{
			name: "no relevant domain levels",
			job: &podgroup_info.PodGroupInfo{
				Name: "test-job",
				SubGroups: map[string]*subgroup_info.SubGroupInfo{
					podgroup_info.DefaultSubGroup: subgroup_info.NewSubGroupInfo(podgroup_info.DefaultSubGroup, 1).WithPodInfos(map[common_info.PodID]*pod_info.PodInfo{
						"pod1": {UID: "pod1", Name: "pod1", Status: pod_status.Pending},
					}),
				},
				TopologyConstraint: &podgroup_info.TopologyConstraintInfo{
					RequiredLevel:  "zone",
					PreferredLevel: "rack",
				},
			},
			topologyTree: &Info{
				Name: "test-topology",
				TopologyResource: &kueuev1alpha1.Topology{
					Spec: kueuev1alpha1.TopologySpec{
						Levels: []kueuev1alpha1.TopologyLevel{
							{NodeLabel: "datacenter"},
							{NodeLabel: "region"},
						},
					},
				},
				DomainsByLevel: map[DomainLevel]LevelDomainInfos{
					"datacenter": {
						"datacenter1": {
							ID:              "datacenter1",
							Level:           "datacenter",
							AllocatablePods: 1,
						},
					},
				},
			},
			taskOrderFunc: func(l, r interface{}) bool {
				return l.(*pod_info.PodInfo).Name < r.(*pod_info.PodInfo).Name
			},
			expectedDomains: nil,
			expectedError:   "the topology test-topology doesn't have a level matching the required(zone) specified for the job test-job",
		},
		{
			name: "complex topology with multiple levels",
			job: &podgroup_info.PodGroupInfo{
				Name: "test-job",
				SubGroups: map[string]*subgroup_info.SubGroupInfo{
					podgroup_info.DefaultSubGroup: subgroup_info.NewSubGroupInfo(podgroup_info.DefaultSubGroup, 3).WithPodInfos(map[common_info.PodID]*pod_info.PodInfo{
						"pod1": {UID: "pod1", Name: "pod1", Status: pod_status.Pending},
						"pod2": {UID: "pod2", Name: "pod2", Status: pod_status.Pending},
						"pod3": {UID: "pod3", Name: "pod3", Status: pod_status.Pending},
					}),
				},
				TopologyConstraint: &podgroup_info.TopologyConstraintInfo{
					RequiredLevel:  "region",
					PreferredLevel: "zone",
				},
			},
			topologyTree: &Info{
				Name: "test-topology",
				TopologyResource: &kueuev1alpha1.Topology{
					Spec: kueuev1alpha1.TopologySpec{
						Levels: []kueuev1alpha1.TopologyLevel{
							{NodeLabel: "datacenter"},
							{NodeLabel: "region"},
							{NodeLabel: "zone"},
							{NodeLabel: "rack"},
						},
					},
				},
				DomainsByLevel: map[DomainLevel]LevelDomainInfos{
					"rack": {
						"rack1.zone1.region1": {
							ID:              "rack1.zone1.region1",
							Level:           "rack",
							AllocatablePods: 3,
						},
						"rack2.zone1.region1": {
							ID:              "rack2.zone1.region1",
							Level:           "rack",
							AllocatablePods: 3,
						},
					},
					"zone": {
						"zone1.region1": {
							ID:              "zone1.region1",
							Level:           "zone",
							AllocatablePods: 6,
						},
					},
					"region": {
						"region1": {
							ID:              "region1",
							Level:           "region",
							AllocatablePods: 9,
						},
					},
				},
			},
			taskOrderFunc: func(l, r interface{}) bool {
				return l.(*pod_info.PodInfo).Name < r.(*pod_info.PodInfo).Name
			},
			expectedDomains: []*DomainInfo{
				{
					ID:              "zone1.region1",
					Level:           "zone",
					AllocatablePods: 6,
				},
				{
					ID:              "region1",
					Level:           "region",
					AllocatablePods: 9,
				},
			},
			expectedError: "",
		},
		{
			name: "mixed task statuses - some pending, some running",
			job: &podgroup_info.PodGroupInfo{
				Name: "test-job",
				SubGroups: map[string]*subgroup_info.SubGroupInfo{
					podgroup_info.DefaultSubGroup: subgroup_info.NewSubGroupInfo(podgroup_info.DefaultSubGroup, 2).WithPodInfos(map[common_info.PodID]*pod_info.PodInfo{
						"pod1": {UID: "pod1", Name: "pod1", Status: pod_status.Running, NodeName: "node1"},
						"pod2": {UID: "pod2", Name: "pod2", Status: pod_status.Pending},
						"pod3": {UID: "pod3", Name: "pod3", Status: pod_status.Pending},
					}),
				},
				TopologyConstraint: &podgroup_info.TopologyConstraintInfo{
					RequiredLevel: "zone",
				},
			},
			topologyTree: &Info{
				Name: "test-topology",
				TopologyResource: &kueuev1alpha1.Topology{
					Spec: kueuev1alpha1.TopologySpec{
						Levels: []kueuev1alpha1.TopologyLevel{
							{NodeLabel: "zone"},
						},
					},
				},
				DomainsByLevel: map[DomainLevel]LevelDomainInfos{
					"zone": {
						"zone1": {
							ID:    "zone1",
							Level: "zone",
							Nodes: map[string]*node_info.NodeInfo{
								"node1": {
									Node: &v1.Node{
										ObjectMeta: metav1.ObjectMeta{
											Name: "node1",
										},
									},
								},
							},
							AllocatablePods: 2,
						},
					},
				},
			},
			taskOrderFunc: func(l, r interface{}) bool {
				return l.(*pod_info.PodInfo).Name < r.(*pod_info.PodInfo).Name
			},
			expectedDomains: []*DomainInfo{
				{
					ID:              "zone1",
					Level:           "zone",
					AllocatablePods: 2,
				},
			},
			expectedError: "",
		},
		{
			name: "mixed task statuses with required constraint - choose zone with existing pods",
			job: &podgroup_info.PodGroupInfo{
				Name: "test-job",
				SubGroups: map[string]*subgroup_info.SubGroupInfo{
					podgroup_info.DefaultSubGroup: subgroup_info.NewSubGroupInfo(podgroup_info.DefaultSubGroup, 2).WithPodInfos(map[common_info.PodID]*pod_info.PodInfo{
						"pod1": {UID: "pod1", Name: "pod1", Status: pod_status.Running, NodeName: "node2"},
						"pod2": {UID: "pod2", Name: "pod2", Status: pod_status.Pending},
						"pod3": {UID: "pod3", Name: "pod3", Status: pod_status.Pending},
					}),
				},
				TopologyConstraint: &podgroup_info.TopologyConstraintInfo{
					RequiredLevel: "zone",
				},
			},
			topologyTree: &Info{
				Name: "test-topology",
				TopologyResource: &kueuev1alpha1.Topology{
					Spec: kueuev1alpha1.TopologySpec{
						Levels: []kueuev1alpha1.TopologyLevel{
							{NodeLabel: "zone"},
						},
					},
				},
				DomainsByLevel: map[DomainLevel]LevelDomainInfos{
					"zone": {
						"zone1": {
							ID:    "zone1",
							Level: "zone",
							Nodes: map[string]*node_info.NodeInfo{
								"node1": {
									Node: &v1.Node{
										ObjectMeta: metav1.ObjectMeta{
											Name: "node1",
										},
									},
								},
							},
							AllocatablePods: 2,
						},
						"zone2": {
							ID:    "zone2",
							Level: "zone",
							Nodes: map[string]*node_info.NodeInfo{
								"node2": {
									Node: &v1.Node{
										ObjectMeta: metav1.ObjectMeta{
											Name: "node2",
										},
									},
								},
							},
							AllocatablePods: 2,
						},
					},
				},
			},
			taskOrderFunc: func(l, r interface{}) bool {
				return l.(*pod_info.PodInfo).Name < r.(*pod_info.PodInfo).Name
			},
			expectedDomains: []*DomainInfo{
				{
					ID:              "zone2",
					Level:           "zone",
					AllocatablePods: 2,
				},
			},
			expectedError: "",
		},
		{
			name: "Return multiple levels of domains",
			job: &podgroup_info.PodGroupInfo{
				Name: "test-job",
				SubGroups: map[string]*subgroup_info.SubGroupInfo{
					podgroup_info.DefaultSubGroup: subgroup_info.NewSubGroupInfo(podgroup_info.DefaultSubGroup, 4).WithPodInfos(map[common_info.PodID]*pod_info.PodInfo{
						"pod1": {UID: "pod1", Name: "pod1", Status: pod_status.Pending},
						"pod2": {UID: "pod2", Name: "pod2", Status: pod_status.Pending},
						"pod3": {UID: "pod3", Name: "pod3", Status: pod_status.Pending},
						"pod4": {UID: "pod4", Name: "pod4", Status: pod_status.Pending},
					}),
				},
				TopologyConstraint: &podgroup_info.TopologyConstraintInfo{
					RequiredLevel:  "region",
					PreferredLevel: "rack",
				},
			},
			topologyTree: &Info{
				Name: "test-topology",
				TopologyResource: &kueuev1alpha1.Topology{
					Spec: kueuev1alpha1.TopologySpec{
						Levels: []kueuev1alpha1.TopologyLevel{
							{NodeLabel: "datacenter"},
							{NodeLabel: "region"},
							{NodeLabel: "zone"},
							{NodeLabel: "rack"},
						},
					},
				},
				DomainsByLevel: map[DomainLevel]LevelDomainInfos{
					"rack": {
						"rack1.zone1.region1": {
							ID:              "rack1.zone1.region1",
							Level:           "rack",
							AllocatablePods: 3,
						},
						"rack2.zone1.region1": {
							ID:              "rack2.zone1.region1",
							Level:           "rack",
							AllocatablePods: 3,
						},
						"rack1.zone2.region1": {
							ID:              "rack1.zone2.region1",
							Level:           "rack",
							AllocatablePods: 2,
						},
						"rack2.zone2.region1": {
							ID:              "rack2.zone1.region1",
							Level:           "rack",
							AllocatablePods: 1,
						},
						"rack3.zone3.region1": {
							ID:              "rack3.zone2.region1",
							Level:           "rack",
							AllocatablePods: 1,
						},
						"rack4.zone2.region1": {
							ID:              "rack4.zone2.region1",
							Level:           "rack",
							AllocatablePods: 1,
						},
						"rack5.zone2.region1": {
							ID:              "rack5.zone2.region1",
							Level:           "rack",
							AllocatablePods: 1,
						},
					},
					"zone": {
						"zone1.region1": {
							ID:              "zone1.region1",
							Level:           "zone",
							AllocatablePods: 6,
							Children: map[DomainID]*DomainInfo{
								"rack1.zone1.region1": {
									ID:              "rack1.zone1.region1",
									Level:           "rack",
									AllocatablePods: 3,
								},
								"rack2.zone1.region1": {
									ID:              "rack2.zone1.region1",
									Level:           "rack",
									AllocatablePods: 3,
								},
							},
						},
						"zone2.region1": {
							ID:              "zone2.region1",
							Level:           "zone",
							AllocatablePods: 6,
							Children: map[DomainID]*DomainInfo{
								"rack1.zone2.region1": {
									ID:              "rack1.zone2.region1",
									Level:           "rack",
									AllocatablePods: 2,
								},
								"rack2.zone1.region1": {
									ID:              "rack2.zone1.region1",
									Level:           "rack",
									AllocatablePods: 1,
								},
								"rack3.zone2.region1": {
									ID:              "rack3.zone2.region1",
									Level:           "rack",
									AllocatablePods: 1,
								},
								"rack4.zone2.region1": {
									ID:              "rack4.zone2.region1",
									Level:           "rack",
									AllocatablePods: 1,
								},
								"rack5.zone2.region1": {
									ID:              "rack5.zone2.region1",
									Level:           "rack",
									AllocatablePods: 1,
								},
							},
						},
					},
					"region": {
						"region1": {
							ID:              "region1",
							Level:           "region",
							AllocatablePods: 9,
						},
					},
				},
			},
			taskOrderFunc: func(l, r interface{}) bool {
				return l.(*pod_info.PodInfo).Name < r.(*pod_info.PodInfo).Name
			},
			expectedDomains: []*DomainInfo{
				{
					ID:              "zone1.region1",
					Level:           "zone",
					AllocatablePods: 6,
				},
				{
					ID:              "zone2.region1",
					Level:           "zone",
					AllocatablePods: 6,
				},
				{
					ID:              "region1",
					Level:           "region",
					AllocatablePods: 9,
				},
			},
			expectedError: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			plugin := &topologyPlugin{}

			result, err := plugin.getJobAllocatableDomains(tt.job, len(podgroup_info.GetTasksToAllocate(tt.job, nil, nil, true)), tt.topologyTree)

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
			if len(result) != len(tt.expectedDomains) {
				t.Errorf("expected %d domains, but got %d", len(tt.expectedDomains), len(result))
				return
			}

			// Sort both slices by domain ID for consistent comparison
			sortDomains := func(domains []*DomainInfo) {
				sort.Slice(domains, func(i, j int) bool {
					return domains[i].ID < domains[j].ID
				})
			}
			sortDomains(result)
			sortDomains(tt.expectedDomains)

			for i, expectedDomain := range tt.expectedDomains {
				if i >= len(result) {
					t.Errorf("expected domain at index %d not found in result", i)
					continue
				}

				actualDomain := result[i]
				if actualDomain.ID != expectedDomain.ID {
					t.Errorf("domain %d: expected ID %s, got %s", i, expectedDomain.ID, actualDomain.ID)
				}
				if actualDomain.Level != expectedDomain.Level {
					t.Errorf("domain %d: expected Level %s, got %s", i, expectedDomain.Level, actualDomain.Level)
				}
				if actualDomain.AllocatablePods != expectedDomain.AllocatablePods {
					t.Errorf("domain %d: expected AllocatablePods %d, got %d", i, expectedDomain.AllocatablePods, actualDomain.AllocatablePods)
				}
			}
		})
	}
}
