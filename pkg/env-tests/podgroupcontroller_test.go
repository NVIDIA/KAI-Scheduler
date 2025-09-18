// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package env_tests

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/xyproto/randomstring"
	"sigs.k8s.io/controller-runtime/pkg/client"

	corev1 "k8s.io/api/core/v1"
	resourcev1beta1 "k8s.io/api/resource/v1beta1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"

	kaiv1alpha2 "github.com/NVIDIA/KAI-scheduler/pkg/apis/scheduling/v1alpha2"
	schedulingv2 "github.com/NVIDIA/KAI-scheduler/pkg/apis/scheduling/v2"
	schedulingv2alpha2 "github.com/NVIDIA/KAI-scheduler/pkg/apis/scheduling/v2alpha2"
	"github.com/NVIDIA/KAI-scheduler/pkg/common/constants"
	"github.com/NVIDIA/KAI-scheduler/pkg/env-tests/binder"
	"github.com/NVIDIA/KAI-scheduler/pkg/env-tests/podgroupcontroller"
)

var _ = Describe("PodGroupController", Ordered, func() {
	// Define a timeout for eventually assertions
	const interval = time.Millisecond * 10
	const defaultTimeout = interval * 200

	var (
		testNamespace  *corev1.Namespace
		testDepartment *schedulingv2.Queue
		testQueue      *schedulingv2.Queue
		testNode       *corev1.Node

		backgroundCtx context.Context
		cancel        context.CancelFunc
	)

	BeforeEach(func(ctx context.Context) {
		// Create a test namespace
		testNamespace = &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test-" + randomstring.HumanFriendlyEnglishString(10),
			},
		}
		Expect(ctrlClient.Create(ctx, testNamespace)).To(Succeed())

		testDepartment = CreateQueueObject("test-department", "")
		Expect(ctrlClient.Create(ctx, testDepartment)).To(Succeed(), "Failed to create test department")

		testQueue = CreateQueueObject("test-queue", testDepartment.Name)
		Expect(ctrlClient.Create(ctx, testQueue)).To(Succeed(), "Failed to create test queue")

		testNode = CreateNodeObject(ctx, ctrlClient, DefaultNodeConfig("test-node"))
		Expect(ctrlClient.Create(ctx, testNode)).To(Succeed(), "Failed to create test node")

		backgroundCtx, cancel = context.WithCancel(context.Background())
		err := podgroupcontroller.RunPodGroupController(cfg, backgroundCtx)
		Expect(err).NotTo(HaveOccurred(), "Failed to run podgroupcontroller")

		err = binder.RunBinder(cfg, backgroundCtx)
		Expect(err).NotTo(HaveOccurred(), "Failed to run binder")
	})

	AfterEach(func(ctx context.Context) {
		Expect(ctrlClient.Delete(ctx, testDepartment)).To(Succeed(), "Failed to delete test department")
		Expect(ctrlClient.Delete(ctx, testQueue)).To(Succeed(), "Failed to delete test queue")
		Expect(ctrlClient.Delete(ctx, testNode)).To(Succeed(), "Failed to delete test node")

		err := WaitForObjectDeletion(ctx, ctrlClient, testDepartment, defaultTimeout, interval)
		Expect(err).NotTo(HaveOccurred(), "Failed to wait for test department to be deleted")

		err = WaitForObjectDeletion(ctx, ctrlClient, testQueue, defaultTimeout, interval)
		Expect(err).NotTo(HaveOccurred(), "Failed to wait for test queue to be deleted")

		err = WaitForObjectDeletion(ctx, ctrlClient, testNode, defaultTimeout, interval)
		Expect(err).NotTo(HaveOccurred(), "Failed to wait for test node to be deleted")

		cancel()
	})

	Context("simple podgroup test", func() {
		AfterEach(func(ctx context.Context) {
			err := DeleteAllInNamespace(ctx, ctrlClient, testNamespace.Name,
				&corev1.Pod{},
				&schedulingv2alpha2.PodGroup{},
				&resourcev1beta1.ResourceClaim{},
				&kaiv1alpha2.BindRequest{},
			)
			Expect(err).NotTo(HaveOccurred(), "Failed to delete test resources")

			err = WaitForNoObjectsInNamespace(ctx, ctrlClient, testNamespace.Name, defaultTimeout, interval,
				&corev1.PodList{},
				&schedulingv2alpha2.PodGroupList{},
				&resourcev1beta1.ResourceClaimList{},
				&kaiv1alpha2.BindRequestList{},
			)
			Expect(err).NotTo(HaveOccurred(), "Failed to wait for test resources to be deleted")
		})

		It("Should update podgroup status", func(ctx context.Context) {
			testPod := CreatePodObject(testNamespace.Name, "test-pod", corev1.ResourceRequirements{
				Limits: corev1.ResourceList{
					constants.GpuResource: resource.MustParse("1"),
				},
			})
			Expect(ctrlClient.Create(ctx, testPod)).To(Succeed(), "Failed to create test pod")

			Expect(GroupPods(ctx, ctrlClient, podGroupConfig{
				queueName:    testQueue.Name,
				podgroupName: "test-podgroup",
				minMember:    1,
			}, []*corev1.Pod{testPod})).To(Succeed(), "Failed to group pod")

			var podgroup schedulingv2alpha2.PodGroup
			Expect(ctrlClient.Get(ctx, client.ObjectKey{Name: "test-podgroup", Namespace: testNamespace.Name}, &podgroup)).To(Succeed(), "Failed to get test podgroup")

			bindRequest := &kaiv1alpha2.BindRequest{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: testNamespace.Name,
					Name:      "test-bind-request",
				},
				Spec: kaiv1alpha2.BindRequestSpec{
					PodName:              testPod.Name,
					SelectedNode:         testNode.Name,
					ReceivedResourceType: "Regular",
					ReceivedGPU: &kaiv1alpha2.ReceivedGPU{
						Count:   1,
						Portion: "1",
					},
				},
			}
			Expect(ctrlClient.Create(ctx, bindRequest)).To(Succeed(), "Failed to create test bind request")

			// Wait for pod to be bound
			err := WaitForPodBound(ctx, ctrlClient, testPod.Name, testNamespace.Name, defaultTimeout, interval)
			Expect(err).NotTo(HaveOccurred(), "Failed to wait for test pod to be bound")

			// Wait for podgroup status allocated resources to be updated
			err = wait.PollUntilContextTimeout(ctx, interval, defaultTimeout, true, func(ctx context.Context) (bool, error) {
				err := ctrlClient.Get(ctx, client.ObjectKey{Name: "test-podgroup", Namespace: testNamespace.Name}, &podgroup)
				if err != nil {
					return false, err
				}

				if podgroup.Status.ResourcesStatus.Allocated == nil {
					return false, nil
				}

				if podgroup.Status.ResourcesStatus.Allocated[constants.GpuResource] != resource.MustParse("1") {
					return false, nil
				}

				return true, nil
			})
			Expect(err).NotTo(HaveOccurred(), "Failed to wait for podgroup status allocated resources to be updated")
		})
	})
})
