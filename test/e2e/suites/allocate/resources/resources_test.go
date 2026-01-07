/*
Copyright 2025 NVIDIA CORPORATION
SPDX-License-Identifier: Apache-2.0
*/
package resources

import (
	"context"
	"sync"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	v2 "github.com/NVIDIA/KAI-scheduler/pkg/apis/scheduling/v2"
	"github.com/NVIDIA/KAI-scheduler/pkg/common/constants"
	"github.com/NVIDIA/KAI-scheduler/test/e2e/modules/constant/labels"
	testcontext "github.com/NVIDIA/KAI-scheduler/test/e2e/modules/context"
	"github.com/NVIDIA/KAI-scheduler/test/e2e/modules/resources/capacity"
	"github.com/NVIDIA/KAI-scheduler/test/e2e/modules/resources/rd"
	"github.com/NVIDIA/KAI-scheduler/test/e2e/modules/resources/rd/queue"
	"github.com/NVIDIA/KAI-scheduler/test/e2e/modules/utils"
	"github.com/NVIDIA/KAI-scheduler/test/e2e/modules/wait"
	runtimeClient "sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("Schedule pod with resource request", Ordered, func() {
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

	It("No resource requests", func(ctx context.Context) {
		pod := rd.CreatePodObject(testCtx.Queues[0], v1.ResourceRequirements{})

		_, err := rd.CreatePod(ctx, testCtx.KubeClientset, pod)
		Expect(err).NotTo(HaveOccurred())

		wait.ForPodScheduled(ctx, testCtx.ControllerClient, pod)
	})

	Context("GPU Resources", func() {
		BeforeAll(func(ctx context.Context) {
			capacity.SkipIfInsufficientClusterResources(testCtx.KubeClientset,
				&capacity.ResourceList{
					Gpu:      resource.MustParse("1"),
					PodCount: 1,
				},
			)
		})

		It("Whole GPU request", func(ctx context.Context) {
			pod := rd.CreatePodObject(testCtx.Queues[0], v1.ResourceRequirements{
				Limits: map[v1.ResourceName]resource.Quantity{
					constants.GpuResource: resource.MustParse("1"),
				},
			})

			_, err := rd.CreatePod(ctx, testCtx.KubeClientset, pod)
			Expect(err).NotTo(HaveOccurred())

			wait.ForPodScheduled(ctx, testCtx.ControllerClient, pod)
		})

		It("Fraction GPU request", Label(labels.ReservationPod), func(ctx context.Context) {
			pod := rd.CreatePodObject(testCtx.Queues[0], v1.ResourceRequirements{})
			pod.Annotations = map[string]string{
				constants.GpuFraction: "0.5",
			}

			_, err := rd.CreatePod(ctx, testCtx.KubeClientset, pod)
			Expect(err).NotTo(HaveOccurred())

			wait.ForPodReady(ctx, testCtx.ControllerClient, pod)
		})

		It("Fraction GPU request - fill all the GPUs", Label(labels.ReservationPod), func(ctx context.Context) {
			numGPUs := 0
			var nodes v1.NodeList
			Expect(testCtx.ControllerClient.List(ctx, &nodes)).To(Succeed())
			for _, node := range nodes.Items {
				q := node.Status.Capacity[constants.GpuResource]
				if q.Value() > 0 {
					numGPUs += int(q.Value())
				}
			}
			Expect(numGPUs).To(BeNumerically(">", 0), "No GPUs found in cluster")

			numPods := numGPUs * 2
			pods := make([]*v1.Pod, numPods)
			for i := range numPods {
				pods[i] = rd.CreatePodObject(testCtx.Queues[0], v1.ResourceRequirements{})
				pods[i].Annotations = map[string]string{
					constants.GpuFraction: "0.7",
				}
			}
			errs := make(chan error, len(pods))
			var wg sync.WaitGroup
			for _, pod := range pods {
				wg.Add(1)
				go func() {
					defer wg.Done()
					_, err := rd.CreatePod(ctx, testCtx.KubeClientset, pod)
					errs <- err
				}()
			}
			wg.Wait()
			close(errs)
			for err := range errs {
				Expect(err).NotTo(HaveOccurred(), "Failed to create pod")
			}

			wait.ForAtLeastNPodsScheduled(ctx, testCtx.ControllerClient, testCtx.Queues[0].Namespace, pods, numGPUs)
			wait.ForAtLeastNPodCreation(ctx, testCtx.ControllerClient, metav1.LabelSelector{
				MatchLabels: map[string]string{constants.AppLabelName: "engine-e2e"},
				MatchExpressions: []metav1.LabelSelectorRequirement{
					{
						Key:      constants.GPUGroup,
						Operator: metav1.LabelSelectorOpExists,
					},
				},
			}, numGPUs)

			time.Sleep(3 * time.Second)

			var allPods v1.PodList
			n := 0
			for range 5 {
				Expect(testCtx.ControllerClient.List(ctx, &allPods,
					runtimeClient.InNamespace(testCtx.Queues[0].Namespace),
					runtimeClient.MatchingLabels{constants.AppLabelName: "engine-e2e"},
				)).To(Succeed(), "Failed to list pods")
				n = 0
				for _, pod := range allPods.Items {
					if !rd.IsPodScheduled(&pod) {
						continue
					}
					n++
				}
				if n == numGPUs {
					break
				}
				time.Sleep(2 * time.Second)
			}
			Expect(n).To(BeNumerically(">=", numGPUs), "Expected %d allocated pods, got %d", numGPUs, n)

			var allocatedPods []*v1.Pod
			gpuGroupsMap := make(map[string][]*v1.Pod)
			var configMaps v1.ConfigMapList
			Expect(testCtx.ControllerClient.List(ctx, &configMaps,
				runtimeClient.InNamespace(testCtx.Queues[0].Namespace),
			)).To(Succeed(), "Failed to list config maps")
			cmByIndex := make(map[string][]*v1.ConfigMap)
			for _, pod := range allPods.Items {
				if !rd.IsPodScheduled(&pod) {
					continue
				}
				allocatedPods = append(allocatedPods, &pod)

				group, ok := pod.Labels[constants.GPUGroup]
				Expect(ok).To(BeTrue(), "GPU group label not found")
				gpuGroupsMap[group] = append(gpuGroupsMap[group], &pod)

				cmName := pod.Annotations[constants.GpuSharingConfigMapAnnotation]
				for _, cm := range configMaps.Items {
					if cm.Name != cmName {
						continue
					}
					index := cm.Data[constants.NvidiaVisibleDevices]
					cmByIndex[index] = append(cmByIndex[index], &cm)
					break
				}
			}
			Expect(len(allocatedPods)).To(BeNumerically(">=", numGPUs), "Expected at least %d allocated pods, got %d", numGPUs, len(allocatedPods))

			for group, pods := range gpuGroupsMap {
				Expect(len(pods)).To(Equal(1), "Expected one pod per group, got %d for group %s", len(pods), group)
			}

			for index, cm := range cmByIndex {
				Expect(len(cm)).To(Equal(1), "Expected one config map per index, got %d for index %s", len(cm), index)
			}
		})

		It("GPU memory request - valid", Label(labels.ReservationPod), func(ctx context.Context) {
			pod := rd.CreatePodObject(testCtx.Queues[0], v1.ResourceRequirements{})
			pod.Annotations = map[string]string{
				constants.GpuMemory: "500",
			}

			_, err := rd.CreatePod(ctx, testCtx.KubeClientset, pod)
			Expect(err).NotTo(HaveOccurred())

			wait.ForPodScheduled(ctx, testCtx.ControllerClient, pod)
		})

		It("GPU memory request - too much memory for a single gpu", Label(labels.ReservationPod),
			func(ctx context.Context) {
				pod := rd.CreatePodObject(testCtx.Queues[0], v1.ResourceRequirements{})
				pod.Annotations = map[string]string{
					constants.GpuMemory: "500000000",
				}

				_, err := rd.CreatePod(ctx, testCtx.KubeClientset, pod)
				Expect(err).NotTo(HaveOccurred())

				wait.ForPodUnschedulable(ctx, testCtx.ControllerClient, pod)
			})
	})

	Context("CPU Resources", func() {
		It("CPU Request", func(ctx context.Context) {
			capacity.SkipIfInsufficientClusterResources(testCtx.KubeClientset,
				&capacity.ResourceList{
					Cpu:      resource.MustParse("1"),
					PodCount: 1,
				},
			)

			pod := rd.CreatePodObject(testCtx.Queues[0], v1.ResourceRequirements{
				Requests: map[v1.ResourceName]resource.Quantity{
					v1.ResourceCPU: resource.MustParse("1"),
				},
			})

			_, err := rd.CreatePod(ctx, testCtx.KubeClientset, pod)
			Expect(err).NotTo(HaveOccurred())

			wait.ForPodScheduled(ctx, testCtx.ControllerClient, pod)
		})

		It("Memory Request", func(ctx context.Context) {
			capacity.SkipIfInsufficientClusterResources(testCtx.KubeClientset,
				&capacity.ResourceList{
					Memory:   resource.MustParse("100M"),
					PodCount: 1,
				},
			)

			pod := rd.CreatePodObject(testCtx.Queues[0], v1.ResourceRequirements{
				Requests: map[v1.ResourceName]resource.Quantity{
					v1.ResourceMemory: resource.MustParse("100M"),
				},
			})

			_, err := rd.CreatePod(ctx, testCtx.KubeClientset, pod)
			Expect(err).NotTo(HaveOccurred())
			wait.ForPodScheduled(ctx, testCtx.ControllerClient, pod)
		})

		It("Other Resource Request", func(ctx context.Context) {
			capacity.SkipIfInsufficientClusterResources(testCtx.KubeClientset,
				&capacity.ResourceList{
					PodCount: 1,
					OtherResources: map[v1.ResourceName]resource.Quantity{
						v1.ResourceEphemeralStorage: resource.MustParse("100M"),
					},
				},
			)

			pod := rd.CreatePodObject(testCtx.Queues[0], v1.ResourceRequirements{
				Requests: map[v1.ResourceName]resource.Quantity{
					v1.ResourceEphemeralStorage: resource.MustParse("100M"),
				},
			})

			_, err := rd.CreatePod(ctx, testCtx.KubeClientset, pod)
			Expect(err).NotTo(HaveOccurred())
			wait.ForPodScheduled(ctx, testCtx.ControllerClient, pod)
		})
	})
})
