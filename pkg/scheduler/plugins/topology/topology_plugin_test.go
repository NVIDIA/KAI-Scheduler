// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package topology

import (
	"context"
	"testing"

	kubeaischedulerver "github.com/NVIDIA/KAI-scheduler/pkg/apis/client/clientset/versioned/fake"
	"github.com/NVIDIA/KAI-scheduler/pkg/common/constants"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/common_info"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/node_info"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/pod_info"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/resource_info"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/cache"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/conf"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/framework"
	"github.com/stretchr/testify/assert"

	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"

	kueuev1alpha1 "sigs.k8s.io/kueue/apis/kueue/v1alpha1"
	kueuefake "sigs.k8s.io/kueue/client-go/clientset/versioned/fake"
)

func TestTopologyPlugin_initializeTopologyTree(t *testing.T) {
	fakeKubeClient := fake.NewSimpleClientset()
	fakeKubeAISchedulerClient := kubeaischedulerver.NewSimpleClientset()
	fakeKueueClient := kueuefake.NewSimpleClientset()

	testNodes := []*v1.Node{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test-node-0",
				Labels: map[string]string{
					"test-topology-label/block": "test-block-1",
					"test-topology-label/rack":  "test-rack-1",
				},
			},
			Spec: v1.NodeSpec{
				Unschedulable: false,
			},
			Status: v1.NodeStatus{
				Allocatable: v1.ResourceList{
					"cpu":                 resource.MustParse("1"),
					constants.GpuResource: resource.MustParse("1"),
				},
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test-node-1",
				Labels: map[string]string{
					"test-topology-label/block": "test-block-1",
					"test-topology-label/rack":  "test-rack-2",
				},
			},
			Spec: v1.NodeSpec{
				Unschedulable: false,
			},
			Status: v1.NodeStatus{
				Allocatable: v1.ResourceList{
					"cpu":                 resource.MustParse("1"),
					constants.GpuResource: resource.MustParse("1"),
				},
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test-node-2",
				Labels: map[string]string{
					"test-topology-label/block": "test-block-2",
					"test-topology-label/rack":  "test-rack-1",
				},
			},
			Spec: v1.NodeSpec{
				Unschedulable: false,
			},
			Status: v1.NodeStatus{
				Allocatable: v1.ResourceList{
					"cpu":                 resource.MustParse("1"),
					constants.GpuResource: resource.MustParse("3"),
				},
			},
		},
	}

	testTopology := &kueuev1alpha1.Topology{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-topology",
		},
		Spec: kueuev1alpha1.TopologySpec{
			Levels: []kueuev1alpha1.TopologyLevel{
				{
					NodeLabel: "test-topology-label/block",
				},
				{
					NodeLabel: "test-topology-label/rack",
				},
			},
		},
	}

	schedulerConfig := &conf.SchedulerConfiguration{
		Actions: "allocate",
		Tiers: []conf.Tier{
			{
				Plugins: []conf.PluginOption{
					{
						Name: "predicates",
					},
				},
			},
		},
	}

	schedulerParams := conf.SchedulerParams{
		SchedulerName:   "test-scheduler",
		PartitionParams: &conf.SchedulingNodePoolParams{},
	}

	schedulerCache := cache.New(&cache.SchedulerCacheParams{
		KubeClient:                  fakeKubeClient,
		KAISchedulerClient:          fakeKubeAISchedulerClient,
		KueueClient:                 fakeKueueClient,
		SchedulerName:               schedulerParams.SchedulerName,
		NodePoolParams:              schedulerParams.PartitionParams,
		RestrictNodeScheduling:      false,
		DetailedFitErrors:           false,
		ScheduleCSIStorage:          false,
		FullHierarchyFairness:       true,
		NodeLevelScheduler:          false,
		NumOfStatusRecordingWorkers: 1,
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var err error

	for _, node := range testNodes {
		_, err = fakeKubeClient.CoreV1().Nodes().Create(ctx, node, metav1.CreateOptions{})
		assert.NoError(t, err)
	}

	_, err = fakeKueueClient.KueueV1alpha1().Topologies().Create(ctx, testTopology, metav1.CreateOptions{})
	assert.NoError(t, err)

	schedulerCache.Run(ctx.Done())
	schedulerCache.WaitForCacheSync(ctx.Done())

	session := &framework.Session{
		Config:          schedulerConfig,
		SchedulerParams: schedulerParams,
		Cache:           schedulerCache,
		Nodes: map[string]*node_info.NodeInfo{
			testNodes[0].Name: {
				Node:        testNodes[0],
				Allocatable: resource_info.ResourceFromResourceList(testNodes[0].Status.Allocatable),
				Used:        resource_info.NewResource(500, 0, 1),
			},
			testNodes[1].Name: {
				Node:        testNodes[1],
				Allocatable: resource_info.ResourceFromResourceList(testNodes[1].Status.Allocatable),
				Used:        resource_info.NewResource(500, 0, 0),
			},
			testNodes[2].Name: {
				Node:        testNodes[2],
				Allocatable: resource_info.ResourceFromResourceList(testNodes[2].Status.Allocatable),
				Used:        resource_info.NewResource(1000, 0, 3),
			},
		},
		Topologies: []*kueuev1alpha1.Topology{testTopology},
	}

	plugin := New(nil)
	plugin.OnSessionOpen(session)

	topologyTrees := plugin.(*topologyPlugin).TopologyTrees
	assert.Equal(t, 1, len(topologyTrees))
	testTopologyObj := topologyTrees["test-topology"]
	assert.Equal(t, "test-topology", testTopologyObj.Name)
	assert.Equal(t, 5, len(testTopologyObj.Domains))

	assert.Equal(t, "test-block-1", testTopologyObj.Domains["test-block-1"].Name)
	assert.Equal(t, 1, testTopologyObj.Domains["test-block-1"].Depth)
	assert.Equal(t, "CPU: 2 (cores), memory: 0 (GB), Gpus: 2",
		testTopologyObj.Domains["test-block-1"].AvailableResources.String())
	assert.Equal(t, "CPU: 1 (cores), memory: 0 (GB), Gpus: 1",
		testTopologyObj.Domains["test-block-1"].AllocatedResources.String())

	assert.Equal(t, "test-rack-1", testTopologyObj.Domains["test-block-1.test-rack-1"].Name)
	assert.Equal(t, "test-block-1", testTopologyObj.Domains["test-block-1.test-rack-1"].Parent.Name)
	assert.Equal(t, 2, testTopologyObj.Domains["test-block-1.test-rack-1"].Depth)
	assert.Equal(t, "CPU: 1 (cores), memory: 0 (GB), Gpus: 1",
		testTopologyObj.Domains["test-block-1.test-rack-1"].AvailableResources.String())
	assert.Equal(t, "CPU: 0.5 (cores), memory: 0 (GB), Gpus: 1",
		testTopologyObj.Domains["test-block-1.test-rack-1"].AllocatedResources.String())

	assert.Equal(t, "test-rack-2", testTopologyObj.Domains["test-block-1.test-rack-2"].Name)
	assert.Equal(t, "test-block-1", testTopologyObj.Domains["test-block-1.test-rack-2"].Parent.Name)
	assert.Equal(t, 2, testTopologyObj.Domains["test-block-1.test-rack-2"].Depth)
	assert.Equal(t, "CPU: 1 (cores), memory: 0 (GB), Gpus: 1",
		testTopologyObj.Domains["test-block-1.test-rack-2"].AvailableResources.String())
	assert.Equal(t, "CPU: 0.5 (cores), memory: 0 (GB), Gpus: 0",
		testTopologyObj.Domains["test-block-1.test-rack-2"].AllocatedResources.String())

	assert.Equal(t, "test-block-2", testTopologyObj.Domains["test-block-2"].Name)
	assert.Equal(t, 1, testTopologyObj.Domains["test-block-2"].Depth)
	assert.Equal(t, "CPU: 1 (cores), memory: 0 (GB), Gpus: 3",
		testTopologyObj.Domains["test-block-2"].AvailableResources.String())
	assert.Equal(t, "CPU: 1 (cores), memory: 0 (GB), Gpus: 3",
		testTopologyObj.Domains["test-block-2"].AllocatedResources.String())

	assert.Equal(t, "test-rack-1", testTopologyObj.Domains["test-block-2.test-rack-1"].Name)
	assert.Equal(t, "test-block-2", testTopologyObj.Domains["test-block-2.test-rack-1"].Parent.Name)
	assert.Equal(t, 2, testTopologyObj.Domains["test-block-2.test-rack-1"].Depth)
	assert.Equal(t, "CPU: 1 (cores), memory: 0 (GB), Gpus: 3",
		testTopologyObj.Domains["test-block-2.test-rack-1"].AvailableResources.String())
	assert.Equal(t, "CPU: 1 (cores), memory: 0 (GB), Gpus: 3",
		testTopologyObj.Domains["test-block-2.test-rack-1"].AllocatedResources.String())
}

func TestTopologyPlugin_HandleAllocate(t *testing.T) {
	tests := []struct {
		name           string
		setupTopology  func() *topologyPlugin
		setupEvent     func() *framework.Event
		podInfos       map[common_info.PodID]*pod_info.PodInfo
		expectedAllocs map[string]map[string]int64 // topologyName -> domainID -> expected CPU allocation
	}{
		{
			name: "SingleTopologySingleDomain",
			setupTopology: func() *topologyPlugin {
				plugin := &topologyPlugin{
					enabled:       true,
					TopologyTrees: make(map[string]*TopologyInfo),
				}

				// Create topology resource
				topology := &kueuev1alpha1.Topology{
					ObjectMeta: metav1.ObjectMeta{Name: "test-topology"},
					Spec: kueuev1alpha1.TopologySpec{
						Levels: []kueuev1alpha1.TopologyLevel{
							{NodeLabel: "zone"},
						},
					},
				}

				// Create domain info
				domainInfo := NewTopologyDomainInfo("zone1", "zone1", "zone", 1)
				domainInfo.AvailableResources = resource_info.NewResource(4000, 0, 0)
				domainInfo.AllocatedResources = resource_info.EmptyResource()
				domainInfo.AllocatedResources.BaseResource.ScalarResources()["pods"] = 0

				// Create topology tree
				topologyTree := &TopologyInfo{
					Name:             "test-topology",
					Domains:          map[TopologyDomainID]*TopologyDomainInfo{"zone1": domainInfo},
					Root:             NewTopologyDomainInfo("root", "datacenter", "cluster", 0),
					TopologyResource: topology,
				}

				// Set parent relationship
				domainInfo.Parent = topologyTree.Root

				plugin.TopologyTrees["test-topology"] = topologyTree
				return plugin
			},
			setupEvent: func() *framework.Event {
				return &framework.Event{
					Task: &pod_info.PodInfo{
						Pod: &corev1.Pod{
							ObjectMeta: metav1.ObjectMeta{Name: "test-pod"},
							Spec: corev1.PodSpec{
								NodeName: "node1",
							},
						},
						NodeName: "node1",
					},
				}
			},
			podInfos: map[common_info.PodID]*pod_info.PodInfo{
				"test-pod": {
					AcceptedResource: resource_info.NewResourceRequirements(0, 1000, 0),
				},
			},
			expectedAllocs: map[string]map[string]int64{
				"test-topology": {
					"zone1": 1000, // Expect 1 CPU allocation
				},
			},
		},
		{
			name: "MultiLevelTopology",
			setupTopology: func() *topologyPlugin {
				plugin := &topologyPlugin{
					enabled:       true,
					TopologyTrees: make(map[string]*TopologyInfo),
				}

				// Create topology resource
				topology := &kueuev1alpha1.Topology{
					ObjectMeta: metav1.ObjectMeta{Name: "multi-topology"},
					Spec: kueuev1alpha1.TopologySpec{
						Levels: []kueuev1alpha1.TopologyLevel{
							{NodeLabel: "zone"},
							{NodeLabel: "rack"},
						},
					},
				}

				// Create domain hierarchy
				zoneDomain := NewTopologyDomainInfo("zone1", "zone1", "zone", 1)
				zoneDomain.AvailableResources = resource_info.NewResource(8, 16, 0)
				zoneDomain.AllocatedResources = resource_info.EmptyResource()
				zoneDomain.AllocatedResources.BaseResource.ScalarResources()["pods"] = 0

				rackDomain := NewTopologyDomainInfo("zone1.rack1", "rack1", "rack", 2)
				rackDomain.AvailableResources = resource_info.NewResource(4, 8, 0)
				rackDomain.AllocatedResources = resource_info.EmptyResource()
				rackDomain.AllocatedResources.BaseResource.ScalarResources()["pods"] = 0

				// Create topology tree
				topologyTree := &TopologyInfo{
					Name: "multi-topology",
					Domains: map[TopologyDomainID]*TopologyDomainInfo{
						"zone1":       zoneDomain,
						"zone1.rack1": rackDomain,
					},
					Root:             NewTopologyDomainInfo("root", "datacenter", "cluster", 0),
					TopologyResource: topology,
				}

				// Set parent relationships
				rackDomain.Parent = zoneDomain
				zoneDomain.Parent = topologyTree.Root

				plugin.TopologyTrees["multi-topology"] = topologyTree
				return plugin
			},
			setupEvent: func() *framework.Event {
				return &framework.Event{
					Task: &pod_info.PodInfo{
						Pod: &corev1.Pod{
							ObjectMeta: metav1.ObjectMeta{Name: "test-pod"},
							Spec: corev1.PodSpec{
								NodeName: "node1",
							},
						},
						NodeName: "node1",
					},
				}
			},
			podInfos: map[common_info.PodID]*pod_info.PodInfo{
				"test-pod": {
					AcceptedResource: resource_info.NewResourceRequirements(0, 1000, 0),
				},
			},
			expectedAllocs: map[string]map[string]int64{
				"multi-topology": {
					"zone1":       1000, // Expect 1 CPU allocation in zone
					"zone1.rack1": 1000, // Expect 1 CPU allocation in rack
				},
			},
		},
		{
			name: "MultipleTopologies",
			setupTopology: func() *topologyPlugin {
				plugin := &topologyPlugin{
					enabled:       true,
					TopologyTrees: make(map[string]*TopologyInfo),
				}

				// Create first topology
				topology1 := &kueuev1alpha1.Topology{
					ObjectMeta: metav1.ObjectMeta{Name: "topology-1"},
					Spec: kueuev1alpha1.TopologySpec{
						Levels: []kueuev1alpha1.TopologyLevel{
							{NodeLabel: "zone"},
						},
					},
				}

				domain1 := NewTopologyDomainInfo("zone1", "zone1", "zone", 1)
				domain1.AvailableResources = resource_info.NewResource(4, 8, 0)
				domain1.AllocatedResources = resource_info.EmptyResource()
				domain1.AllocatedResources.BaseResource.ScalarResources()["pods"] = 0

				topologyTree1 := &TopologyInfo{
					Name:             "topology-1",
					Domains:          map[TopologyDomainID]*TopologyDomainInfo{"zone1": domain1},
					Root:             NewTopologyDomainInfo("root", "datacenter", "cluster", 0),
					TopologyResource: topology1,
				}
				domain1.Parent = topologyTree1.Root

				// Create second topology
				topology2 := &kueuev1alpha1.Topology{
					ObjectMeta: metav1.ObjectMeta{Name: "topology-2"},
					Spec: kueuev1alpha1.TopologySpec{
						Levels: []kueuev1alpha1.TopologyLevel{
							{NodeLabel: "region"},
						},
					},
				}

				domain2 := NewTopologyDomainInfo("region1", "region1", "region", 1)
				domain2.AvailableResources = resource_info.NewResource(8, 16, 0)
				domain2.AllocatedResources = resource_info.EmptyResource()
				domain2.AllocatedResources.BaseResource.ScalarResources()["pods"] = 0

				topologyTree2 := &TopologyInfo{
					Name:             "topology-2",
					Domains:          map[TopologyDomainID]*TopologyDomainInfo{"region1": domain2},
					Root:             NewTopologyDomainInfo("root", "datacenter", "cluster", 0),
					TopologyResource: topology2,
				}
				domain2.Parent = topologyTree2.Root

				plugin.TopologyTrees["topology-1"] = topologyTree1
				plugin.TopologyTrees["topology-2"] = topologyTree2
				return plugin
			},
			setupEvent: func() *framework.Event {
				return &framework.Event{
					Task: &pod_info.PodInfo{
						Pod: &corev1.Pod{
							ObjectMeta: metav1.ObjectMeta{Name: "test-pod"},
							Spec: corev1.PodSpec{
								NodeName: "node1",
							},
						},
						NodeName: "node1",
					},
				}
			},
			podInfos: map[common_info.PodID]*pod_info.PodInfo{
				"test-pod": {
					AcceptedResource: resource_info.NewResourceRequirements(0, 1000, 0),
				},
			},
			expectedAllocs: map[string]map[string]int64{
				"topology-1": {
					"zone1": 1000, // Expect 1 CPU allocation
				},
				"topology-2": {
					"region1": 1000, // Expect 1 CPU allocation
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup topology plugin
			plugin := tt.setupTopology()

			// Create mock session
			ssn := &framework.Session{
				Nodes: map[string]*node_info.NodeInfo{
					"node1": {
						Node: &corev1.Node{
							ObjectMeta: metav1.ObjectMeta{
								Name: "node1",
								Labels: map[string]string{
									"zone":   "zone1",
									"rack":   "rack1",
									"region": "region1",
								},
							},
						},
						PodInfos: tt.podInfos,
					},
				},
			}

			// Create event
			event := tt.setupEvent()

			// Get the allocate handler
			allocateHandler := plugin.handleAllocate(ssn)

			// Execute the handler
			allocateHandler(event)

			// Verify allocations
			for topologyName, expectedDomains := range tt.expectedAllocs {
				topologyTree, exists := plugin.TopologyTrees[topologyName]
				assert.True(t, exists, "Topology %s should exist", topologyName)

				for domainID, expectedCPU := range expectedDomains {
					domainInfo, exists := topologyTree.Domains[TopologyDomainID(domainID)]
					assert.True(t, exists, "Domain %s should exist in topology %s", domainID, topologyName)
					assert.Equal(t, expectedCPU, int64(domainInfo.AllocatedResources.Cpu()),
						"Expected %d CPU allocation in domain %s of topology %s", expectedCPU, domainID, topologyName)
					assert.Equal(t, int64(1), domainInfo.AllocatedResources.BaseResource.ScalarResources()["pods"],
						"Expected 1 allocated pod in domain %s of topology %s", domainID, topologyName)
				}
			}
		})
	}
}

func TestTopologyPlugin_HandleDeallocate(t *testing.T) {
	tests := []struct {
		name            string
		setupTopology   func() *topologyPlugin
		setupEvent      func() *framework.Event
		podInfos        map[common_info.PodID]*pod_info.PodInfo
		expectedAllocs  map[string]map[string]int64 // topologyName -> domainID -> expected CPU allocation
		expectedPodsNum map[string]map[string]int
	}{
		{
			name: "SingleTopologySingleDomain",
			setupTopology: func() *topologyPlugin {
				plugin := &topologyPlugin{
					enabled:       true,
					TopologyTrees: make(map[string]*TopologyInfo),
				}

				// Create topology resource
				topology := &kueuev1alpha1.Topology{
					ObjectMeta: metav1.ObjectMeta{Name: "test-topology"},
					Spec: kueuev1alpha1.TopologySpec{
						Levels: []kueuev1alpha1.TopologyLevel{
							{NodeLabel: "zone"},
						},
					},
				}

				// Create domain info
				domainInfo := NewTopologyDomainInfo("zone1", "zone1", "zone", 1)
				domainInfo.AvailableResources = resource_info.NewResource(4000, 0, 0)
				domainInfo.AllocatedResources = resource_info.NewResource(1000, 0, 0)
				domainInfo.AllocatedResources.BaseResource.ScalarResources()["pods"] = 1

				// Create topology tree
				topologyTree := &TopologyInfo{
					Name:             "test-topology",
					Domains:          map[TopologyDomainID]*TopologyDomainInfo{"zone1": domainInfo},
					Root:             NewTopologyDomainInfo("root", "datacenter", "cluster", 0),
					TopologyResource: topology,
				}

				// Set parent relationship
				domainInfo.Parent = topologyTree.Root

				plugin.TopologyTrees["test-topology"] = topologyTree
				return plugin
			},
			setupEvent: func() *framework.Event {
				return &framework.Event{
					Task: &pod_info.PodInfo{
						Pod: &corev1.Pod{
							ObjectMeta: metav1.ObjectMeta{Name: "test-pod"},
							Spec: corev1.PodSpec{
								NodeName: "node1",
							},
						},
						NodeName: "node1",
					},
				}
			},
			podInfos: map[common_info.PodID]*pod_info.PodInfo{
				"test-pod": {
					AcceptedResource: resource_info.NewResourceRequirements(0, 1000, 0),
				},
			},
			expectedAllocs: map[string]map[string]int64{
				"test-topology": {
					"zone1": 0, // Expect 1 CPU allocation
				},
			},
			expectedPodsNum: map[string]map[string]int{
				"test-topology": {
					"zone1": 0,
				},
			},
		},
		{
			name: "MultiLevelTopology",
			setupTopology: func() *topologyPlugin {
				plugin := &topologyPlugin{
					enabled:       true,
					TopologyTrees: make(map[string]*TopologyInfo),
				}

				// Create topology resource
				topology := &kueuev1alpha1.Topology{
					ObjectMeta: metav1.ObjectMeta{Name: "multi-topology"},
					Spec: kueuev1alpha1.TopologySpec{
						Levels: []kueuev1alpha1.TopologyLevel{
							{NodeLabel: "zone"},
							{NodeLabel: "rack"},
						},
					},
				}

				// Create domain hierarchy
				zoneDomain := NewTopologyDomainInfo("zone1", "zone1", "zone", 1)
				zoneDomain.AvailableResources = resource_info.NewResource(8, 16, 0)
				zoneDomain.AllocatedResources = resource_info.NewResource(3000, 0, 0)
				zoneDomain.AllocatedResources.BaseResource.ScalarResources()["pods"] = 2

				rackDomain := NewTopologyDomainInfo("zone1.rack1", "rack1", "rack", 2)
				rackDomain.AvailableResources = resource_info.NewResource(4, 8, 0)
				rackDomain.AllocatedResources = resource_info.NewResource(2000, 0, 0)
				rackDomain.AllocatedResources.BaseResource.ScalarResources()["pods"] = 1

				// Create topology tree
				topologyTree := &TopologyInfo{
					Name: "multi-topology",
					Domains: map[TopologyDomainID]*TopologyDomainInfo{
						"zone1":       zoneDomain,
						"zone1.rack1": rackDomain,
					},
					Root:             NewTopologyDomainInfo("root", "datacenter", "cluster", 0),
					TopologyResource: topology,
				}

				// Set parent relationships
				rackDomain.Parent = zoneDomain
				zoneDomain.Parent = topologyTree.Root

				plugin.TopologyTrees["multi-topology"] = topologyTree
				return plugin
			},
			setupEvent: func() *framework.Event {
				return &framework.Event{
					Task: &pod_info.PodInfo{
						Pod: &corev1.Pod{
							ObjectMeta: metav1.ObjectMeta{Name: "test-pod"},
							Spec: corev1.PodSpec{
								NodeName: "node1",
							},
						},
						NodeName: "node1",
					},
				}
			},
			podInfos: map[common_info.PodID]*pod_info.PodInfo{
				"test-pod": {
					AcceptedResource: resource_info.NewResourceRequirements(0, 1000, 0),
				},
			},
			expectedAllocs: map[string]map[string]int64{
				"multi-topology": {
					"zone1":       2000, // Expect 1 CPU allocation in zone
					"zone1.rack1": 1000, // Expect 1 CPU allocation in rack
				},
			},
			expectedPodsNum: map[string]map[string]int{
				"multi-topology": {
					"zone1":       1,
					"zone1.rack1": 0,
				},
			},
		},
		{
			name: "MultipleTopologies",
			setupTopology: func() *topologyPlugin {
				plugin := &topologyPlugin{
					enabled:       true,
					TopologyTrees: make(map[string]*TopologyInfo),
				}

				// Create first topology
				topology1 := &kueuev1alpha1.Topology{
					ObjectMeta: metav1.ObjectMeta{Name: "topology-1"},
					Spec: kueuev1alpha1.TopologySpec{
						Levels: []kueuev1alpha1.TopologyLevel{
							{NodeLabel: "zone"},
						},
					},
				}

				domain1 := NewTopologyDomainInfo("zone1", "zone1", "zone", 1)
				domain1.AvailableResources = resource_info.NewResource(4, 8, 0)
				domain1.AllocatedResources = resource_info.NewResource(2000, 0, 0)
				domain1.AllocatedResources.BaseResource.ScalarResources()["pods"] = 5

				topologyTree1 := &TopologyInfo{
					Name:             "topology-1",
					Domains:          map[TopologyDomainID]*TopologyDomainInfo{"zone1": domain1},
					Root:             NewTopologyDomainInfo("root", "datacenter", "cluster", 0),
					TopologyResource: topology1,
				}
				domain1.Parent = topologyTree1.Root

				// Create second topology
				topology2 := &kueuev1alpha1.Topology{
					ObjectMeta: metav1.ObjectMeta{Name: "topology-2"},
					Spec: kueuev1alpha1.TopologySpec{
						Levels: []kueuev1alpha1.TopologyLevel{
							{NodeLabel: "region"},
						},
					},
				}

				domain2 := NewTopologyDomainInfo("region1", "region1", "region", 1)
				domain2.AvailableResources = resource_info.NewResource(8, 16, 0)
				domain2.AllocatedResources = resource_info.NewResource(1000, 0, 0)
				domain2.AllocatedResources.BaseResource.ScalarResources()["pods"] = 2

				topologyTree2 := &TopologyInfo{
					Name:             "topology-2",
					Domains:          map[TopologyDomainID]*TopologyDomainInfo{"region1": domain2},
					Root:             NewTopologyDomainInfo("root", "datacenter", "cluster", 0),
					TopologyResource: topology2,
				}
				domain2.Parent = topologyTree2.Root

				plugin.TopologyTrees["topology-1"] = topologyTree1
				plugin.TopologyTrees["topology-2"] = topologyTree2
				return plugin
			},
			setupEvent: func() *framework.Event {
				return &framework.Event{
					Task: &pod_info.PodInfo{
						Pod: &corev1.Pod{
							ObjectMeta: metav1.ObjectMeta{Name: "test-pod"},
							Spec: corev1.PodSpec{
								NodeName: "node1",
							},
						},
						NodeName: "node1",
					},
				}
			},
			podInfos: map[common_info.PodID]*pod_info.PodInfo{
				"test-pod": {
					AcceptedResource: resource_info.NewResourceRequirements(0, 1000, 0),
				},
			},
			expectedAllocs: map[string]map[string]int64{
				"topology-1": {
					"zone1": 1000, // Expect 1 CPU allocation
				},
				"topology-2": {
					"region1": 0, // Expect 1 CPU allocation
				},
			},
			expectedPodsNum: map[string]map[string]int{
				"topology-1": {
					"zone1": 4,
				},
				"topology-2": {
					"region1": 1,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup topology plugin
			plugin := tt.setupTopology()

			// Create mock session
			ssn := &framework.Session{
				Nodes: map[string]*node_info.NodeInfo{
					"node1": {
						Node: &corev1.Node{
							ObjectMeta: metav1.ObjectMeta{
								Name: "node1",
								Labels: map[string]string{
									"zone":   "zone1",
									"rack":   "rack1",
									"region": "region1",
								},
							},
						},
						PodInfos: tt.podInfos,
					},
				},
			}

			// Create event
			event := tt.setupEvent()

			// Get the allocate handler
			deallocateHandler := plugin.handleDeallocate(ssn)

			// Execute the handler
			deallocateHandler(event)

			// Verify allocations
			for topologyName, expectedDomains := range tt.expectedAllocs {
				topologyTree, exists := plugin.TopologyTrees[topologyName]
				assert.True(t, exists, "Topology %s should exist", topologyName)

				for domainID, expectedCPU := range expectedDomains {
					domainInfo, exists := topologyTree.Domains[TopologyDomainID(domainID)]
					assert.True(t, exists, "Domain %s should exist in topology %s", domainID, topologyName)
					assert.Equal(t, expectedCPU, int64(domainInfo.AllocatedResources.Cpu()),
						"Expected %d CPU allocation in domain %s of topology %s", expectedCPU, domainID, topologyName)
				}
			}

			// Verify pods
			for topologyName, expectedDomains := range tt.expectedPodsNum {
				topologyTree, exists := plugin.TopologyTrees[topologyName]
				assert.True(t, exists, "Topology %s should exist", topologyName)

				for domainID, expectedPodsNum := range expectedDomains {
					domainInfo, exists := topologyTree.Domains[TopologyDomainID(domainID)]
					assert.True(t, exists, "Domain %s should exist in topology %s", domainID, topologyName)
					assert.Equal(t, int64(expectedPodsNum), domainInfo.AllocatedResources.BaseResource.ScalarResources()["pods"],
						"Expected 1 allocated pod in domain %s of topology %s", domainID, topologyName)
				}
			}
		})
	}
}
