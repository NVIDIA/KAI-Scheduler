/*
Copyright 2025 NVIDIA CORPORATION
SPDX-License-Identifier: Apache-2.0
*/
package jobset

import (
	"context"
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	jobsetv1alpha2 "sigs.k8s.io/jobset/api/jobset/v1alpha2"

	v2 "github.com/NVIDIA/KAI-scheduler/pkg/apis/scheduling/v2"
	"github.com/NVIDIA/KAI-scheduler/pkg/apis/scheduling/v2alpha2"
	"github.com/NVIDIA/KAI-scheduler/pkg/common/constants"
	"github.com/NVIDIA/KAI-scheduler/test/e2e/modules/configurations"
	"github.com/NVIDIA/KAI-scheduler/test/e2e/modules/constant"
	testcontext "github.com/NVIDIA/KAI-scheduler/test/e2e/modules/context"
	"github.com/NVIDIA/KAI-scheduler/test/e2e/modules/resources/capacity"
	"github.com/NVIDIA/KAI-scheduler/test/e2e/modules/resources/rd/crd"
	"github.com/NVIDIA/KAI-scheduler/test/e2e/modules/resources/rd/jobset"
	"github.com/NVIDIA/KAI-scheduler/test/e2e/modules/resources/rd/queue"
	"github.com/NVIDIA/KAI-scheduler/test/e2e/modules/utils"
	"github.com/NVIDIA/KAI-scheduler/test/e2e/modules/wait"
	kaiv1 "github.com/NVIDIA/KAI-scheduler/pkg/apis/kai/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	runtimeClient "sigs.k8s.io/controller-runtime/pkg/client"
)


var _ = Describe("JobSet integration", Ordered, func() {
	var (
		testCtx *testcontext.TestContext
	)

	BeforeAll(func(ctx context.Context) {
		testCtx = testcontext.GetConnectivity(ctx, Default)
		crd.SkipIfCrdIsNotInstalled(ctx, testCtx.KubeConfig, "jobsets.jobset.x-k8s.io", "v1alpha2")
		Expect(jobsetv1alpha2.AddToScheme(testCtx.ControllerClient.Scheme())).To(Succeed())
		parentQueue := queue.CreateQueueObject(utils.GenerateRandomK8sName(10), "")
		childQueue := queue.CreateQueueObject(utils.GenerateRandomK8sName(10), parentQueue.Name)
		testCtx.InitQueues([]*v2.Queue{childQueue, parentQueue})
	})

	AfterEach(func(ctx context.Context) {
		testCtx.TestContextCleanup(ctx)
	})

	AfterAll(func(ctx context.Context) {
		testCtx.ClusterCleanup(ctx)
	})

	Context("JobSet submission", func() {
		It("should schedule pods with parallelism=2 and completions=3, first pod completes 5 seconds earlier", func(ctx context.Context) {
			// Set default-staleness-grace-period to 1s for this test
			originalShard := &kaiv1.SchedulingShard{}
			err := testCtx.ControllerClient.Get(ctx, client.ObjectKey{Name: "default"}, originalShard)
			Expect(err).To(Succeed())
			originalValue := originalShard.Spec.Args["default-staleness-grace-period"]
			
			// Set to 1s
			err = configurations.SetShardArg(ctx, testCtx, "default", "default-staleness-grace-period", ptr.To("1s"))
			Expect(err).To(Succeed())
			wait.WaitForDeploymentPodsRunning(ctx, testCtx.ControllerClient, constant.SchedulerDeploymentName, constants.DefaultKAINamespace)
			
			// Restore original value in defer
			defer func() {
				var restoreValue *string
				if originalValue != "" {
					restoreValue = ptr.To(originalValue)
				}
				err := configurations.SetShardArg(ctx, testCtx, "default", "default-staleness-grace-period", restoreValue)
				Expect(err).To(Succeed())
				wait.WaitForDeploymentPodsRunning(ctx, testCtx.ControllerClient, constant.SchedulerDeploymentName, constants.DefaultKAINamespace)
			}()

			testNamespace := queue.GetConnectedNamespaceToQueue(testCtx.Queues[0])
			jobSetName := "test-jobset-" + utils.GenerateRandomK8sName(10)

			jobSet := jobset.CreateObject(jobSetName, testNamespace, testCtx.Queues[0].Name, 2, 3)
			Expect(testCtx.ControllerClient.Create(ctx, jobSet)).To(Succeed())
			defer func() {
				Expect(testCtx.ControllerClient.Delete(ctx, jobSet)).To(Succeed())
			}()

			// Wait for pods to be created
			pods := waitForJobSetPods(ctx, testCtx, jobSetName, testNamespace, 3)

			// Wait for all pods to be scheduled
			wait.ForPodsScheduled(ctx, testCtx.ControllerClient, testNamespace, pods)

			// Wait for first pod to complete (completes 5s earlier)
			firstPod := pods[0]
			wait.ForPodSucceededOrError(ctx, testCtx.ControllerClient, firstPod)

			// Wait for remaining pods to complete
			for i := 1; i < len(pods); i++ {
				wait.ForPodSucceededOrError(ctx, testCtx.ControllerClient, pods[i])
			}
		})

		It("should create separate PodGroups per replicatedJob when startupPolicyOrder is InOrder", func(ctx context.Context) {
			testNamespace := queue.GetConnectedNamespaceToQueue(testCtx.Queues[0])
			jobSetName := "inorder-jobset-" + utils.GenerateRandomK8sName(10)

			jobSet := jobset.CreateObjectWithStartupPolicy(jobSetName, testNamespace, testCtx.Queues[0].Name, "InOrder")
			Expect(testCtx.ControllerClient.Create(ctx, jobSet)).To(Succeed())
			defer func() {
				Expect(testCtx.ControllerClient.Delete(ctx, jobSet)).To(Succeed())
			}()

			// Wait for pods and verify PodGroups are created separately
			pods := waitForJobSetPods(ctx, testCtx, jobSetName, testNamespace, 2)
			wait.ForPodsScheduled(ctx, testCtx.ControllerClient, testNamespace, pods)

			// Verify 2 separate PodGroups are created (one per replicatedJob)
			Eventually(func(g Gomega) {
				podGroups := &v2alpha2.PodGroupList{}
				err := testCtx.ControllerClient.List(ctx, podGroups, runtimeClient.InNamespace(testNamespace))
				g.Expect(err).To(Succeed())

				// Should have 2 PodGroups (one per replicatedJob with InOrder)
				g.Expect(len(podGroups.Items)).To(Equal(2), "Expected 2 separate PodGroups for InOrder startup policy")

				// Verify each PodGroup corresponds to a replicatedJob by checking name pattern
				job1PG := findPodGroupByName(podGroups.Items, "job1")
				job2PG := findPodGroupByName(podGroups.Items, "job2")
				g.Expect(job1PG).NotTo(BeNil(), "PodGroup for job1 should exist")
				g.Expect(job2PG).NotTo(BeNil(), "PodGroup for job2 should exist")
			}, time.Minute).Should(Succeed())
		})

		It("should create a single PodGroup for all replicatedJobs when startupPolicyOrder is not InOrder", func(ctx context.Context) {
			testNamespace := queue.GetConnectedNamespaceToQueue(testCtx.Queues[0])
			jobSetName := "anyorder-jobset-" + utils.GenerateRandomK8sName(10)

			jobSet := jobset.CreateObjectWithStartupPolicy(jobSetName, testNamespace, testCtx.Queues[0].Name, "AnyOrder")
			Expect(testCtx.ControllerClient.Create(ctx, jobSet)).To(Succeed())
			defer func() {
				Expect(testCtx.ControllerClient.Delete(ctx, jobSet)).To(Succeed())
			}()

			// Wait for pods and verify they all use the same PodGroup
			pods := waitForJobSetPods(ctx, testCtx, jobSetName, testNamespace, 2)
			wait.ForPodsScheduled(ctx, testCtx.ControllerClient, testNamespace, pods)

			// Verify only 1 PodGroup is created for all replicatedJobs with AnyOrder
			Eventually(func(g Gomega) {
				podGroups := &v2alpha2.PodGroupList{}
				err := testCtx.ControllerClient.List(ctx, podGroups, runtimeClient.InNamespace(testNamespace))
				g.Expect(err).To(Succeed())

				// Should have 1 PodGroup (shared by all replicatedJobs with AnyOrder)
				g.Expect(len(podGroups.Items)).To(Equal(1), "Expected 1 PodGroup for AnyOrder startup policy")

				// Verify PodGroup name doesn't contain replicatedJob name
				// PodGroup name format: pg-<jobset-name>-<jobset-uid> (no replicatedJob suffix)
				if len(podGroups.Items) > 0 {
					pgName := podGroups.Items[0].Name
					g.Expect(pgName).NotTo(ContainSubstring("-job1"), "PodGroup name should not contain replicatedJob name")
					g.Expect(pgName).NotTo(ContainSubstring("-job2"), "PodGroup name should not contain replicatedJob name")
				}
			}, time.Minute).Should(Succeed())
		})
	})

	Context("JobSet PodGroup creation scenarios", func() {
		It("should create PodGroup with MinMember=8 for single ReplicatedJob with high parallelism", func(ctx context.Context) {
			// Check if cluster has enough GPU resources (8 GPUs needed)
			capacity.SkipIfInsufficientClusterResources(testCtx.KubeClientset,
				&capacity.ResourceList{
					Gpu:      resource.MustParse("8"),
					PodCount: 8,
				},
			)

			testNamespace := queue.GetConnectedNamespaceToQueue(testCtx.Queues[0])
			jobSetName := "high-parallelism-" + utils.GenerateRandomK8sName(10)

			jobSet := jobset.CreateObjectWithHighParallelism(jobSetName, testNamespace, testCtx.Queues[0].Name)
			Expect(testCtx.ControllerClient.Create(ctx, jobSet)).To(Succeed())
			defer func() {
				Expect(testCtx.ControllerClient.Delete(ctx, jobSet)).To(Succeed())
			}()

			// Wait for pods to be created
			pods := waitForJobSetPods(ctx, testCtx, jobSetName, testNamespace, 8)
			wait.ForPodsScheduled(ctx, testCtx.ControllerClient, testNamespace, pods)

			// Verify PodGroup is created with correct MinMember
			Eventually(func(g Gomega) {
				podGroups := &v2alpha2.PodGroupList{}
				err := testCtx.ControllerClient.List(ctx, podGroups, runtimeClient.InNamespace(testNamespace))
				g.Expect(err).To(Succeed())

				// Should have 1 PodGroup (one per replicatedJob with InOrder)
				g.Expect(len(podGroups.Items)).To(Equal(1))
				// MinMember: 1 replica * 8 parallelism = 8
				g.Expect(podGroups.Items[0].Spec.MinMember).To(Equal(int32(8)))
			}, time.Minute).Should(Succeed())
		})

		It("should create separate PodGroups for multiple ReplicatedJobs with different parallelism", func(ctx context.Context) {
			// Check if cluster has enough GPU resources (2 GPUs needed for worker jobs)
			// Coordinator: 2 pods (0 GPU), Worker: 2 pods (2 GPU), Total: 4 pods, 2 GPU
			capacity.SkipIfInsufficientClusterResources(testCtx.KubeClientset,
				&capacity.ResourceList{
					Gpu:      resource.MustParse("2"),
					PodCount: 4, // Coordinator (2) + Worker (2) = 4 pods
				},
			)

			testNamespace := queue.GetConnectedNamespaceToQueue(testCtx.Queues[0])
			jobSetName := "multi-parallelism-" + utils.GenerateRandomK8sName(10)

			jobSet := jobset.CreateObjectWithMultipleReplicatedJobs(jobSetName, testNamespace, testCtx.Queues[0].Name)
			Expect(testCtx.ControllerClient.Create(ctx, jobSet)).To(Succeed())
			defer func() {
				Expect(testCtx.ControllerClient.Delete(ctx, jobSet)).To(Succeed())
			}()

			// Wait for pods to be created (coordinator: 2 pods, worker: 2 pods)
			// With AnyOrder, all replicatedJobs are created simultaneously
			pods := waitForJobSetPods(ctx, testCtx, jobSetName, testNamespace, 4) // Total: 2 + 2 = 4 pods
			wait.ForPodsScheduled(ctx, testCtx.ControllerClient, testNamespace, pods)

			// Verify 1 PodGroup is created for all replicatedJobs with AnyOrder
			Eventually(func(g Gomega) {
				podGroups := &v2alpha2.PodGroupList{}
				err := testCtx.ControllerClient.List(ctx, podGroups, runtimeClient.InNamespace(testNamespace))
				g.Expect(err).To(Succeed())

				// Should have 1 PodGroup for all replicatedJobs with AnyOrder
				g.Expect(len(podGroups.Items)).To(Equal(1))

				// Verify MinMember is sum of all replicatedJobs
				// Coordinator: 1 replica * 2 parallelism = 2
				// Worker: 1 replica * 2 parallelism = 2
				// Total MinMember: 4
				g.Expect(podGroups.Items[0].Spec.MinMember).To(Equal(int32(4)))
			}, time.Minute).Should(Succeed())
		})

		It("should create PodGroup with MinMember=1 for single replica with default parallelism", func(ctx context.Context) {
			testNamespace := queue.GetConnectedNamespaceToQueue(testCtx.Queues[0])
			jobSetName := "default-parallelism-" + utils.GenerateRandomK8sName(10)

			jobSet := jobset.CreateObjectWithDefaultParallelism(jobSetName, testNamespace, testCtx.Queues[0].Name)
			Expect(testCtx.ControllerClient.Create(ctx, jobSet)).To(Succeed())
			defer func() {
				Expect(testCtx.ControllerClient.Delete(ctx, jobSet)).To(Succeed())
			}()

			// Wait for pods to be created
			pods := waitForJobSetPods(ctx, testCtx, jobSetName, testNamespace, 1)
			wait.ForPodsScheduled(ctx, testCtx.ControllerClient, testNamespace, pods)

			// Verify PodGroup is created with correct MinMember (defaults to 1)
			Eventually(func(g Gomega) {
				podGroups := &v2alpha2.PodGroupList{}
				err := testCtx.ControllerClient.List(ctx, podGroups, runtimeClient.InNamespace(testNamespace))
				g.Expect(err).To(Succeed())

				// Should have 1 PodGroup
				g.Expect(len(podGroups.Items)).To(Equal(1))
				// MinMember defaults to 1 (1 replica, parallelism defaults to 1)
				g.Expect(podGroups.Items[0].Spec.MinMember).To(Equal(int32(1)))
			}, time.Minute).Should(Succeed())
		})
	})
})



func waitForJobSetPods(ctx context.Context, testCtx *testcontext.TestContext, jobSetName, namespace string, expectedCount int) []*v1.Pod {
	var pods []*v1.Pod
	Eventually(func() error {
		podList := &v1.PodList{}
		err := testCtx.ControllerClient.List(ctx, podList, client.InNamespace(namespace),
			client.MatchingLabels{
				"jobset.sigs.k8s.io/jobset-name": jobSetName,
			})
		if err != nil {
			return err
		}
		// Count all pods (including completed ones) for the JobSet.
		// With parallelism and completions, pods may be created sequentially,
		// so we wait for all expected pods to be created, not just running.
		if len(podList.Items) < expectedCount {
			return fmt.Errorf("expected %d pods, got %d", expectedCount, len(podList.Items))
		}
		pods = make([]*v1.Pod, len(podList.Items))
		for i := range podList.Items {
			pods[i] = &podList.Items[i]
		}
		return nil
	}, 120*time.Second, 2*time.Second).Should(Succeed())
	return pods
}


// findPodGroupByName finds a PodGroup by replicatedJob name suffix.
func findPodGroupByName(podGroups []v2alpha2.PodGroup, replicatedJobName string) *v2alpha2.PodGroup {
	for i := range podGroups {
		if podGroups[i].Name == "" {
			continue
		}
		// Extract the last part after the last dash (replicatedJob name)
		lastDashIndex := -1
		for k := len(podGroups[i].Name) - 1; k >= 0; k-- {
			if podGroups[i].Name[k] == '-' {
				lastDashIndex = k
				break
			}
		}
		if lastDashIndex >= 0 && lastDashIndex < len(podGroups[i].Name)-1 {
			lastPart := podGroups[i].Name[lastDashIndex+1:]
			if lastPart == replicatedJobName {
				return &podGroups[i]
			}
		}
	}
	return nil
}

