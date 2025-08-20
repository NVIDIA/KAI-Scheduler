/*
Copyright 2025 NVIDIA CORPORATION
SPDX-License-Identifier: Apache-2.0
*/

package topology

import (
	"context"
	"fmt"
	"math/rand"

	v2 "github.com/NVIDIA/KAI-scheduler/pkg/apis/scheduling/v2"
	"github.com/NVIDIA/KAI-scheduler/pkg/apis/scheduling/v2alpha2"
	testcontext "github.com/NVIDIA/KAI-scheduler/test/e2e/modules/context"
	"github.com/NVIDIA/KAI-scheduler/test/e2e/modules/resources/capacity"
	"github.com/NVIDIA/KAI-scheduler/test/e2e/modules/resources/rd"
	"github.com/NVIDIA/KAI-scheduler/test/e2e/modules/resources/rd/pod_group"
	"github.com/NVIDIA/KAI-scheduler/test/e2e/modules/resources/rd/queue"
	"github.com/NVIDIA/KAI-scheduler/test/e2e/modules/utils"
	"github.com/NVIDIA/KAI-scheduler/test/e2e/modules/wait"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kueuev1alpha1 "sigs.k8s.io/kueue/apis/kueue/v1alpha1"
	kueue "sigs.k8s.io/kueue/client-go/clientset/versioned"
)

var _ = Describe("Topology", Ordered, func() {
	var (
		testCtx       *testcontext.TestContext
		nodeGpusCount int
		topologyNodes map[string]*corev1.Node
		zonesMap      map[string][]*corev1.Node
		racksMap      map[string][]*corev1.Node
	)

	BeforeAll(func(ctx context.Context) {
		testCtx = testcontext.GetConnectivity(ctx, Default)
		parentQueue := queue.CreateQueueObject(utils.GenerateRandomK8sName(10), "")
		childQueue := queue.CreateQueueObject(utils.GenerateRandomK8sName(10), parentQueue.Name)
		testCtx.InitQueues([]*v2.Queue{childQueue, parentQueue})
		capacity.SkipIfInsufficientClusterTopologyResources(testCtx.KubeClientset, []capacity.ResourceList{
			{PodCount: 4, Gpu: resource.MustParse("1")}, {PodCount: 4, Gpu: resource.MustParse("1")},
			{PodCount: 4, Gpu: resource.MustParse("1")}, {PodCount: 4, Gpu: resource.MustParse("1")},
		})

		nodes, err := testCtx.KubeClientset.CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{})
		Expect(err).NotTo(HaveOccurred(), "Failed to list nodes")
		nodesList := []*corev1.Node{}
		nodeGpusCount = -1
		for _, node := range nodes.Items {
			gpuCount := node.Status.Allocatable[v1.ResourceName("nvidia.com/gpu")]
			if gpuCount.Value() > 0 {
				if nodeGpusCount == -1 {
					nodeGpusCount = int(gpuCount.Value())
				} else if int(gpuCount.Value()) != nodeGpusCount {
					continue
				}
				nodesList = append(nodesList, &node)
			}
		}
		Expect(len(nodesList)).To(BeNumerically(">=", 4),
			fmt.Sprintf("Not enough nodes with equal amount of GPUs(%d) in cluster", nodeGpusCount))

		selectedTopologyNodes := chooseRandomNodes(nodesList, 4)
		topologyNodes = make(map[string]*corev1.Node)
		for _, node := range selectedTopologyNodes {
			topologyNodes[node.Name] = node
		}
		zonesMap = map[string][]*corev1.Node{
			"zone1": selectedTopologyNodes,
		}
		racksMap = map[string][]*corev1.Node{
			"rack1": {selectedTopologyNodes[0], selectedTopologyNodes[2]},
			"rack2": {selectedTopologyNodes[1], selectedTopologyNodes[3]},
		}

		// Add topology labels to the nodes
		for zoneName, zoneNodes := range zonesMap {
			for _, node := range zoneNodes {
				node.Labels["e2e-topology-label/zone"] = zoneName
			}
		}
		for rackName, rackNodes := range racksMap {
			for _, node := range rackNodes {
				node.Labels["e2e-topology-label/rack"] = rackName
			}
		}

		for _, node := range selectedTopologyNodes {
			_, err = testCtx.KubeClientset.CoreV1().Nodes().Update(context.TODO(), node, metav1.UpdateOptions{})
			Expect(err).NotTo(HaveOccurred(), "Failed to update nodes")
		}

		// Create topology tree
		treeCrd := &kueuev1alpha1.Topology{
			ObjectMeta: metav1.ObjectMeta{
				Name: "e2e-topology-tree",
			},
			Spec: kueuev1alpha1.TopologySpec{
				Levels: []kueuev1alpha1.TopologyLevel{
					{NodeLabel: "e2e-topology-label/zone"},
					{NodeLabel: "e2e-topology-label/rack"},
					{NodeLabel: "kubernetes.io/hostname"},
				},
			},
		}

		kueueClient := kueue.NewForConfigOrDie(testCtx.KubeConfig)
		_, err = kueueClient.KueueV1alpha1().Topologies().Create(context.TODO(), treeCrd, metav1.CreateOptions{})
		Expect(err).NotTo(HaveOccurred(), "Failed to create topology tree")
	})

	AfterAll(func(ctx context.Context) {
		testCtx.ClusterCleanup(ctx)

		kueueClient := kueue.NewForConfigOrDie(testCtx.KubeConfig)
		err := kueueClient.KueueV1alpha1().Topologies().Delete(context.TODO(), "e2e-topology-tree", metav1.DeleteOptions{})
		Expect(err).NotTo(HaveOccurred(), "Failed to delete topology tree")

		// clean nodes topology labels
		for _, nodeObj := range topologyNodes {
			node, err := testCtx.KubeClientset.CoreV1().Nodes().Get(context.TODO(), nodeObj.Name, metav1.GetOptions{})
			Expect(err).NotTo(HaveOccurred(), "Failed to get node for label cleaning")
			delete(node.Labels, "e2e-topology-label/zone")
			delete(node.Labels, "e2e-topology-label/rack")
			_, err = testCtx.KubeClientset.CoreV1().Nodes().Update(context.TODO(), node, metav1.UpdateOptions{})
			Expect(err).NotTo(HaveOccurred(), "Failed to update nodes")
		}
	})

	Context("Topology", func() {
		It("should allocate pods to nodes in the topology", func(ctx context.Context) {
			namespace := queue.GetConnectedNamespaceToQueue(testCtx.Queues[0])
			queueName := testCtx.Queues[0].Name

			// Make sure
			podCount := 2
			podResource := v1.ResourceList{
				v1.ResourceName("nvidia.com/gpu"): resource.MustParse(fmt.Sprintf("%d", nodeGpusCount)),
			}

			podGroup := pod_group.Create(
				namespace, "distributed-pod-group"+utils.GenerateRandomK8sName(10), queueName)
			podGroup.Spec.MinMember = int32(podCount)
			podGroup.Spec.TopologyConstraint = v2alpha2.TopologyConstraint{
				RequiredTopologyLevel: "e2e-topology-label/rack",
				Topology:              "e2e-topology-tree",
			}

			pods := []*v1.Pod{}

			Expect(testCtx.ControllerClient.Create(ctx, podGroup)).To(Succeed())
			for i := 0; i < podCount; i++ {
				pod := rd.CreatePodObject(testCtx.Queues[0], v1.ResourceRequirements{Requests: podResource, Limits: podResource})
				pod.Name = "distributed-pod-" + utils.GenerateRandomK8sName(10)
				pod.Annotations[pod_group.PodGroupNameAnnotation] = podGroup.Name
				pod.Labels[pod_group.PodGroupNameAnnotation] = podGroup.Name
				_, err := rd.CreatePod(ctx, testCtx.KubeClientset, pod)
				Expect(err).To(Succeed())
				pods = append(pods, pod)
			}

			wait.ForPodsScheduled(ctx, testCtx.ControllerClient, namespace, pods)

			// Validate that all the pods have been scheduled to the same rack
			podList, err := testCtx.KubeClientset.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{})
			Expect(err).NotTo(HaveOccurred(), "Failed to list pods")

			scheduledRacks := map[string][]string{}
			for _, pod := range podList.Items {
				podRack := topologyNodes[pod.Spec.NodeName].Labels["e2e-topology-label/rack"]
				scheduledRacks[podRack] = append(scheduledRacks[podRack], pod.Name)
			}

			Expect(len(scheduledRacks)).To(Equal(1), "Expected all pods scheduled to one rack, got %v", scheduledRacks)
		}) // , FlakeAttempts(3) - make each time to choose a different rack? randomly choose a node an fill it.
	})
})

// chooseRandomNodes selects n random nodes from the cluster
func chooseRandomNodes(baseNodes []*corev1.Node, n int) []*corev1.Node {

	// Ensure we have enough nodes
	Expect(len(baseNodes)).To(BeNumerically(">=", n),
		"Not enough available nodes in cluster. Need %d, have %d", n, len(baseNodes))

	// Shuffle the nodes and take the first n
	rand.Shuffle(len(baseNodes), func(i, j int) {
		baseNodes[i], baseNodes[j] = baseNodes[j], baseNodes[i]
	})

	return baseNodes[:n]
}
