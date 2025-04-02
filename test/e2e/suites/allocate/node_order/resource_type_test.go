/*
Copyright 2025 NVIDIA CORPORATION
SPDX-License-Identifier: Apache-2.0
*/
package node_order

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
	runtimeClient "sigs.k8s.io/controller-runtime/pkg/client"

	v2 "github.com/NVIDIA/KAI-scheduler/pkg/apis/scheduling/v2"
	"github.com/NVIDIA/KAI-scheduler/pkg/common/constants"
	testcontext "github.com/NVIDIA/KAI-scheduler/test/e2e/modules/context"
	"github.com/NVIDIA/KAI-scheduler/test/e2e/modules/resources/rd"
	"github.com/NVIDIA/KAI-scheduler/test/e2e/modules/resources/rd/queue"
	"github.com/NVIDIA/KAI-scheduler/test/e2e/modules/utils"
	"github.com/NVIDIA/KAI-scheduler/test/e2e/modules/wait"
)

var _ = Describe("Resource Type Plugins", Ordered, func() {
	var (
		testCtx *testcontext.TestContext
	)

	BeforeAll(func(ctx context.Context) {
		testCtx = testcontext.GetConnectivity(ctx, Default)
		parentQueue := queue.CreateQueueObject(utils.GenerateRandomK8sName(10), "")
		childQueue := queue.CreateQueueObject(utils.GenerateRandomK8sName(10), parentQueue.Name)
		testCtx.InitQueues([]*v2.Queue{childQueue, parentQueue})
	})

	AfterAll(func(ctx context.Context) {
		testCtx.ClusterCleanup(ctx)
	})

	AfterEach(func(ctx context.Context) {
		testCtx.TestContextCleanup(ctx)
	})

	Context("CPU only jobs", func() {
		BeforeEach(func(ctx context.Context) {
			cpuNode := rd.FindNodeWithNoGPU(ctx, testCtx.ControllerClient)
			if cpuNode == nil {
				Skip("Failed to find node without GPUs")
			}
		})

		It("schedules CPU only jobs on CPU nodes", func(ctx context.Context) {
			pod := rd.CreatePodObject(testCtx.Queues[0], v1.ResourceRequirements{})
			pod, err := rd.CreatePod(ctx, testCtx.KubeClientset, pod)
			Expect(err).To(Succeed())
			wait.ForPodScheduled(ctx, testCtx.ControllerClient, pod)
			Expect(testCtx.ControllerClient.Get(
				ctx, runtimeClient.ObjectKeyFromObject(pod), pod)).To(Succeed())
			var node v1.Node
			Expect(testCtx.ControllerClient.Get(
				ctx,
				runtimeClient.ObjectKey{Name: pod.Spec.NodeName},
				&node,
			)).To(Succeed())

			Expect(isCPUOnlyNode(&node)).To(BeTrue())
		})
	})
})

func isCPUOnlyNode(node *v1.Node) bool {
	gpus, found := node.Status.Capacity[constants.GpuResource]
	return !found || gpus.CmpInt64(0) == 0
}
