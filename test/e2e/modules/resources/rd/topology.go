// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package rd

import (
	"context"
	"fmt"
	rand "math/rand/v2"
	"slices"

	"github.com/NVIDIA/KAI-scheduler/pkg/common/constants"
	"github.com/NVIDIA/KAI-scheduler/test/e2e/modules/resources/capacity"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	kueuev1alpha1 "sigs.k8s.io/kueue/apis/kueue/v1alpha1"
	kueue "sigs.k8s.io/kueue/client-go/clientset/versioned"
)

const (
	TestZoneLabelKey = "e2e-topology-label/zone"
	TestRackLabelKey = "e2e-topology-label/rack"
	NodeNameLabelKey = "kubernetes.io/hostname"

	numNodesInTestTopology = 4
)

type TestTopologyData struct {
	TopologyCrd   *kueuev1alpha1.Topology
	TopologyNodes map[string]*corev1.Node
	Zones         map[string][]*corev1.Node
	Racks         map[string][]*corev1.Node
}

func CreateRackZoneTopology(
	ctx context.Context, kubeClientset *kubernetes.Clientset, kubeConfig *rest.Config) (TestTopologyData, []string) {
	testTopologyData := TestTopologyData{
		TopologyCrd:   nil,
		TopologyNodes: make(map[string]*corev1.Node),
		Zones:         map[string][]*corev1.Node{"zone1": {}},
		Racks:         map[string][]*corev1.Node{"rack1": {}, "rack2": {}},
	}

	requiredNodesResources := []capacity.ResourceList{}
	for range numNodesInTestTopology {
		requiredNodesResources = append(requiredNodesResources, capacity.ResourceList{Gpu: resource.MustParse("1")})
	}
	capacity.SkipIfInsufficientClusterTopologyResources(kubeClientset, requiredNodesResources)

	// Create topology tree
	testTopologyData.TopologyCrd = &kueuev1alpha1.Topology{
		ObjectMeta: metav1.ObjectMeta{
			Name: "e2e-topology-tree",
		},
		Spec: kueuev1alpha1.TopologySpec{
			Levels: []kueuev1alpha1.TopologyLevel{
				{NodeLabel: TestZoneLabelKey},
				{NodeLabel: TestRackLabelKey},
				{NodeLabel: NodeNameLabelKey},
			},
		},
	}
	kueueClient := kueue.NewForConfigOrDie(kubeConfig)
	_, err := kueueClient.KueueV1alpha1().Topologies().Create(
		context.TODO(), testTopologyData.TopologyCrd, metav1.CreateOptions{})
	Expect(err).NotTo(HaveOccurred(), "Failed to create topology tree")

	nodes, err := kubeClientset.CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{})
	Expect(err).NotTo(HaveOccurred(), "Failed to list nodes")

	gpuNodesMap := map[int][]string{}
	for _, node := range nodes.Items {
		gpuCountQuantity := node.Status.Allocatable[v1.ResourceName(constants.GpuResource)]
		gpuCount := int(gpuCountQuantity.Value())
		if gpuCount == 0 {
			continue // skip nodes without GPUs
		}
		if gpuNodesMap[gpuCount] == nil {
			gpuNodesMap[gpuCount] = []string{}
		}
		gpuNodesMap[gpuCount] = append(gpuNodesMap[gpuCount], node.Name)
	}

	for _, nodes := range gpuNodesMap {
		if len(nodes) >= numNodesInTestTopology {
			return testTopologyData, nodes
		}
	}

	Fail("Not enough nodes with an equal amount of GPUs in cluster for the test. Required: %d", numNodesInTestTopology)
	return testTopologyData, nil
}

func AssignNodesToTestTopology(ctx context.Context,
	kubeClient client.Client,
	nodesNamesList []string,
	testTopologyData TestTopologyData,
) {
	selectedTopologyNodesNames := shuffleNodes(nodesNamesList)
	selectedTopologyNodesNames = slices.Clone(selectedTopologyNodesNames[:numNodesInTestTopology])
	for _, nodeName := range selectedTopologyNodesNames {
		node := &corev1.Node{}
		err := kubeClient.Get(ctx, client.ObjectKey{Name: nodeName}, node)
		Expect(err).NotTo(HaveOccurred(), "Failed to get node fresh copy for label cleaning")

		originalNode := node.DeepCopy()
		testTopologyData.TopologyNodes[node.Name] = node

		chosenZone := len(selectedTopologyNodesNames) % len(testTopologyData.Zones)
		zoneName := fmt.Sprintf("zone%d", chosenZone)
		testTopologyData.Zones[zoneName] = append(testTopologyData.Zones[zoneName], node)
		node.Labels[TestZoneLabelKey] = zoneName

		chosenRack := len(selectedTopologyNodesNames) % len(testTopologyData.Racks)
		rackName := fmt.Sprintf("rack%d", chosenRack)
		testTopologyData.Racks[rackName] = append(testTopologyData.Racks[rackName], node)
		node.Labels[TestRackLabelKey] = rackName

		err = kubeClient.Patch(ctx, node, client.MergeFrom(originalNode))
		Expect(err).NotTo(HaveOccurred(), "Failed to add topology labels to nodes")
	}
}

func CleanNodesFromTopology(ctx context.Context,
	kubeClient client.Client,
	testTopologyData TestTopologyData,
) {
	labelsToClean := []string{}
	for _, label := range testTopologyData.TopologyCrd.Spec.Levels {
		if label.NodeLabel != NodeNameLabelKey {
			labelsToClean = append(labelsToClean, label.NodeLabel)
		}
	}

	for _, nodeObj := range testTopologyData.TopologyNodes {
		node := &corev1.Node{}
		err := kubeClient.Get(ctx, client.ObjectKey{Name: nodeObj.Name}, node)
		Expect(err).NotTo(HaveOccurred(), "Failed to get node fresh copy for label cleaning")

		originalNode := node.DeepCopy()
		for _, label := range labelsToClean {
			delete(node.Labels, label)
		}
		err = kubeClient.Patch(ctx, node, client.MergeFrom(originalNode))
		Expect(err).NotTo(HaveOccurred(), "Failed to clean topology labels from nodes")
	}
}

func CleanRackZoneTopology(ctx context.Context, testTopologyData TestTopologyData, kubeConfig *rest.Config) {

	kueueClient := kueue.NewForConfigOrDie(kubeConfig)
	err := kueueClient.KueueV1alpha1().Topologies().Delete(
		context.TODO(), testTopologyData.TopologyCrd.Name, metav1.DeleteOptions{})
	Expect(err).NotTo(HaveOccurred(), "Failed to delete test topology tree %s", testTopologyData.TopologyCrd.Name)
}

func shuffleNodes(nodes []string) []string {
	rand.Shuffle(len(nodes), func(i, j int) {
		nodes[i], nodes[j] = nodes[j], nodes[i]
	})

	return nodes
}
