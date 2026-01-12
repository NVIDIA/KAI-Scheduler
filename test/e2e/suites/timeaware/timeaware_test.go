/*
Copyright 2025 NVIDIA CORPORATION
SPDX-License-Identifier: Apache-2.0
*/
package timeaware

import (
	"context"
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"

	v2 "github.com/NVIDIA/KAI-scheduler/pkg/apis/scheduling/v2"
	"github.com/NVIDIA/KAI-scheduler/pkg/common/constants"
	testcontext "github.com/NVIDIA/KAI-scheduler/test/e2e/modules/context"
	"github.com/NVIDIA/KAI-scheduler/test/e2e/modules/resources/capacity"
	"github.com/NVIDIA/KAI-scheduler/test/e2e/modules/resources/rd"
	"github.com/NVIDIA/KAI-scheduler/test/e2e/modules/resources/rd/queue"
	"github.com/NVIDIA/KAI-scheduler/test/e2e/modules/utils"
	"github.com/NVIDIA/KAI-scheduler/test/e2e/modules/wait"
)

const (
	// Time to let a queue accumulate usage history
	usageAccumulationTime = 20 * time.Second
	// Time to wait for fairness to kick in
	fairnessTimeout = 30 * time.Second
)

func createQueuesWithEqualWeight(parentDeserved, childDeserved float64) (*v2.Queue, *v2.Queue, *v2.Queue) {
	parentQueueName := utils.GenerateRandomK8sName(10)
	parentQueue := queue.CreateQueueObjectWithGpuResource(parentQueueName,
		v2.QueueResource{
			Quota:           parentDeserved,
			OverQuotaWeight: 1,
			Limit:           -1,
		}, "")

	queueAName := utils.GenerateRandomK8sName(10)
	queueA := queue.CreateQueueObjectWithGpuResource(queueAName,
		v2.QueueResource{
			Quota:           childDeserved,
			OverQuotaWeight: 1, // Equal weight
			Limit:           -1,
		}, parentQueueName)

	queueBName := utils.GenerateRandomK8sName(10)
	queueB := queue.CreateQueueObjectWithGpuResource(queueBName,
		v2.QueueResource{
			Quota:           childDeserved,
			OverQuotaWeight: 1, // Equal weight
			Limit:           -1,
		}, parentQueueName)

	return parentQueue, queueA, queueB
}

func createGPUPod(ctx context.Context, queueObj *v2.Queue, gpus int) *v1.Pod {
	pod := rd.CreatePodObject(queueObj, v1.ResourceRequirements{
		Limits: map[v1.ResourceName]resource.Quantity{
			constants.GpuResource: resource.MustParse(fmt.Sprintf("%d", gpus)),
		},
	})
	pod, err := rd.CreatePod(ctx, testCtx.KubeClientset, pod)
	Expect(err).To(Succeed(), "Failed to create pod")
	return pod
}

// Time Aware Fairness tests verify that:
//  1. When prometheus.enabled=true in KAI Config, the operator creates a Prometheus instance
//  2. When a SchedulingShard has usageDBConfig.clientType=prometheus without a connectionString,
//     the operator auto-resolves the URL to the managed prometheus-operated service
//  3. The scheduler correctly uses historical usage data for fair scheduling decisions
var _ = Describe("Time Aware Fairness", Label("timeaware", "nightly"), Ordered, func() {
	BeforeAll(func(ctx context.Context) {
		// Ensure we have at least 1 GPU for the test
		capacity.SkipIfInsufficientClusterTopologyResources(testCtx.KubeClientset, []capacity.ResourceList{
			{
				Gpu:      resource.MustParse("1"),
				PodCount: 2,
			},
		})
	})

	AfterEach(func(ctx context.Context) {
		testCtx.ClusterCleanup(ctx)
	})

	It("should schedule fairly based on historical usage", func(ctx context.Context) {
		By("Creating a department and two queues with equal weight")
		// Get a fresh test context with the current context to avoid stale context from BeforeSuite
		testCtx = testcontext.GetConnectivity(ctx, Default)
		// Both queues have equal quota (0) and equal weight (1)
		// This means they should share resources 50/50 based on fairness
		parentQueue, queueA, queueB := createQueuesWithEqualWeight(1, 0)
		testCtx.InitQueues([]*v2.Queue{parentQueue, queueA, queueB})

		By("Submitting a job to queue-a that monopolizes the GPU")
		podA := createGPUPod(ctx, queueA, 1)
		wait.ForPodScheduled(ctx, testCtx.ControllerClient, podA)

		By(fmt.Sprintf("Letting queue-a accumulate usage history for %v", usageAccumulationTime))
		// This sleep allows Prometheus to scrape allocation metrics multiple times
		// and build up historical usage data for queue-a
		time.Sleep(usageAccumulationTime)

		By("Submitting a competing job from queue-b")
		podB := createGPUPod(ctx, queueB, 1)

		By("Verifying queue-b job gets scheduled due to time-aware fairness")
		// With time-aware fairness enabled:
		// - queue-a has accumulated historical usage (monopolized the GPU)
		// - queue-b has zero historical usage
		// - queue-b should have higher fairshare and its job should be scheduled
		// - This means queue-a's pod should be preempted/reclaimed
		Eventually(func(g Gomega) {
			// Refresh pod status
			updatedPodB, err := rd.GetPod(ctx, testCtx.KubeClientset, podB.Namespace, podB.Name)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(rd.IsPodScheduled(updatedPodB)).To(BeTrue(),
				"queue-b pod should be scheduled due to time-aware fairness - "+
					"queue-a's historical usage should result in lower fairshare")
		}, fairnessTimeout, 1*time.Second).Should(Succeed())
	})
})
