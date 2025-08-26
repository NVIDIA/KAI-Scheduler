/*
Copyright 2025 NVIDIA CORPORATION
SPDX-License-Identifier: Apache-2.0
*/

package topology

import (
	"context"
	"fmt"

	v2 "github.com/NVIDIA/KAI-scheduler/pkg/apis/scheduling/v2"
	"github.com/NVIDIA/KAI-scheduler/pkg/apis/scheduling/v2alpha2"
	"github.com/NVIDIA/KAI-scheduler/pkg/common/constants"
	"github.com/NVIDIA/KAI-scheduler/test/e2e/modules/configurations/feature_flags"
	testcontext "github.com/NVIDIA/KAI-scheduler/test/e2e/modules/context"
	"github.com/NVIDIA/KAI-scheduler/test/e2e/modules/resources/rd"
	"github.com/NVIDIA/KAI-scheduler/test/e2e/modules/resources/rd/pod_group"
	"github.com/NVIDIA/KAI-scheduler/test/e2e/modules/resources/rd/queue"
	"github.com/NVIDIA/KAI-scheduler/test/e2e/modules/utils"
	"github.com/NVIDIA/KAI-scheduler/test/e2e/modules/wait"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("Topology", Ordered, func() {
	var (
		testCtx          *testcontext.TestContext
		gpuNodesNames    []string
		testTopologyData rd.TestTopologyData
	)

	BeforeAll(func(ctx context.Context) {
		testCtx = testcontext.GetConnectivity(ctx, Default)
		parentQueue := queue.CreateQueueObject(utils.GenerateRandomK8sName(10), "")
		childQueue := queue.CreateQueueObject(utils.GenerateRandomK8sName(10), parentQueue.Name)
		testCtx.InitQueues([]*v2.Queue{childQueue, parentQueue})

		testTopologyData, gpuNodesNames = rd.CreateRackZoneTopology(ctx, testCtx.KubeClientset, testCtx.KubeConfig)

		// Set spreading strategy to try and increase the probability of
		//  out-of-topology allocation more common in case of a bug.
		if err := feature_flags.SetPlacementStrategy(ctx, testCtx, feature_flags.SpreadStrategy); err != nil {
			Fail(fmt.Sprintf("Failed to patch scheduler config with spreading plugin: %v", err))
		}
	})

	AfterAll(func(ctx context.Context) {
		if err := feature_flags.SetPlacementStrategy(ctx, testCtx, feature_flags.DefaultStrategy); err != nil {
			Fail(fmt.Sprintf("Failed to patch scheduler config with spreading plugin: %v", err))
		}

		rd.CleanRackZoneTopology(ctx, testTopologyData, testCtx.KubeConfig)

		testCtx.ClusterCleanup(ctx)
	})

	Context("Topology", func() {
		BeforeEach(func(ctx context.Context) {
			rd.AssignNodesToTestTopology(ctx, testCtx.ControllerClient, gpuNodesNames, testTopologyData)
		})

		AfterEach(func(ctx context.Context) {
			rd.CleanNodesFromTopology(ctx, testCtx.ControllerClient, testTopologyData)
			testCtx.TestContextCleanup(ctx)
		})

		It("required only - rack level", func(ctx context.Context) {
			namespace := queue.GetConnectedNamespaceToQueue(testCtx.Queues[0])
			topologyConstraint := v2alpha2.TopologyConstraint{
				RequiredTopologyLevel: rd.TestRackLabelKey,
				Topology:              "e2e-topology-tree",
			}

			gpusPerNode := testTopologyData.TopologyNodes[gpuNodesNames[0]].
				Status.Allocatable[v1.ResourceName(constants.GpuResource)]
			podResource := v1.ResourceList{
				v1.ResourceName(constants.GpuResource): gpusPerNode,
			}

			pods := createDistributedWorkload(ctx, testCtx, 2, podResource, topologyConstraint)
			wait.ForPodsScheduled(ctx, testCtx.ControllerClient, namespace, pods)

			// Validate that all the pods have been scheduled to the same rack
			podList, err := testCtx.KubeClientset.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{})
			Expect(err).NotTo(HaveOccurred(), "Failed to list pods")

			scheduledRacks := map[string][]string{}
			for _, pod := range podList.Items {
				podRack := testTopologyData.TopologyNodes[pod.Spec.NodeName].Labels[rd.TestRackLabelKey]
				scheduledRacks[podRack] = append(scheduledRacks[podRack], pod.Name)
			}

			Expect(len(scheduledRacks)).To(Equal(1), "Expected all pods scheduled to one rack, got %v", scheduledRacks)
		})

		It("preferred only - rack level", func(ctx context.Context) {
			namespace := queue.GetConnectedNamespaceToQueue(testCtx.Queues[0])
			topologyConstraint := v2alpha2.TopologyConstraint{
				PreferredTopologyLevel: rd.TestRackLabelKey,
				Topology:               "e2e-topology-tree",
			}

			gpusPerNode := testTopologyData.TopologyNodes[gpuNodesNames[0]].
				Status.Allocatable[v1.ResourceName(constants.GpuResource)]
			podResource := v1.ResourceList{
				v1.ResourceName(constants.GpuResource): gpusPerNode,
			}

			pods := createDistributedWorkload(ctx, testCtx, 2, podResource, topologyConstraint)
			wait.ForPodsScheduled(ctx, testCtx.ControllerClient, namespace, pods)

			// Validate that all the pods have been scheduled to the same rack
			podList, err := testCtx.KubeClientset.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{})
			Expect(err).NotTo(HaveOccurred(), "Failed to list pods")

			scheduledRacks := map[string][]string{}
			for _, pod := range podList.Items {
				podRack := testTopologyData.TopologyNodes[pod.Spec.NodeName].Labels[rd.TestRackLabelKey]
				scheduledRacks[podRack] = append(scheduledRacks[podRack], pod.Name)
			}

			Expect(len(scheduledRacks)).To(Equal(1), "Expected all pods scheduled to one rack, got %v", scheduledRacks)
		})

		It("required rack and preferred node - all pods in a single node", func(ctx context.Context) {
			namespace := queue.GetConnectedNamespaceToQueue(testCtx.Queues[0])
			topologyConstraint := v2alpha2.TopologyConstraint{
				RequiredTopologyLevel:  rd.TestRackLabelKey,
				PreferredTopologyLevel: rd.NodeNameLabelKey,
				Topology:               "e2e-topology-tree",
			}

			gpusPerNode := testTopologyData.TopologyNodes[gpuNodesNames[0]].
				Status.Allocatable[v1.ResourceName(constants.GpuResource)]
			halfGpusPerNode := int64(gpusPerNode.AsFloat64Slow() / 2)
			podResource := v1.ResourceList{
				v1.ResourceName(constants.GpuResource): *resource.NewQuantity(halfGpusPerNode, resource.DecimalSI),
			}

			pods := createDistributedWorkload(ctx, testCtx, 2, podResource, topologyConstraint)
			wait.ForPodsScheduled(ctx, testCtx.ControllerClient, namespace, pods)

			// Validate that all the pods have been scheduled to the same rack
			podList, err := testCtx.KubeClientset.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{})
			Expect(err).NotTo(HaveOccurred(), "Failed to list pods")

			scheduledNodes := map[string][]string{}
			for _, pod := range podList.Items {
				scheduledNodes[pod.Spec.NodeName] = append(scheduledNodes[pod.Spec.NodeName], pod.Name)
			}

			Expect(len(scheduledNodes)).To(Equal(1), "Expected all pods scheduled to one node, got %v", scheduledNodes)
		})

		It("required rack and preferred node - all pods in a rack", func(ctx context.Context) {
			namespace := queue.GetConnectedNamespaceToQueue(testCtx.Queues[0])
			topologyConstraint := v2alpha2.TopologyConstraint{
				RequiredTopologyLevel:  rd.TestRackLabelKey,
				PreferredTopologyLevel: rd.NodeNameLabelKey,
				Topology:               "e2e-topology-tree",
			}

			gpusPerNode := testTopologyData.TopologyNodes[gpuNodesNames[0]].
				Status.Allocatable[v1.ResourceName(constants.GpuResource)]
			podResource := v1.ResourceList{
				v1.ResourceName(constants.GpuResource): gpusPerNode,
			}

			pods := createDistributedWorkload(ctx, testCtx, 2, podResource, topologyConstraint)
			wait.ForPodsScheduled(ctx, testCtx.ControllerClient, namespace, pods)

			// Validate that all the pods have been scheduled to the same rack
			podList, err := testCtx.KubeClientset.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{})
			Expect(err).NotTo(HaveOccurred(), "Failed to list pods")

			scheduledNodes := map[string][]string{}
			scheduledRacks := map[string][]string{}
			for _, pod := range podList.Items {
				scheduledNodes[pod.Spec.NodeName] = append(scheduledNodes[pod.Spec.NodeName], pod.Name)
				podRack := testTopologyData.TopologyNodes[pod.Spec.NodeName].Labels[rd.TestRackLabelKey]
				scheduledRacks[podRack] = append(scheduledRacks[podRack], pod.Name)
			}

			Expect(len(scheduledNodes)).To(BeNumerically(">", 1), "Expected all pods scheduled to one more then one node, got %v", scheduledNodes)
			Expect(len(scheduledRacks)).To(Equal(1), "Expected all pods scheduled to the same rack, got %v", scheduledRacks)
		})
	}, MustPassRepeatedly(3))

	Context("Empty context to jump over ginkgo bug", func() {
		It("should not create test suite while ensuring that the test suite is executed", func(ctx context.Context) {
			Expect(true).To(BeTrue())
		})
	})
}, Ordered)

func createDistributedWorkload(ctx context.Context, testCtx *testcontext.TestContext,
	podCount int, podResource v1.ResourceList, topologyConstraint v2alpha2.TopologyConstraint) []*v1.Pod {
	namespace := queue.GetConnectedNamespaceToQueue(testCtx.Queues[0])
	queueName := testCtx.Queues[0].Name

	podGroup := pod_group.Create(namespace, "distributed-pod-group"+utils.GenerateRandomK8sName(10), queueName)
	podGroup.Spec.MinMember = int32(podCount)
	podGroup.Spec.TopologyConstraint = topologyConstraint

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

	return pods
}
