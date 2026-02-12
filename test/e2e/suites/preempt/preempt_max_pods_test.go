/*
Copyright 2025 NVIDIA CORPORATION
SPDX-License-Identifier: Apache-2.0
*/
package preempt

import (
	"context"
	"fmt"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	v2 "github.com/NVIDIA/KAI-scheduler/pkg/apis/scheduling/v2"
	"github.com/NVIDIA/KAI-scheduler/pkg/common/constants"
	"github.com/NVIDIA/KAI-scheduler/test/e2e/modules/constant"
	"github.com/NVIDIA/KAI-scheduler/test/e2e/modules/constant/labels"
	testcontext "github.com/NVIDIA/KAI-scheduler/test/e2e/modules/context"
	"github.com/NVIDIA/KAI-scheduler/test/e2e/modules/resources/capacity"
	"github.com/NVIDIA/KAI-scheduler/test/e2e/modules/resources/fillers"
	"github.com/NVIDIA/KAI-scheduler/test/e2e/modules/resources/rd"
	"github.com/NVIDIA/KAI-scheduler/test/e2e/modules/resources/rd/queue"
	"github.com/NVIDIA/KAI-scheduler/test/e2e/modules/utils"
	"github.com/NVIDIA/KAI-scheduler/test/e2e/modules/wait"
)

var _ = Describe("Preemption with Max Pods Limit", Ordered, func() {
	var (
		testCtx                         *testcontext.TestContext
		lowPreemptiblePriorityClass     string
		highPreemptiblePriorityClass    string
		lowNonPreemptiblePriorityClass  string
		highNonPreemptiblePriorityClass string
		targetNode                      string
	)

	BeforeAll(func(ctx context.Context) {
		testCtx = testcontext.GetConnectivity(ctx, Default)
		capacity.SkipIfInsufficientClusterResources(testCtx.KubeClientset, &capacity.ResourceList{
			Gpu:      resource.MustParse("1"),
			Cpu:      resource.MustParse("100m"),
			PodCount: 1,
		})

		// Get a node with GPU for testing
		nodes, err := testCtx.KubeClientset.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
		Expect(err).To(Succeed())
		for _, node := range nodes.Items {
			// Find a node with GPU
			if gpuCount, ok := node.Status.Allocatable[constants.GpuResource]; ok && gpuCount.Value() > 0 {
				targetNode = node.Name
				break
			}
		}
		if targetNode == "" {
			Skip("No node with GPU found")
		}

		parentQueue := queue.CreateQueueObject(utils.GenerateRandomK8sName(10), "")
		testQueue := queue.CreateQueueObject(utils.GenerateRandomK8sName(10), parentQueue.Name)
		testCtx.InitQueues([]*v2.Queue{testQueue, parentQueue})

		lowPreemptiblePriorityClass = utils.GenerateRandomK8sName(10)
		lowPreemptiblePriorityValue := utils.RandomIntBetween(0, constant.NonPreemptiblePriorityThreshold-2)
		_, err = testCtx.KubeClientset.SchedulingV1().PriorityClasses().
			Create(ctx, rd.CreatePriorityClass(lowPreemptiblePriorityClass, lowPreemptiblePriorityValue),
				metav1.CreateOptions{})
		Expect(err).To(Succeed())

		highPreemptiblePriorityClass = utils.GenerateRandomK8sName(10)
		_, err = testCtx.KubeClientset.SchedulingV1().PriorityClasses().
			Create(ctx, rd.CreatePriorityClass(highPreemptiblePriorityClass, lowPreemptiblePriorityValue+1),
				metav1.CreateOptions{})
		Expect(err).To(Succeed())

		lowNonPreemptiblePriorityClass = utils.GenerateRandomK8sName(10)
		lowNonPreemptiblePriorityValue := utils.RandomIntBetween(constant.NonPreemptiblePriorityThreshold,
			constant.NonPreemptiblePriorityThreshold*2)
		_, err = testCtx.KubeClientset.SchedulingV1().PriorityClasses().
			Create(ctx, rd.CreatePriorityClass(lowNonPreemptiblePriorityClass, lowNonPreemptiblePriorityValue),
				metav1.CreateOptions{})
		Expect(err).To(Succeed())

		highNonPreemptiblePriorityClass = utils.GenerateRandomK8sName(10)
		_, err = testCtx.KubeClientset.SchedulingV1().PriorityClasses().
			Create(ctx, rd.CreatePriorityClass(highNonPreemptiblePriorityClass, lowNonPreemptiblePriorityValue+1),
				metav1.CreateOptions{})
		Expect(err).To(Succeed())
	})

	AfterAll(func(ctx context.Context) {
		err := rd.DeleteAllE2EPriorityClasses(ctx, testCtx.ControllerClient)
		Expect(err).To(Succeed())
		testCtx.ClusterCleanup(ctx)
	})

	AfterEach(func(ctx context.Context) {
		testCtx.TestContextCleanup(ctx)
	})

	It("Simple case: preempt on node at max pods", func(ctx context.Context) {
		// Get node's max pod capacity
		node, err := testCtx.KubeClientset.CoreV1().Nodes().Get(ctx, targetNode, metav1.GetOptions{})
		Expect(err).To(Succeed())
		maxPods := int(node.Status.Allocatable.Pods().Value())

		// Fill node to max capacity with low-priority CPU pods
		_, _, err = fillers.FillAllNodesWithJobs(ctx, testCtx, testCtx.Queues[0],
			v1.ResourceRequirements{
				Requests: map[v1.ResourceName]resource.Quantity{
					v1.ResourceCPU: resource.MustParse("10m"),
				},
			},
			nil, nil, lowPreemptiblePriorityClass, targetNode)
		Expect(err).To(Succeed())

		// Verify node is at max pods
		node, err = testCtx.KubeClientset.CoreV1().Nodes().Get(ctx, targetNode, metav1.GetOptions{})
		Expect(err).To(Succeed())
		podList, err := testCtx.KubeClientset.CoreV1().Pods("").List(ctx, metav1.ListOptions{
			FieldSelector: fmt.Sprintf("spec.nodeName=%s,status.phase!=Failed,status.phase!=Succeeded", targetNode),
		})
		Expect(err).To(Succeed())
		currentPods := len(podList.Items)
		maxPods = int(node.Status.Allocatable.Pods().Value())
		Expect(currentPods).To(Equal(maxPods), "Node should be at max pod capacity")

		// Create high-priority pod that should preempt one low-priority pod
		highPriorityPod := rd.CreatePodObject(testCtx.Queues[0], v1.ResourceRequirements{
			Requests: map[v1.ResourceName]resource.Quantity{
				v1.ResourceCPU: resource.MustParse("10m"),
			},
		})
		highPriorityPod.Spec.PriorityClassName = highPreemptiblePriorityClass
		highPriorityPod.Spec.NodeSelector = map[string]string{
			constant.NodeNamePodLabelName: targetNode,
		}

		_, err = rd.CreatePod(ctx, testCtx.KubeClientset, highPriorityPod)
		Expect(err).To(Succeed())

		// Wait for preemption and scheduling
		wait.ForPodScheduled(ctx, testCtx.ControllerClient, highPriorityPod)

		// Verify high-priority pod is scheduled on target node
		scheduledPod, err := testCtx.KubeClientset.CoreV1().Pods(highPriorityPod.Namespace).
			Get(ctx, highPriorityPod.Name, metav1.GetOptions{})
		Expect(err).To(Succeed())
		Expect(scheduledPod.Spec.NodeName).To(Equal(targetNode))
	})

	It("node at maxPods-1, fraction pod cannot allocate", Label(labels.ReservationPod), func(ctx context.Context) {
		// Get node's max pod capacity
		node, err := testCtx.KubeClientset.CoreV1().Nodes().Get(ctx, targetNode, metav1.GetOptions{})
		Expect(err).To(Succeed())
		maxPods := int(node.Status.Allocatable.Pods().Value())

		// Fill node to max capacity with low-priority CPU pods
		_, _, err = fillers.FillAllNodesWithJobs(ctx, testCtx, testCtx.Queues[0],
			v1.ResourceRequirements{
				Requests: map[v1.ResourceName]resource.Quantity{
					v1.ResourceCPU: resource.MustParse("10m"),
				},
			},
			nil, nil, lowPreemptiblePriorityClass, targetNode)
		Expect(err).To(Succeed())

		// Verify node is at max pods
		node, err = testCtx.KubeClientset.CoreV1().Nodes().Get(ctx, targetNode, metav1.GetOptions{})
		Expect(err).To(Succeed())
		podList, err := testCtx.KubeClientset.CoreV1().Pods("").List(ctx, metav1.ListOptions{
			FieldSelector: fmt.Sprintf("spec.nodeName=%s,status.phase!=Failed,status.phase!=Succeeded", targetNode),
		})
		Expect(err).To(Succeed())
		currentPods := len(podList.Items)
		maxPods = int(node.Status.Allocatable.Pods().Value())
		Expect(currentPods).To(Equal(maxPods), "Node should be at max pod capacity")

		// delete one e2e pod
		for _, pod := range podList.Items {
			if pod.Labels[constant.AppLabelName] != "engine-e2e" {
				continue
			}
			if pod.Status.Phase != v1.PodRunning {
				continue
			}
			if pod.Spec.NodeName != targetNode {
				continue
			}
			namespace := queue.GetConnectedNamespaceToQueue(testCtx.Queues[0])
			err = testCtx.KubeClientset.CoreV1().Pods(namespace).Delete(ctx, pod.Name, metav1.DeleteOptions{})
			Expect(err).To(Succeed())
			break
		}

		// Create a fractional GPU pod (will need 2 pods: task + reservation)
		// This should fail because 109 + 2 = 111 > 110
		fractionPod := rd.CreatePodObject(testCtx.Queues[0], v1.ResourceRequirements{})
		fractionPod.Annotations = map[string]string{
			constants.GpuFraction: "0.5",
		}
		fractionPod.Spec.PriorityClassName = lowPreemptiblePriorityClass
		fractionPod.Spec.NodeSelector = map[string]string{
			constant.NodeNamePodLabelName: targetNode,
		}

		_, err = rd.CreatePod(ctx, testCtx.KubeClientset, fractionPod)
		Expect(err).To(Succeed())

		// Wait and verify pod remains unschedulable
		wait.ForPodUnschedulable(ctx, testCtx.ControllerClient, fractionPod)

		wait.WaitForEventInNamespace(ctx, testCtx.ControllerClient, fractionPod.Namespace, func(event *v1.Event) bool {
			return event.Reason == "Unschedulable" && (strings.Contains(event.Message, "pod number exceeded") || strings.Contains(event.Message, "max pods"))
		})
	})

	It("Proper reservation calculation: preempt fraction with fraction that reuses GPU group", Label(labels.ReservationPod), func(ctx context.Context) {
		// Step 1: Create 3 fractional GPU pods on the same GPU (will use 4 pods: 3 task + 1 reservation)
		fractionPods := make([]*v1.Pod, 3)
		for i := 0; i < 3; i++ {
			pod := rd.CreatePodObject(testCtx.Queues[0], v1.ResourceRequirements{})
			pod.Annotations = map[string]string{
				constants.GpuFraction: "0.3", // Each takes 30% of GPU
			}
			pod.Spec.PriorityClassName = lowPreemptiblePriorityClass
			pod.Spec.NodeSelector = map[string]string{
				constant.NodeNamePodLabelName: targetNode,
			}
			createdPod, err := rd.CreatePod(ctx, testCtx.KubeClientset, pod)
			Expect(err).To(Succeed())
			fractionPods[i] = createdPod
		}

		// Wait for all fraction pods to be scheduled
		for _, pod := range fractionPods {
			wait.ForPodScheduled(ctx, testCtx.ControllerClient, pod)
		}

		// Step 2: Fill remaining capacity with CPU pods (maxPods - 4 fraction/reservation pods)
		_, _, err := fillers.FillAllNodesWithJobs(ctx, testCtx, testCtx.Queues[0],
			v1.ResourceRequirements{
				Requests: map[v1.ResourceName]resource.Quantity{
					v1.ResourceCPU: resource.MustParse("10m"),
				},
			},
			nil, nil, lowPreemptiblePriorityClass, targetNode)
		Expect(err).To(Succeed())

		// Verify node is at max pods
		podList, err := testCtx.KubeClientset.CoreV1().Pods("").List(ctx, metav1.ListOptions{
			FieldSelector: fmt.Sprintf("spec.nodeName=%s,status.phase!=Failed,status.phase!=Succeeded", targetNode),
		})
		Expect(err).To(Succeed())
		node, err := testCtx.KubeClientset.CoreV1().Nodes().Get(ctx, targetNode, metav1.GetOptions{})
		Expect(err).To(Succeed())
		maxPods := int(node.Status.Allocatable.Pods().Value())
		Expect(len(podList.Items)).To(Equal(maxPods), "Node should be at max pod capacity")

		// Step 3: Create high-priority fractional GPU pod that can reuse the same GPU group
		// This should succeed by preempting one of the low-priority fraction pods
		// Result: still maxPods total (3 fraction pods + 1 reservation + CPU fillers)
		highPriorityFractionPod := rd.CreatePodObject(testCtx.Queues[0], v1.ResourceRequirements{})
		highPriorityFractionPod.Annotations = map[string]string{
			constants.GpuFraction: "0.3", // Same size, can fit on same GPU
		}
		highPriorityFractionPod.Spec.PriorityClassName = highPreemptiblePriorityClass
		highPriorityFractionPod.Spec.NodeSelector = map[string]string{
			constant.NodeNamePodLabelName: targetNode,
		}

		_, err = rd.CreatePod(ctx, testCtx.KubeClientset, highPriorityFractionPod)
		Expect(err).To(Succeed())

		// Wait for preemption and scheduling
		wait.ForPodScheduled(ctx, testCtx.ControllerClient, highPriorityFractionPod)

		// Verify high-priority fraction pod is scheduled on target node
		scheduledPod, err := testCtx.KubeClientset.CoreV1().Pods(highPriorityFractionPod.Namespace).
			Get(ctx, highPriorityFractionPod.Name, metav1.GetOptions{})
		Expect(err).To(Succeed())
		Expect(scheduledPod.Spec.NodeName).To(Equal(targetNode))

		// Verify one of the low-priority fraction pods was preempted
		remainingLowPriorityFractions := 0
		for _, pod := range fractionPods {
			pod, err := testCtx.KubeClientset.CoreV1().Pods(pod.Namespace).
				Get(ctx, pod.Name, metav1.GetOptions{})
			if err == nil && pod.Status.Phase == v1.PodRunning {
				remainingLowPriorityFractions++
			}
		}
		Expect(remainingLowPriorityFractions).To(Equal(2), "Exactly one low-priority fraction pod should be preempted")

		// Verify total pod count is still at max
		node2, err := testCtx.KubeClientset.CoreV1().Nodes().Get(ctx, targetNode, metav1.GetOptions{})
		Expect(err).To(Succeed())
		maxPods2 := int(node2.Status.Allocatable.Pods().Value())

		finalPodList, err := testCtx.KubeClientset.CoreV1().Pods("").List(ctx, metav1.ListOptions{
			FieldSelector: fmt.Sprintf("spec.nodeName=%s,status.phase!=Failed,status.phase!=Succeeded", targetNode),
		})
		Expect(err).To(Succeed())
		Expect(len(finalPodList.Items)).To(BeNumerically("<=", maxPods2), "Should not exceed max pods")
	})
})
