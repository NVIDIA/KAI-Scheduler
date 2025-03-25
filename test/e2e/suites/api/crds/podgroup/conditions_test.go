/*
Copyright 2025 NVIDIA CORPORATION
SPDX-License-Identifier: Apache-2.0
*/
package podgroup

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	runtimeClient "sigs.k8s.io/controller-runtime/pkg/client"

	v2 "github.com/NVIDIA/KAI-scheduler/pkg/apis/scheduling/v2"
	"github.com/NVIDIA/KAI-scheduler/pkg/apis/scheduling/v2alpha2"
	"github.com/NVIDIA/KAI-scheduler/pkg/common/constants"
	e2econstant "github.com/NVIDIA/KAI-scheduler/test/e2e/modules/constant"
	testcontext "github.com/NVIDIA/KAI-scheduler/test/e2e/modules/context"
	"github.com/NVIDIA/KAI-scheduler/test/e2e/modules/resources/rd"
	"github.com/NVIDIA/KAI-scheduler/test/e2e/modules/resources/rd/queue"
	"github.com/NVIDIA/KAI-scheduler/test/e2e/modules/utils"
)

var _ = Describe("PodGroup Conditions", Ordered, func() {
	var (
		testCtx                     *testcontext.TestContext
		testQueue                   *v2.Queue
		nonPreemptiblePriorityClass string
	)

	BeforeAll(func(ctx context.Context) {
		testCtx = testcontext.GetConnectivity(ctx, Default)

		parentQueue := queue.CreateQueueObject(utils.GenerateRandomK8sName(10), "")
		testQueue = queue.CreateQueueObject(utils.GenerateRandomK8sName(10), parentQueue.Name)
		testCtx.InitQueues([]*v2.Queue{testQueue, parentQueue})

		nonPreemptiblePriorityClass = "high"
		nonPreemptiblePriorityValue := e2econstant.NonPreemptiblePriorityThreshold + 2
		_, err := testCtx.KubeClientset.SchedulingV1().PriorityClasses().
			Create(ctx, rd.CreatePriorityClass(nonPreemptiblePriorityClass, nonPreemptiblePriorityValue),
				metav1.CreateOptions{})
		Expect(err).NotTo(HaveOccurred())
	})

	AfterAll(func(ctx context.Context) {
		testCtx.ClusterCleanup(ctx)
		Expect(rd.DeleteAllE2EPriorityClasses(ctx, testCtx.ControllerClient)).To(Succeed())
	})

	AfterEach(func(ctx context.Context) {
		testCtx.TestContextCleanup(ctx)
	})

	Context("Non preemptible Jobs over quota", func() {
		BeforeAll(func(ctx context.Context) {
			originalTestQueue := testQueue.DeepCopy()
			testQueue.Spec.Resources.GPU.Quota = 0
			Expect(testCtx.ControllerClient.Patch(ctx, testQueue, runtimeClient.MergeFrom(originalTestQueue))).To(Succeed())
		})
		AfterAll(func(ctx context.Context) {
			originalTestQueue := testQueue.DeepCopy()
			testQueue.Spec.Resources.GPU.Quota = -1
			Expect(testCtx.ControllerClient.Patch(ctx, testQueue, runtimeClient.MergeFrom(originalTestQueue))).To(Succeed())
		})
		It("sets condition with NonPreemptibleOverQuota reason on PodGroup", func(ctx context.Context) {
			pod := rd.CreatePodObject(testQueue, v1.ResourceRequirements{
				Limits: v1.ResourceList{
					constants.GpuResource: resource.MustParse("1"),
				},
			})
			pod.Spec.PriorityClassName = nonPreemptiblePriorityClass
			createdPod, err := rd.CreatePod(ctx, testCtx.KubeClientset, pod)
			Expect(err).NotTo(HaveOccurred())

			waitForPGConditionReason(ctx, testCtx, createdPod, v2alpha2.NonPreemptibleOverQuota)
		})
	})

	Context("Jobs Over Queue Limit", func() {
		BeforeAll(func(ctx context.Context) {
			originalTestQueue := testQueue.DeepCopy()
			testQueue.Spec.Resources.GPU.Limit = 0
			Expect(testCtx.ControllerClient.Patch(ctx, testQueue, runtimeClient.MergeFrom(originalTestQueue))).To(Succeed())
		})
		AfterAll(func(ctx context.Context) {
			originalTestQueue := testQueue.DeepCopy()
			testQueue.Spec.Resources.GPU.Limit = -1
			Expect(testCtx.ControllerClient.Patch(ctx, testQueue, runtimeClient.MergeFrom(originalTestQueue))).To(Succeed())
		})

		Context("Preemptible Job", func() {
			It("sets condition with reason on PodGroup", func(ctx context.Context) {
				pod := rd.CreatePodObject(testQueue, v1.ResourceRequirements{
					Limits: v1.ResourceList{
						constants.GpuResource: resource.MustParse("1"),
					},
				})
				createdPod, err := rd.CreatePod(ctx, testCtx.KubeClientset, pod)
				Expect(err).NotTo(HaveOccurred())

				waitForPGConditionReason(ctx, testCtx, createdPod, v2alpha2.OverLimit)
			})
		})

		Context("NonPreemptible Job", func() {
			It("sets condition with reason on PodGroup", func(ctx context.Context) {
				pod := rd.CreatePodObject(testQueue, v1.ResourceRequirements{
					Limits: v1.ResourceList{
						constants.GpuResource: resource.MustParse("1"),
					},
				})
				createdPod, err := rd.CreatePod(ctx, testCtx.KubeClientset, pod)
				createdPod.Spec.PriorityClassName = nonPreemptiblePriorityClass
				Expect(err).NotTo(HaveOccurred())

				waitForPGConditionReason(ctx, testCtx, createdPod, v2alpha2.OverLimit)
			})
		})
	})
})

func waitForPGConditionReason(
	ctx context.Context, testCtx *testcontext.TestContext,
	pod *v1.Pod, expectedReason v2alpha2.UnschedulableReason,
) *v2alpha2.PodGroup {
	podGroup := &v2alpha2.PodGroup{}
	Eventually(func() bool {
		updatedPod := &v1.Pod{}
		Expect(testCtx.ControllerClient.Get(ctx, runtimeClient.ObjectKeyFromObject(pod), updatedPod)).To(Succeed())
		podGroupName, found := updatedPod.Annotations[constants.PodGroupAnnotationForPod]
		if !found {
			return false
		}
		Expect(testCtx.ControllerClient.Get(ctx, runtimeClient.ObjectKey{Name: podGroupName, Namespace: pod.Namespace}, podGroup)).To(Succeed())

		for _, condition := range podGroup.Status.SchedulingConditions {
			for _, reason := range condition.Reasons {
				if reason.Reason == expectedReason {
					return true
				}
			}
		}

		return false
	}, time.Minute, time.Millisecond*100).Should(BeTrue())

	return podGroup
}
