/*
Copyright 2025 NVIDIA CORPORATION
SPDX-License-Identifier: Apache-2.0
*/
package node_order

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"golang.org/x/exp/maps"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	runtimeClient "sigs.k8s.io/controller-runtime/pkg/client"

	v2 "github.com/NVIDIA/KAI-scheduler/pkg/apis/scheduling/v2"
	"github.com/NVIDIA/KAI-scheduler/test/e2e/modules/constant"
	testcontext "github.com/NVIDIA/KAI-scheduler/test/e2e/modules/context"
	"github.com/NVIDIA/KAI-scheduler/test/e2e/modules/resources/capacity"
	"github.com/NVIDIA/KAI-scheduler/test/e2e/modules/resources/rd"
	"github.com/NVIDIA/KAI-scheduler/test/e2e/modules/resources/rd/queue"
	"github.com/NVIDIA/KAI-scheduler/test/e2e/modules/utils"
	"github.com/NVIDIA/KAI-scheduler/test/e2e/modules/wait"
)

var (
	podLabels = map[string]string{"hello": "world"}
)

var _ = Describe("Affinity", Ordered, func() {
	var (
		testCtx                 *testcontext.TestContext
		namespace               string
		pod                     *v1.Pod
		weightedPodAffinityTerm []v1.WeightedPodAffinityTerm
	)

	BeforeAll(func(ctx context.Context) {
		testCtx = testcontext.GetConnectivity(ctx, Default)
		parentQueue := queue.CreateQueueObject(utils.GenerateRandomK8sName(10), "")
		childQueue := queue.CreateQueueObject(utils.GenerateRandomK8sName(10), parentQueue.Name)
		testCtx.InitQueues([]*v2.Queue{childQueue, parentQueue})
		namespace = queue.GetConnectedNamespaceToQueue(testCtx.Queues[0])
		capacity.SkipIfInsufficientClusterTopologyResources(testCtx.KubeClientset, []capacity.ResourceList{
			{
				PodCount: 2,
			},
			{
				PodCount: 2,
			},
		})

		weightedPodAffinityTerm = []v1.WeightedPodAffinityTerm{
			{
				Weight: 10,
				PodAffinityTerm: v1.PodAffinityTerm{
					LabelSelector: &metav1.LabelSelector{
						MatchLabels: podLabels,
					},
					Namespaces:  []string{namespace},
					TopologyKey: constant.NodeNamePodLabelName,
				},
			},
		}
	})

	AfterAll(func(ctx context.Context) {
		testCtx.ClusterCleanup(ctx)
	})

	BeforeEach(func(ctx context.Context) {
		var err error
		pod = rd.CreatePodObject(testCtx.Queues[0], v1.ResourceRequirements{})
		maps.Copy(pod.Labels, podLabels)
		pod, err = rd.CreatePod(ctx, testCtx.KubeClientset, pod)
		Expect(err).To(Succeed())
		wait.ForPodScheduled(ctx, testCtx.ControllerClient, pod)
		Expect(testCtx.ControllerClient.Get(
			ctx, runtimeClient.ObjectKeyFromObject(pod), pod)).To(Succeed())
	})

	AfterEach(func(ctx context.Context) {
		testCtx.TestContextCleanup(ctx)
	})

	Context("Pod Affinity", func() {
		It("schedules the a new pod with matching labels pod affinity", func(ctx context.Context) {
			testedPod := rd.CreatePodObject(testCtx.Queues[0], v1.ResourceRequirements{})
			testedPod.Spec.Affinity = &v1.Affinity{
				PodAffinity: &v1.PodAffinity{
					PreferredDuringSchedulingIgnoredDuringExecution: weightedPodAffinityTerm,
				},
			}

			testedPod, err := rd.CreatePod(ctx, testCtx.KubeClientset, testedPod)
			Expect(err).To(Succeed())
			wait.ForPodScheduled(ctx, testCtx.ControllerClient, testedPod)
			Expect(testCtx.ControllerClient.Get(
				ctx, runtimeClient.ObjectKeyFromObject(testedPod), testedPod)).To(Succeed())

			Expect(testedPod.Spec.NodeName).To(Equal(pod.Spec.NodeName))
		})

		It("schedules the a new pod NOT with matching labels pod anti-affinity", func(ctx context.Context) {
			testedPod := rd.CreatePodObject(testCtx.Queues[0], v1.ResourceRequirements{})
			testedPod.Spec.Affinity = &v1.Affinity{
				PodAntiAffinity: &v1.PodAntiAffinity{
					PreferredDuringSchedulingIgnoredDuringExecution: weightedPodAffinityTerm,
				},
			}
			testedPod, err := rd.CreatePod(ctx, testCtx.KubeClientset, testedPod)
			Expect(err).To(Succeed())
			wait.ForPodScheduled(ctx, testCtx.ControllerClient, testedPod)
			Expect(testCtx.ControllerClient.Get(
				ctx, runtimeClient.ObjectKeyFromObject(testedPod), testedPod)).To(Succeed())

			Expect(testedPod.Spec.NodeName).NotTo(Equal(pod.Spec.NodeName))
		})
	})
})
