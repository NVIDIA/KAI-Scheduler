/*
Copyright 2025 NVIDIA CORPORATION
SPDX-License-Identifier: Apache-2.0
*/
package reclaim

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
	resourceapi "k8s.io/api/resource/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/utils/pointer"
	"k8s.io/utils/ptr"
	runtimeClient "sigs.k8s.io/controller-runtime/pkg/client"

	v2 "github.com/NVIDIA/KAI-scheduler/pkg/apis/scheduling/v2"
	"github.com/NVIDIA/KAI-scheduler/pkg/common/constants"
	testcontext "github.com/NVIDIA/KAI-scheduler/test/e2e/modules/context"
	"github.com/NVIDIA/KAI-scheduler/test/e2e/modules/resources/capacity"
	"github.com/NVIDIA/KAI-scheduler/test/e2e/modules/resources/rd"
	"github.com/NVIDIA/KAI-scheduler/test/e2e/modules/resources/rd/queue"
	"github.com/NVIDIA/KAI-scheduler/test/e2e/modules/utils"
	"github.com/NVIDIA/KAI-scheduler/test/e2e/modules/wait"
)

// createPodWithDRAClaim creates a pod with a DRA resource claim requesting gpus
// The claim is created first and the pod is set as the owner
func createPodWithDRAClaim(ctx context.Context, testCtx *testcontext.TestContext, q *v2.Queue, gpus int) *v1.Pod {
	namespace := queue.GetConnectedNamespaceToQueue(q)
	gpuDeviceClassName := "gpu.nvidia.com"

	// Create the resource claim
	claim := rd.CreateResourceClaim(namespace, q.Name, gpuDeviceClassName, gpus)
	claim, err := testCtx.KubeClientset.ResourceV1().ResourceClaims(namespace).Create(ctx, claim, metav1.CreateOptions{})
	Expect(err).To(Succeed())

	// Wait for the ResourceClaim to be accessible via the controller client
	Eventually(func() error {
		claimObj := &resourceapi.ResourceClaim{}
		return testCtx.ControllerClient.Get(ctx, types.NamespacedName{
			Namespace: namespace,
			Name:      claim.Name,
		}, claimObj)
	}).Should(Succeed(), "ResourceClaim should be accessible via controller client")

	// Create the pod with the claim
	pod := rd.CreatePodObject(q, v1.ResourceRequirements{})
	pod.Spec.ResourceClaims = []v1.PodResourceClaim{{
		Name:              "gpu-claim",
		ResourceClaimName: ptr.To(claim.Name),
	}}

	pod, err = rd.CreatePod(ctx, testCtx.KubeClientset, pod)
	Expect(err).To(Succeed())

	// Update the claim to set the pod as owner
	claim.OwnerReferences = []metav1.OwnerReference{
		{
			APIVersion: "v1",
			Kind:       "Pod",
			Name:       pod.Name,
			UID:        pod.UID,
		},
	}
	_, err = testCtx.KubeClientset.ResourceV1().ResourceClaims(namespace).Update(ctx, claim, metav1.UpdateOptions{})
	Expect(err).To(Succeed())

	return pod
}

var _ = Describe("Reclaim DRA", Ordered, func() {
	Context("Quota/Fair-share based reclaim with DRA", func() {
		var (
			testCtx            *testcontext.TestContext
			gpuDeviceClassName = "gpu.nvidia.com"
		)

		BeforeAll(func(ctx context.Context) {
			testCtx = testcontext.GetConnectivity(ctx, Default)
			capacity.SkipIfInsufficientDynamicResources(testCtx.KubeClientset, gpuDeviceClassName, 1, 4)
		})

		AfterEach(func(ctx context.Context) {
			testCtx.ClusterCleanup(ctx)
		})

		It("Under quota and Over fair share -> Over quota and Over quota - Should reclaim", func(ctx context.Context) {
			// 4 GPUs in total (quota of 1)
			// reclaimee: 2+2 => 4 (OFS)           => 2 (OQ)
			// reclaimer: 0   => requesting 2 (UQ) => 2 (OQ)

			testCtx = testcontext.GetConnectivity(ctx, Default)
			parentQueue, reclaimeeQueue, reclaimerQueue := createQueues(4, 1, 1)
			testCtx.InitQueues([]*v2.Queue{parentQueue, reclaimeeQueue, reclaimerQueue})

			reclaimee1Namespace := queue.GetConnectedNamespaceToQueue(reclaimeeQueue)
			claimTemplate := rd.CreateResourceClaimTemplate(reclaimee1Namespace, reclaimeeQueue.Name, gpuDeviceClassName, 1)
			claimTemplate, err := testCtx.KubeClientset.ResourceV1().ResourceClaimTemplates(reclaimee1Namespace).Create(ctx, claimTemplate, metav1.CreateOptions{})
			Expect(err).To(Succeed())

			// Try with template claim
			pod := createPodWithDRAClaim(ctx, testCtx, reclaimeeQueue, 2)
			pod2 := createPodWithDRAClaim(ctx, testCtx, reclaimeeQueue, 2)

			wait.ForPodScheduled(ctx, testCtx.ControllerClient, pod)
			wait.ForPodScheduled(ctx, testCtx.ControllerClient, pod2)

			pendingPod := createPodWithDRAClaim(ctx, testCtx, reclaimerQueue, 2)
			wait.ForPodScheduled(ctx, testCtx.ControllerClient, pendingPod)
		})

		It("Under quota and Over fair share -> Under quota and Under quota - Should reclaim", func(ctx context.Context) {
			// 4 GPUs in total (quota of 2)
			// reclaimee: 1+2 => 3 (OFS)                 => 1 (UQ)
			// reclaimer: 1   => 1 + requesting 1 (UQ) => 2 (UQ)

			testCtx = testcontext.GetConnectivity(ctx, Default)
			parentQueue, reclaimeeQueue, reclaimerQueue := createQueues(4, 2, 2)
			testCtx.InitQueues([]*v2.Queue{parentQueue, reclaimeeQueue, reclaimerQueue})

			pod := createPodWithDRAClaim(ctx, testCtx, reclaimeeQueue, 1)
			pod2 := createPodWithDRAClaim(ctx, testCtx, reclaimeeQueue, 2)

			wait.ForPodScheduled(ctx, testCtx.ControllerClient, pod)
			wait.ForPodScheduled(ctx, testCtx.ControllerClient, pod2)

			runningPod := createPodWithDRAClaim(ctx, testCtx, reclaimerQueue, 1)
			wait.ForPodScheduled(ctx, testCtx.ControllerClient, runningPod)

			pendingPod := createPodWithDRAClaim(ctx, testCtx, reclaimerQueue, 1)
			wait.ForPodScheduled(ctx, testCtx.ControllerClient, pendingPod)
		})

		It("Under quota and Over fair share -> Over fair share and Over quota - Should not reclaim", func(ctx context.Context) {
			// 4 GPUs in total (quota of 1)
			// reclaimee: 2+1+1 => 4 (OFS)               => 2 (OQ)
			// reclaimer: 1     => 1 + requesting 2 (UQ) => 3 (OFS)

			testCtx = testcontext.GetConnectivity(ctx, Default)
			parentQueue, reclaimeeQueue, reclaimerQueue := createQueues(4, 1, 1)
			testCtx.InitQueues([]*v2.Queue{parentQueue, reclaimeeQueue, reclaimerQueue})

			pod := createPodWithDRAClaim(ctx, testCtx, reclaimeeQueue, 2)
			pod2 := createPodWithDRAClaim(ctx, testCtx, reclaimeeQueue, 1)
			pod3 := createPodWithDRAClaim(ctx, testCtx, reclaimeeQueue, 1)

			wait.ForPodScheduled(ctx, testCtx.ControllerClient, pod)
			wait.ForPodScheduled(ctx, testCtx.ControllerClient, pod2)
			wait.ForPodScheduled(ctx, testCtx.ControllerClient, pod3)

			runningPod := createPodWithDRAClaim(ctx, testCtx, reclaimerQueue, 1)
			wait.ForPodScheduled(ctx, testCtx.ControllerClient, runningPod)

			pendingPod := createPodWithDRAClaim(ctx, testCtx, reclaimerQueue, 2)
			wait.ForPodUnschedulable(ctx, testCtx.ControllerClient, pendingPod)
		})

		It("Over quota and Over fair share -> Over quota and Over quota - Should reclaim", func(ctx context.Context) {
			// 4 GPUs in total (quota of 0)
			// reclaimee: 1+2 => 3 (OFS)                  =>  1 (OQ)
			// reclaimer: 1   => 1 + requesting 1 (OQ)  =>  2 (OQ)

			testCtx = testcontext.GetConnectivity(ctx, Default)
			parentQueue, reclaimeeQueue, reclaimerQueue := createQueues(4, 0, 0)
			testCtx.InitQueues([]*v2.Queue{parentQueue, reclaimeeQueue, reclaimerQueue})

			runningPod := createPodWithDRAClaim(ctx, testCtx, reclaimeeQueue, 1)
			runningPod2 := createPodWithDRAClaim(ctx, testCtx, reclaimeeQueue, 2)

			wait.ForPodScheduled(ctx, testCtx.ControllerClient, runningPod)
			wait.ForPodScheduled(ctx, testCtx.ControllerClient, runningPod2)

			runningPod3 := createPodWithDRAClaim(ctx, testCtx, reclaimerQueue, 1)
			wait.ForPodScheduled(ctx, testCtx.ControllerClient, runningPod3)

			pendingPod := createPodWithDRAClaim(ctx, testCtx, reclaimerQueue, 1)
			wait.ForPodScheduled(ctx, testCtx.ControllerClient, pendingPod)
		})

		It("Under Quota and Over Quota -> Under Quota and Over Quota - Should reclaim", func(ctx context.Context) {
			// 4 GPUs in total (quota of 2)
			// reclaimee: 2+1+1 => 4 (OQ)          => 3 (OQ)
			// reclaimer: 0     => requesting 1 (UQ) => 1 (UQ)

			testCtx = testcontext.GetConnectivity(ctx, Default)
			parentQueue, reclaimeeQueue, reclaimerQueue := createQueues(4, 2, 2)
			testCtx.InitQueues([]*v2.Queue{parentQueue, reclaimeeQueue, reclaimerQueue})

			runningPod := createPodWithDRAClaim(ctx, testCtx, reclaimeeQueue, 2)
			runningPod2 := createPodWithDRAClaim(ctx, testCtx, reclaimeeQueue, 1)
			runningPod3 := createPodWithDRAClaim(ctx, testCtx, reclaimeeQueue, 1)

			wait.ForPodScheduled(ctx, testCtx.ControllerClient, runningPod)
			wait.ForPodScheduled(ctx, testCtx.ControllerClient, runningPod2)
			wait.ForPodScheduled(ctx, testCtx.ControllerClient, runningPod3)

			pendingPod := createPodWithDRAClaim(ctx, testCtx, reclaimerQueue, 1)
			wait.ForPodScheduled(ctx, testCtx.ControllerClient, pendingPod)
		})

		It("Under Quota and Over Quota -> Over Quota and Over Quota - Should reclaim", func(ctx context.Context) {
			// 4 GPUs in total (quota of 0)
			// reclaimee: 2 + 1 + 1 => 4 (OQ) => 3 (OQ)
			// reclaimer: 0         => requesting 1 (UQ)  => 1 (OQ)

			testCtx = testcontext.GetConnectivity(ctx, Default)
			parentQueue, reclaimeeQueue, reclaimerQueue := createQueues(4, 0, 0)
			testCtx.InitQueues([]*v2.Queue{parentQueue, reclaimeeQueue, reclaimerQueue})

			runningPod := createPodWithDRAClaim(ctx, testCtx, reclaimeeQueue, 2)
			runningPod2 := createPodWithDRAClaim(ctx, testCtx, reclaimeeQueue, 1)
			runningPod3 := createPodWithDRAClaim(ctx, testCtx, reclaimeeQueue, 1)

			wait.ForPodScheduled(ctx, testCtx.ControllerClient, runningPod)
			wait.ForPodScheduled(ctx, testCtx.ControllerClient, runningPod2)
			wait.ForPodScheduled(ctx, testCtx.ControllerClient, runningPod3)

			pendingPod := createPodWithDRAClaim(ctx, testCtx, reclaimerQueue, 1)
			wait.ForPodScheduled(ctx, testCtx.ControllerClient, pendingPod)
		})

		It("Simple priority reclaim", func(ctx context.Context) {
			testCtx = testcontext.GetConnectivity(ctx, Default)
			parentQueue, reclaimeeQueue, reclaimerQueue := createQueues(4, 1, 0)
			reclaimerQueue.Spec.Priority = pointer.Int(constants.DefaultQueuePriority + 1)
			testCtx.InitQueues([]*v2.Queue{parentQueue, reclaimeeQueue, reclaimerQueue})

			pod := createPodWithDRAClaim(ctx, testCtx, reclaimeeQueue, 1)
			wait.ForPodScheduled(ctx, testCtx.ControllerClient, pod)

			reclaimee := createPodWithDRAClaim(ctx, testCtx, reclaimeeQueue, 3)
			wait.ForPodScheduled(ctx, testCtx.ControllerClient, reclaimee)

			reclaimer := createPodWithDRAClaim(ctx, testCtx, reclaimerQueue, 1)
			wait.ForPodScheduled(ctx, testCtx.ControllerClient, reclaimer)
		})

		It("Simple priority reclaim - but has min runtime", func(ctx context.Context) {
			testCtx = testcontext.GetConnectivity(ctx, Default)
			parentQueue, reclaimeeQueue, reclaimerQueue := createQueues(4, 1, 0)
			reclaimerQueue.Spec.Priority = pointer.Int(constants.DefaultQueuePriority + 1)
			reclaimeeQueue.Spec.ReclaimMinRuntime = &metav1.Duration{Duration: 60 * time.Second}
			testCtx.InitQueues([]*v2.Queue{parentQueue, reclaimeeQueue, reclaimerQueue})

			pod := createPodWithDRAClaim(ctx, testCtx, reclaimeeQueue, 1)
			wait.ForPodScheduled(ctx, testCtx.ControllerClient, pod)

			reclaimee := createPodWithDRAClaim(ctx, testCtx, reclaimeeQueue, 3)
			wait.ForPodScheduled(ctx, testCtx.ControllerClient, reclaimee)

			reclaimer := createPodWithDRAClaim(ctx, testCtx, reclaimerQueue, 1)
			wait.ForPodUnschedulable(ctx, testCtx.ControllerClient, reclaimer)
			wait.ForPodScheduled(ctx, testCtx.ControllerClient, reclaimer)
		})

		It("Reclaim based on priority, maintain over-quota weight proportion - DRA", func(ctx context.Context) {
			// 8 GPUs in total
			// reclaimee1, reclaimee 2: 1 deserved each, 6 GPUs leftover
			// reclaimee1: OQW 1: 2 over-quota
			// reclaimee2: OQW 2: 4 over-quota
			// Total: reclaimee 1: 3 allocated, reclaimee 2: 5 allocated
			// reclaimer: 0 deserved, priority +1, 3 GPUs requested

			testCtx = testcontext.GetConnectivity(ctx, Default)

			capacity.SkipIfInsufficientDynamicResources(testCtx.KubeClientset, gpuDeviceClassName, 1, 8)

			nodes, err := rd.FindNodesWithExactAllocatableDRAGPUs(ctx, testCtx.ControllerClient, 8)
			Expect(err).To(Succeed())
			Expect(len(nodes)).To(BeNumerically(">", 0))
			testNodeName := nodes[0].Name

			parentQueue, reclaimee1Queue, reclaimee2Queue := createQueues(8, 1, 1)
			reclaimee1Queue.Spec.Resources.GPU.OverQuotaWeight = 1
			reclaimee2Queue.Spec.Resources.GPU.OverQuotaWeight = 2

			reclaimeeQueueName := utils.GenerateRandomK8sName(10)
			reclaimerQueue := queue.CreateQueueObjectWithGpuResource(reclaimeeQueueName,
				v2.QueueResource{
					Quota:           0,
					OverQuotaWeight: 1,
					Limit:           -1,
				}, parentQueue.Name)
			reclaimerQueue.Spec.Priority = pointer.Int(constants.DefaultQueuePriority + 1)

			testCtx.InitQueues([]*v2.Queue{parentQueue, reclaimee1Queue, reclaimee2Queue, reclaimerQueue})
			reclaimee1Namespace := queue.GetConnectedNamespaceToQueue(reclaimee1Queue)
			reclaimee2Namespace := queue.GetConnectedNamespaceToQueue(reclaimee2Queue)

			// Create claim template for the job
			claimTemplate := rd.CreateResourceClaimTemplate(reclaimee1Namespace, reclaimee1Queue.Name, gpuDeviceClassName, 1)
			claimTemplate, err = testCtx.KubeClientset.ResourceV1().ResourceClaimTemplates(reclaimee1Namespace).Create(ctx, claimTemplate, metav1.CreateOptions{})
			Expect(err).To(Succeed())

			// Submit more jobs than can schedule
			for range 10 {
				job := rd.CreateBatchJobObject(reclaimee1Queue, v1.ResourceRequirements{})
				job.Spec.Template.Spec.NodeSelector = map[string]string{
					"kubernetes.io/hostname": testNodeName,
				}

				job.Spec.Template.Spec.ResourceClaims = []v1.PodResourceClaim{{
					Name:                      "gpu-claim",
					ResourceClaimTemplateName: ptr.To(claimTemplate.Name),
				}}

				err = testCtx.ControllerClient.Create(ctx, job)
				Expect(err).To(Succeed())
			}

			// Create claim template for the job
			claimTemplate2 := rd.CreateResourceClaimTemplate(reclaimee2Namespace, reclaimee2Queue.Name, gpuDeviceClassName, 1)
			claimTemplate2, err = testCtx.KubeClientset.ResourceV1().ResourceClaimTemplates(reclaimee2Namespace).Create(ctx, claimTemplate2, metav1.CreateOptions{})
			Expect(err).To(Succeed())

			for range 10 {
				job := rd.CreateBatchJobObject(reclaimee2Queue, v1.ResourceRequirements{})
				job.Spec.Template.Spec.NodeSelector = map[string]string{
					"kubernetes.io/hostname": testNodeName,
				}

				job.Spec.Template.Spec.ResourceClaims = []v1.PodResourceClaim{{
					Name:                      "gpu-claim",
					ResourceClaimTemplateName: ptr.To(claimTemplate2.Name),
				}}

				err = testCtx.ControllerClient.Create(ctx, job)
				Expect(err).To(Succeed())
			}

			selector := metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": "engine-e2e",
				},
			}
			wait.ForAtLeastNPodCreation(ctx, testCtx.ControllerClient, selector, 20)

			// Verify expected ratios between queues: 1 + 2 for reclaimee1, 1 + 4 for reclaimee2 - based on weights
			reclaimee1Pods, err := testCtx.KubeClientset.CoreV1().Pods(reclaimee1Namespace).List(ctx, metav1.ListOptions{})
			Expect(err).To(Succeed())
			wait.ForAtLeastNPodsScheduled(ctx, testCtx.ControllerClient, reclaimee1Namespace, podListToPodsSlice(reclaimee1Pods), 3)

			reclaimee2Pods, err := testCtx.KubeClientset.CoreV1().Pods(reclaimee2Namespace).List(ctx, metav1.ListOptions{})
			Expect(err).To(Succeed())
			wait.ForAtLeastNPodsScheduled(ctx, testCtx.ControllerClient, reclaimee2Namespace, podListToPodsSlice(reclaimee2Pods), 5)

			// Submit reclaimer pod that requests 3 GPUs
			reclaimerNamespace := queue.GetConnectedNamespaceToQueue(reclaimerQueue)

			// Create claim for the reclaimer pod
			claim := rd.CreateResourceClaim(reclaimerNamespace, reclaimerQueue.Name, gpuDeviceClassName, 3)
			claim, err = testCtx.KubeClientset.ResourceV1().ResourceClaims(reclaimerNamespace).Create(ctx, claim, metav1.CreateOptions{})
			Expect(err).To(Succeed())

			reclaimerPod := rd.CreatePodObject(reclaimerQueue, v1.ResourceRequirements{})
			reclaimerPod.Spec.NodeSelector = map[string]string{
				"kubernetes.io/hostname": testNodeName,
			}
			reclaimerPod.Spec.ResourceClaims = []v1.PodResourceClaim{{
				Name:              "gpu-claim",
				ResourceClaimName: ptr.To(claim.Name),
			}}

			reclaimerPod, err = rd.CreatePod(ctx, testCtx.KubeClientset, reclaimerPod)
			Expect(err).To(Succeed())

			// Update the claim to set the pod as owner
			claim.OwnerReferences = []metav1.OwnerReference{
				{
					APIVersion: "v1",
					Kind:       "Pod",
					Name:       reclaimerPod.Name,
					UID:        reclaimerPod.UID,
				},
			}
			_, err = testCtx.KubeClientset.ResourceV1().ResourceClaims(reclaimerNamespace).Update(ctx, claim, metav1.UpdateOptions{})
			Expect(err).To(Succeed())

			wait.ForPodScheduled(ctx, testCtx.ControllerClient, reclaimerPod)
			time.Sleep(3 * time.Second)

			wait.ForPodsWithCondition(ctx, testCtx.ControllerClient, func(event watch.Event) bool {
				pods, ok := event.Object.(*v1.PodList)
				if !ok {
					return false
				}
				return len(pods.Items) == 10
			},
				runtimeClient.InNamespace(reclaimee1Namespace),
			)

			wait.ForPodsWithCondition(ctx, testCtx.ControllerClient, func(event watch.Event) bool {
				pods, ok := event.Object.(*v1.PodList)
				if !ok {
					return false
				}
				return len(pods.Items) == 10
			},
				runtimeClient.InNamespace(reclaimee2Namespace),
			)

			// Verify that reclaimees maintain over-quota weights: 1 + 1 for reclaimee1, 1 + 2 for reclaimee2
			reclaimee1Pods, err = testCtx.KubeClientset.CoreV1().Pods(reclaimee1Namespace).List(ctx, metav1.ListOptions{})
			Expect(err).To(Succeed())
			wait.ForAtLeastNPodsScheduled(ctx, testCtx.ControllerClient, reclaimee1Namespace, podListToPodsSlice(reclaimee1Pods), 2)
			wait.ForAtLeastNPodsUnschedulable(ctx, testCtx.ControllerClient, reclaimee1Namespace, podListToPodsSlice(reclaimee1Pods), 8)

			reclaimee2Pods, err = testCtx.KubeClientset.CoreV1().Pods(reclaimee2Namespace).List(ctx, metav1.ListOptions{})
			Expect(err).To(Succeed())
			wait.ForAtLeastNPodsScheduled(ctx, testCtx.ControllerClient, reclaimee2Namespace, podListToPodsSlice(reclaimee2Pods), 3)
			wait.ForAtLeastNPodsUnschedulable(ctx, testCtx.ControllerClient, reclaimee2Namespace, podListToPodsSlice(reclaimee2Pods), 7)
		})
	})
})
