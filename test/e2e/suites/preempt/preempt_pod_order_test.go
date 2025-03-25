/*
Copyright 2025 NVIDIA CORPORATION
SPDX-License-Identifier: Apache-2.0
*/
package preempt

import (
	"context"
	"fmt"
	"strconv"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"

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
	taskOrderLabelKey = "run.ai/task-priority"
)

var _ = Describe("Priority pod order preemption with Elastic Jobs", Ordered, func() {
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

	It("High priority elastic job preempts from low priority elastic jobs - pod order", func(ctx context.Context) {
		testCtx = testcontext.GetConnectivity(ctx, Default)
		parentQueue := queue.CreateQueueObject(utils.GenerateRandomK8sName(10), "")
		testQueue := queue.CreateQueueObject(utils.GenerateRandomK8sName(10), parentQueue.Name)
		testQueue.Spec.Resources.GPU.Quota = 4
		testQueue.Spec.Resources.GPU.Limit = 4
		namespace = queue.GetConnectedNamespaceToQueue(testQueue)
		testCtx.InitQueues([]*v2.Queue{testQueue, parentQueue})

		pgName := utils.GenerateRandomK8sName(10)
		podForPodGroupNum := 4

		var preempteePods []*v1.Pod
		for i := 0; i < podForPodGroupNum; i++ {
			pod := rd.CreatePodWithPodGroupReference(testQueue, pgName, v1.ResourceRequirements{
				Limits: map[v1.ResourceName]resource.Quantity{
					constants.GpuResource: resource.MustParse("1"),
				},
			})
			pod.Labels[rd.PodGroupLabelName] = pgName
			pod.Labels[taskOrderLabelKey] = strconv.Itoa(i % 2)
			pod.Labels[priorityClassNameLabelName] = lowPriority
			pod, err := rd.CreatePod(ctx, testCtx.KubeClientset, pod)
			Expect(err).To(Succeed())
			preempteePods = append(preempteePods, pod)
		}
		podGroup := pod_group.Create(namespace, pgName, testQueue.Name)

		_, err := testCtx.KubeAiSchedClientset.SchedulingV2alpha2().PodGroups(namespace).Create(ctx,
			podGroup, metav1.CreateOptions{})
		Expect(err).To(Succeed())

		wait.ForPodsScheduled(ctx, testCtx.ControllerClient, namespace, preempteePods)

		// higher priority pod
		highPodRequirements := v1.ResourceRequirements{
			Limits: map[v1.ResourceName]resource.Quantity{
				constants.GpuResource: resource.MustParse("2"),
			},
		}

		preemptorPod := rd.CreatePodObject(testQueue, highPodRequirements)
		preemptorPod.Labels[priorityClassNameLabelName] = highPriority
		preemptorPod, err = rd.CreatePod(ctx, testCtx.KubeClientset, preemptorPod)
		Expect(err).To(Succeed())
		wait.ForPodScheduled(ctx, testCtx.ControllerClient, preemptorPod)

		// verify results
		wait.ForPodsWithCondition(ctx, testCtx.ControllerClient, func(watch.Event) bool {
			pods, err := testCtx.KubeClientset.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{
				LabelSelector: fmt.Sprintf("%s=%s", rd.PodGroupLabelName, pgName),
			})
			Expect(err).To(Succeed())

			priorityOnePodsCounter := 0
			for _, pod := range pods.Items {
				if pod.Labels[taskOrderLabelKey] == "1" {
					priorityOnePodsCounter += 1
				}
			}

			return len(pods.Items) == 2 && priorityOnePodsCounter == 2
		})
	})
})
