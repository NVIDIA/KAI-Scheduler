/*
Copyright 2025 NVIDIA CORPORATION
SPDX-License-Identifier: Apache-2.0
*/
package preempt

import (
	"context"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/utils/pointer"

	v2 "github.com/NVIDIA/KAI-scheduler/pkg/apis/scheduling/v2"
	"github.com/NVIDIA/KAI-scheduler/pkg/common/constants"
	testcontext "github.com/NVIDIA/KAI-scheduler/test/e2e/modules/context"
	"github.com/NVIDIA/KAI-scheduler/test/e2e/modules/resources/capacity"
	"github.com/NVIDIA/KAI-scheduler/test/e2e/modules/resources/rd"
	"github.com/NVIDIA/KAI-scheduler/test/e2e/modules/resources/rd/pod_group"
	"github.com/NVIDIA/KAI-scheduler/test/e2e/modules/resources/rd/queue"
	"github.com/NVIDIA/KAI-scheduler/test/e2e/modules/utils"
	"github.com/NVIDIA/KAI-scheduler/test/e2e/modules/wait"
)

const (
	priorityClassNameLabelName = "priorityClassName"
)

var _ = Describe("Priority Preemption with Elastic Jobs", Ordered, func() {
	var (
		testCtx      *testcontext.TestContext
		namespace    string
		lowPriority  string
		highPriority string
	)

	BeforeAll(func(ctx context.Context) {
		testCtx = testcontext.GetConnectivity(ctx, Default)
		capacity.SkipIfInsufficientClusterResources(testCtx.KubeClientset, &capacity.ResourceList{
			Gpu:      resource.MustParse("4"),
			PodCount: 5,
		})

		var err error
		lowPriority, highPriority, err = rd.CreatePreemptibleAndNonPriorityClass(ctx, testCtx.KubeClientset)
		Expect(err).To(Succeed())
	})

	AfterAll(func(ctx context.Context) {
		err := rd.DeleteAllE2EPriorityClasses(ctx, testCtx.ControllerClient)
		Expect(err).To(Succeed())
	})

	AfterEach(func(ctx context.Context) {
		testCtx.ClusterCleanup(ctx)
	})

	It("High priority elastic job preempts from low priority elastic jobs", func(ctx context.Context) {
		testCtx = testcontext.GetConnectivity(ctx, Default)
		parentQueue := queue.CreateQueueObject(utils.GenerateRandomK8sName(10), "")
		testQueue := queue.CreateQueueObject(utils.GenerateRandomK8sName(10), parentQueue.Name)
		testQueue.Spec.Resources.GPU.Quota = 4
		testQueue.Spec.Resources.GPU.Limit = 4
		namespace = queue.GetConnectedNamespaceToQueue(testQueue)
		testCtx.InitQueues([]*v2.Queue{testQueue, parentQueue})

		lowPodRequirements := v1.ResourceRequirements{
			Limits: map[v1.ResourceName]resource.Quantity{
				constants.GpuResource: resource.MustParse("1"),
			},
		}

		// first job with 2 pods
		podGroup1, pods1 := pod_group.CreateWithPods(ctx, testCtx.KubeClientset, testCtx.KubeAiSchedClientset,
			"elastic-job-low-1", testQueue, 2, pointer.String(lowPriority),
			lowPodRequirements)

		// second job with 2 pods
		podGroup2, pods2 := pod_group.CreateWithPods(ctx, testCtx.KubeClientset, testCtx.KubeAiSchedClientset,
			"elastic-job-low-2", testQueue, 2, pointer.String(lowPriority),
			lowPodRequirements)

		var preempteePods []*v1.Pod
		preempteePods = append(preempteePods, pods1...)
		preempteePods = append(preempteePods, pods2...)
		wait.ForPodsScheduled(ctx, testCtx.ControllerClient, namespace, preempteePods)

		// higher priority pod
		highPodRequirements := v1.ResourceRequirements{
			Limits: map[v1.ResourceName]resource.Quantity{
				constants.GpuResource: resource.MustParse("2"),
			},
		}

		preemptorPod := rd.CreatePodObject(testQueue, highPodRequirements)
		preemptorPod.Labels[priorityClassNameLabelName] = highPriority
		_, err := rd.CreatePod(ctx, testCtx.KubeClientset, preemptorPod)
		Expect(err).To(Succeed())
		wait.ForPodScheduled(ctx, testCtx.ControllerClient, preemptorPod)

		// verify results
		wait.ForPodsWithCondition(ctx, testCtx.ControllerClient, func(watch.Event) bool {
			pods, err := testCtx.KubeClientset.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{
				LabelSelector: fmt.Sprintf("%s=%s", rd.PodGroupLabelName, podGroup1.Name),
			})
			Expect(err).To(Succeed())
			return len(pods.Items) == 1
		})

		wait.ForPodsWithCondition(ctx, testCtx.ControllerClient, func(watch.Event) bool {
			pods, err := testCtx.KubeClientset.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{
				LabelSelector: fmt.Sprintf("%s=%s", rd.PodGroupLabelName, podGroup2.Name),
			})
			Expect(err).To(Succeed())
			return len(pods.Items) == 1
		})
	})

	It("High priority elastic job preempts entire elastic job", func(ctx context.Context) {
		testCtx = testcontext.GetConnectivity(ctx, Default)
		parentQueue := queue.CreateQueueObject(utils.GenerateRandomK8sName(10), "")
		testQueue := queue.CreateQueueObject(utils.GenerateRandomK8sName(10), parentQueue.Name)
		testQueue.Spec.Resources.GPU.Quota = 2
		testQueue.Spec.Resources.GPU.Limit = 2
		namespace = queue.GetConnectedNamespaceToQueue(testQueue)
		testCtx.InitQueues([]*v2.Queue{testQueue, parentQueue})

		lowPodRequirements := v1.ResourceRequirements{
			Limits: map[v1.ResourceName]resource.Quantity{
				constants.GpuResource: resource.MustParse("1"),
			},
		}

		// first job with 2 pods
		preempteePodGroup, preempteePods := pod_group.CreateWithPods(ctx, testCtx.KubeClientset,
			testCtx.KubeAiSchedClientset, "elastic-job-low-1", testQueue, 2,
			pointer.String(lowPriority), lowPodRequirements)
		wait.ForPodsScheduled(ctx, testCtx.ControllerClient, namespace, preempteePods)

		// higher priority pod
		highPodRequirements := v1.ResourceRequirements{
			Limits: map[v1.ResourceName]resource.Quantity{
				constants.GpuResource: resource.MustParse("2"),
			},
		}

		pod := rd.CreatePodObject(testQueue, highPodRequirements)
		pod.Labels[priorityClassNameLabelName] = highPriority
		_, err := rd.CreatePod(ctx, testCtx.KubeClientset, pod)
		Expect(err).To(Succeed())
		wait.ForPodScheduled(ctx, testCtx.ControllerClient, pod)

		// verify results
		wait.ForPodsWithCondition(ctx, testCtx.ControllerClient, func(watch.Event) bool {
			pods, err := testCtx.KubeClientset.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{
				LabelSelector: fmt.Sprintf("%s=%s", rd.PodGroupLabelName, preempteePodGroup.Name),
			})
			Expect(err).To(Succeed())
			return len(pods.Items) == 0
		})
	})
})
