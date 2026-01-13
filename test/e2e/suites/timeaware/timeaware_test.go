/*
Copyright 2025 NVIDIA CORPORATION
SPDX-License-Identifier: Apache-2.0
*/
package timeaware

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	v2 "github.com/NVIDIA/KAI-scheduler/pkg/apis/scheduling/v2"
	"github.com/NVIDIA/KAI-scheduler/pkg/common/constants"
	testcontext "github.com/NVIDIA/KAI-scheduler/test/e2e/modules/context"
	"github.com/NVIDIA/KAI-scheduler/test/e2e/modules/resources/capacity"
	"github.com/NVIDIA/KAI-scheduler/test/e2e/modules/resources/rd"
	"github.com/NVIDIA/KAI-scheduler/test/e2e/modules/resources/rd/pod_group"
	"github.com/NVIDIA/KAI-scheduler/test/e2e/modules/resources/rd/queue"
	"github.com/NVIDIA/KAI-scheduler/test/e2e/modules/utils"
	"github.com/NVIDIA/KAI-scheduler/test/e2e/modules/wait"

	e2econstant "github.com/NVIDIA/KAI-scheduler/test/e2e/modules/constant"
)

const (
	// Time to wait for Prometheus to have non-zero samples for queue usage
	prometheusUsageTimeout = 2 * time.Minute
	// Time to wait for fairness to kick in (includes usage fetch interval)
	fairnessTimeout = 2 * time.Minute
	// Name of the service created by the Prometheus Operator for the managed Prometheus instance
	prometheusOperatedServiceName = "prometheus-operated"
)

type promQueryResponse struct {
	Status string `json:"status"`
	Data   struct {
		ResultType string `json:"resultType"`
		Result     []struct {
			Metric map[string]string `json:"metric"`
			Value  []any             `json:"value"`
		} `json:"result"`
	} `json:"data"`
}

// createQueuesForTimeAwareFairness creates queues designed to test time-aware fairness:
//
// The key insight is that fair share is calculated using this formula:
//
//	shareWeight = max(0, nWeight + kValue * (nWeight - nUsage))
//
// Where:
//   - nWeight = normalized over-quota weight
//   - nUsage = normalized historical usage (from Prometheus)
//   - kValue = time-aware fairness multiplier (configured in SchedulingShard)
//
// Setup:
//   - Queue-a: VERY HIGH over-quota weight (10000) → gets ~99.99% base fair share
//   - Queue-b: LOW over-quota weight (1) → gets ~0.01% base fair share
//   - Queue-a monopolizes ALL resources
//
// Without time-aware fairness (kValue=0):
//   - Queue-a fair share ≈ 99.99%, queue-b ≈ 0.01%
//   - Queue-a using 100% is only ~0.01% over fair share
//   - Queue-b's 0.01% fair share is too small to justify reclaiming 1 GPU
//   - Result: NO reclaim, pod-b stays pending
//
// With time-aware fairness (kValue=100, queue-a nUsage=1.0):
//   - Queue-a's weight: max(0, 0.9999 + 100*(0.9999 - 1.0)) = max(0, 0.9999 - 0.01) = 0.9899
//   - Queue-b's weight: max(0, 0.0001 + 100*(0.0001 - 0)) = max(0, 0.0101) = 0.0101
//   - Fair share shifts: queue-a ≈ 99%, queue-b ≈ 1%
//   - With higher kValue, shift is more dramatic
//   - Result: Reclaim happens, pod-b gets scheduled
func createQueuesForTimeAwareFairness() (*v2.Queue, *v2.Queue, *v2.Queue) {
	parentQueueName := utils.GenerateRandomK8sName(10)
	parentQueue := queue.CreateQueueObjectWithGpuResource(parentQueueName,
		v2.QueueResource{
			Quota:           -1, // Unlimited quota for department
			OverQuotaWeight: 1,
			Limit:           -1,
		}, "")

	queueAName := utils.GenerateRandomK8sName(10)
	queueA := queue.CreateQueueObjectWithGpuResource(queueAName,
		v2.QueueResource{
			Quota:           0,     // No guaranteed quota - all resources are over-quota
			OverQuotaWeight: 10000, // VERY HIGH weight - gets ~99.99% base fair share
			Limit:           -1,
		}, parentQueueName)

	queueBName := utils.GenerateRandomK8sName(10)
	queueB := queue.CreateQueueObjectWithGpuResource(queueBName,
		v2.QueueResource{
			Quota:           0, // No guaranteed quota
			OverQuotaWeight: 1, // LOW weight - gets ~0.01% base fair share
			Limit:           -1,
		}, parentQueueName)

	return parentQueue, queueA, queueB
}

func queryPrometheusInstant(ctx context.Context, query string) ([]float64, error) {
	params := map[string]string{"query": query}

	raw, err := testCtx.KubeClientset.
		CoreV1().
		Services(e2econstant.SystemPodsNamespace).
		ProxyGet("http", prometheusOperatedServiceName, "9090", "/api/v1/query", params).
		DoRaw(ctx)
	if err != nil {
		return nil, err
	}

	resp := promQueryResponse{}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, err
	}
	if resp.Status != "success" {
		return nil, fmt.Errorf("prometheus query status=%s", resp.Status)
	}

	values := make([]float64, 0, len(resp.Data.Result))
	for _, r := range resp.Data.Result {
		if len(r.Value) < 2 {
			continue
		}
		valueStr, ok := r.Value[1].(string)
		if !ok {
			continue
		}
		v, err := strconv.ParseFloat(valueStr, 64)
		if err != nil {
			continue
		}
		values = append(values, v)
	}
	return values, nil
}

func maxFloat64(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	max := values[0]
	for _, v := range values[1:] {
		if v > max {
			max = v
		}
	}
	return max
}

func createGuaranteedLowerPriorityClass(ctx context.Context) (string, error) {
	name := utils.GenerateRandomK8sName(10)
	_, err := testCtx.KubeClientset.SchedulingV1().PriorityClasses().Create(
		ctx,
		rd.CreatePriorityClass(name, -1000),
		metav1.CreateOptions{},
	)
	if err != nil {
		return "", err
	}
	return name, nil
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
		Expect(rd.DeleteAllE2EPriorityClasses(ctx, testCtx.ControllerClient)).To(Succeed())
	})

	It("should schedule fairly based on historical usage", func(ctx context.Context) {
		By("Creating a department and two queues with asymmetric weights")
		// Get a fresh test context with the current context to avoid stale context from BeforeSuite
		testCtx = testcontext.GetConnectivity(ctx, Default)

		// Queue-a: very high weight (10000) → ~99.99% base fair share
		// Queue-b: low weight (1) → ~0.01% base fair share
		// Without time-aware (kValue=0): queue-b's tiny fair share may not justify reclaim
		// With time-aware (kValue=10000): queue-a's usage history shifts fair share dramatically
		parentQueue, queueA, queueB := createQueuesForTimeAwareFairness()
		testCtx.InitQueues([]*v2.Queue{parentQueue, queueA, queueB})

		By("Creating a guaranteed-lower priority class for filler jobs")
		lowPriority, err := createGuaranteedLowerPriorityClass(ctx)
		Expect(err).To(Succeed())

		By("Filling ALL cluster GPUs with queue-a pod-group pods to ensure resource contention")
		// Time-aware fairness usage data is derived from Prometheus metric kai_queue_allocated_gpus,
		// which is based on Queue.Status.Allocated. QueueController computes allocations from PodGroups,
		// so the test must create PodGroups (not standalone pods/jobs).
		idleByNode, err := capacity.GetNodesIdleResources(testCtx.KubeClientset)
		Expect(err).To(Succeed())
		idleGPUs := int64(0)
		for _, rl := range idleByNode {
			idleGPUs += rl.Gpu.Value()
		}
		Expect(idleGPUs).To(BeNumerically(">", 0), "Cluster must have at least 1 idle GPU for the test")

		resources := v1.ResourceRequirements{
			Requests: v1.ResourceList{
				constants.GpuResource: resource.MustParse("1"),
			},
			Limits: v1.ResourceList{
				constants.GpuResource: resource.MustParse("1"),
			},
		}
		_, queueAPods := pod_group.CreateWithPods(
			ctx,
			testCtx.KubeClientset,
			testCtx.KubeAiSchedClientset,
			utils.GenerateRandomK8sName(10),
			queueA,
			int(idleGPUs),
			&lowPriority,
			resources,
		)
		namespace := queue.GetConnectedNamespaceToQueue(queueA)
		wait.ForAtLeastNPodsScheduled(ctx, testCtx.ControllerClient, namespace, queueAPods, 1)

		By("Waiting for queue-controller to reflect full queue-a allocation in Queue status")
		Eventually(func(g Gomega) {
			updatedQueue, qErr := testCtx.KubeAiSchedClientset.SchedulingV2().Queues("").Get(ctx, queueA.Name, metav1.GetOptions{})
			g.Expect(qErr).NotTo(HaveOccurred())
			allocated := updatedQueue.Status.Allocated[constants.GpuResource]
			g.Expect(allocated.Value()).To(BeNumerically(">=", idleGPUs), "Expected Queue status allocated GPUs to reach full cluster usage")
		}, prometheusUsageTimeout, 5*time.Second).Should(Succeed())

		By("Waiting for Prometheus to observe full queue-a GPU usage")
		// If Prometheus doesn't have series for queue-a, time-aware fairness can't adjust weights.
		// We use the apiserver proxy to query the managed Prometheus instance without hardcoding URLs.
		Eventually(func(g Gomega) {
			values, qErr := queryPrometheusInstant(ctx, fmt.Sprintf("kai_queue_allocated_gpus{queue_name=\"%s\"}", queueA.Name))
			g.Expect(qErr).NotTo(HaveOccurred())
			g.Expect(values).NotTo(BeEmpty(), "Expected Prometheus to return at least one sample for queue-a")
			g.Expect(maxFloat64(values)).To(BeNumerically(">=", float64(idleGPUs)), "Expected queue-a to be allocating all idle GPUs")
		}, prometheusUsageTimeout, 5*time.Second).Should(Succeed())

		By("Submitting a competing job from queue-b (requires reclaim to schedule)")
		_, queueBPods := pod_group.CreateWithPods(
			ctx,
			testCtx.KubeClientset,
			testCtx.KubeAiSchedClientset,
			utils.GenerateRandomK8sName(10),
			queueB,
			1,
			nil,
			resources,
		)
		podB := queueBPods[0]

		By("Verifying queue-b job gets scheduled due to time-aware fairness reclaim")
		// With time-aware fairness enabled:
		// - queue-a has accumulated historical usage (monopolized ALL GPUs)
		// - queue-b has zero historical usage
		// - queue-b should have higher fairshare and its job should be scheduled
		// - This means one of queue-a's pods must be preempted/reclaimed
		// - If time-aware fairness is NOT working, pod-b will stay pending forever
		Eventually(func(g Gomega) {
			// Refresh pod status
			updatedPodB, err := rd.GetPod(ctx, testCtx.KubeClientset, podB.Namespace, podB.Name)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(rd.IsPodScheduled(updatedPodB)).To(BeTrue(),
				"queue-b pod should be scheduled due to time-aware fairness reclaim - "+
					"queue-a's historical usage should result in lower fairshare")
		}, fairnessTimeout, 2*time.Second).Should(Succeed())
	})
})
