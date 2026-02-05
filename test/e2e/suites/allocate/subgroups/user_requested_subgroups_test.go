/*
Copyright 2025 NVIDIA CORPORATION
SPDX-License-Identifier: Apache-2.0
*/
package subgroups

import (
	"context"
	"time"

	batchv1 "k8s.io/api/batch/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	v2 "github.com/NVIDIA/KAI-scheduler/pkg/apis/scheduling/v2"
	commonconsts "github.com/NVIDIA/KAI-scheduler/pkg/common/constants"
	pluginconstants "github.com/NVIDIA/KAI-scheduler/pkg/podgrouper/podgrouper/plugins/constants"
	testcontext "github.com/NVIDIA/KAI-scheduler/test/e2e/modules/context"
	"github.com/NVIDIA/KAI-scheduler/test/e2e/modules/resources/capacity"
	"github.com/NVIDIA/KAI-scheduler/test/e2e/modules/resources/rd"
	"github.com/NVIDIA/KAI-scheduler/test/e2e/modules/resources/rd/queue"
	"github.com/NVIDIA/KAI-scheduler/test/e2e/modules/utils"
	"github.com/NVIDIA/KAI-scheduler/test/e2e/modules/wait"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("User-requested subgroups", Ordered, func() {
	var (
		testCtx *testcontext.TestContext
	)

	BeforeAll(func(ctx context.Context) {
		testCtx = testcontext.GetConnectivity(ctx, Default)

		parentQueue := queue.CreateQueueObject(utils.GenerateRandomK8sName(10), "")
		childQueue := queue.CreateQueueObject(utils.GenerateRandomK8sName(10), parentQueue.Name)
		childQueue.Spec.Resources.CPU.Quota = 1000
		childQueue.Spec.Resources.CPU.Limit = 1000
		testCtx.InitQueues([]*v2.Queue{childQueue, parentQueue})

		capacity.SkipIfInsufficientClusterTopologyResources(testCtx.KubeClientset, []capacity.ResourceList{
			{
				Cpu:      resource.MustParse("500m"),
				PodCount: 5,
			},
		})
	})

	AfterAll(func(ctx context.Context) {
		err := rd.DeleteAllE2EPriorityClasses(ctx, testCtx.ControllerClient)
		Expect(err).To(Succeed())
		testCtx.ClusterCleanup(ctx)
	})

	AfterEach(func(ctx context.Context) {
		testCtx.TestContextCleanup(ctx)
	})

	It("Creates subgroups when TopOwner has create-subgroup annotation", func(ctx context.Context) {
		namespace := queue.GetConnectedNamespaceToQueue(testCtx.Queues[0])

		// Create a Job with the create-subgroup annotation
		job := createJobWithCreateSubgroupAnnotation(testCtx.Queues[0], "auth-proxy", 2)

		createdJob, err := testCtx.KubeClientset.BatchV1().Jobs(namespace).Create(ctx, job, metav1.CreateOptions{})
		Expect(err).To(Succeed())

		// Wait for pods to be created
		wait.ForAtLeastNPodCreation(ctx, testCtx.ControllerClient, metav1.LabelSelector{
			MatchLabels: map[string]string{
				"job-name": createdJob.Name,
			},
		}, 2)

		// Wait for PodGroup to be created and verify subgroups
		Eventually(func(g Gomega) {
			podGroups, err := testCtx.KubeAiSchedClientset.SchedulingV2alpha2().PodGroups(namespace).List(ctx, metav1.ListOptions{})
			g.Expect(err).To(Succeed())

			// Find the PodGroup for this job
			var foundPG bool
			for _, pg := range podGroups.Items {
				if pg.OwnerReferences != nil {
					for _, ref := range pg.OwnerReferences {
						if ref.UID == createdJob.UID {
							foundPG = true
							// Verify subgroups
							g.Expect(pg.Spec.SubGroups).To(HaveLen(2))

							subGroupNames := make(map[string]int32)
							for _, sg := range pg.Spec.SubGroups {
								subGroupNames[sg.Name] = sg.MinMember
							}

							g.Expect(subGroupNames).To(HaveKey("default"))
							g.Expect(subGroupNames).To(HaveKey("auth-proxy"))
							g.Expect(subGroupNames["auth-proxy"]).To(Equal(int32(1)))
							// MinMember should be original (2) + 1 for auth-proxy = 3
							g.Expect(pg.Spec.MinMember).To(Equal(int32(3)))
							break
						}
					}
				}
			}
			g.Expect(foundPG).To(BeTrue(), "PodGroup for job not found")
		}, 30*time.Second, 1*time.Second).Should(Succeed())
	})

	It("Labels pods with requested-subgroup annotation correctly", func(ctx context.Context) {
		namespace := queue.GetConnectedNamespaceToQueue(testCtx.Queues[0])

		// Create a Job with the create-subgroup annotation
		job := createJobWithCreateSubgroupAnnotation(testCtx.Queues[0], "sidecar", 1)

		createdJob, err := testCtx.KubeClientset.BatchV1().Jobs(namespace).Create(ctx, job, metav1.CreateOptions{})
		Expect(err).To(Succeed())

		// Wait for the Job's pod to be created
		wait.ForAtLeastNPodCreation(ctx, testCtx.ControllerClient, metav1.LabelSelector{
			MatchLabels: map[string]string{
				"job-name": createdJob.Name,
			},
		}, 1)

		// Wait for PodGroup to be created
		var pgName string
		Eventually(func(g Gomega) {
			podGroups, err := testCtx.KubeAiSchedClientset.SchedulingV2alpha2().PodGroups(namespace).List(ctx, metav1.ListOptions{})
			g.Expect(err).To(Succeed())

			for _, pg := range podGroups.Items {
				if pg.OwnerReferences != nil {
					for _, ref := range pg.OwnerReferences {
						if ref.UID == createdJob.UID {
							pgName = pg.Name
							return
						}
					}
				}
			}
			g.Expect(pgName).NotTo(BeEmpty(), "PodGroup for job not found")
		}, 30*time.Second, 1*time.Second).Should(Succeed())

		// Create a pod that references the same PodGroup with requested-subgroup annotation
		pod := createPodWithRequestedSubgroup(testCtx.Queues[0], pgName, "sidecar")
		createdPod, err := rd.CreatePod(ctx, testCtx.KubeClientset, pod)
		Expect(err).To(Succeed())

		// Wait for pod to be labeled with the subgroup name
		Eventually(func(g Gomega) {
			updatedPod, err := testCtx.KubeClientset.CoreV1().Pods(namespace).Get(ctx, createdPod.Name, metav1.GetOptions{})
			g.Expect(err).To(Succeed())
			g.Expect(updatedPod.Labels).To(HaveKeyWithValue(commonconsts.SubGroupLabelKey, "sidecar"))
		}, 30*time.Second, 1*time.Second).Should(Succeed())
	})

	It("Gang scheduling respects subgroup minMember requirements", func(ctx context.Context) {
		namespace := queue.GetConnectedNamespaceToQueue(testCtx.Queues[0])

		// Create a Job with the create-subgroup annotation with parallelism=2
		job := createJobWithCreateSubgroupAnnotation(testCtx.Queues[0], "auth-proxy", 2)

		createdJob, err := testCtx.KubeClientset.BatchV1().Jobs(namespace).Create(ctx, job, metav1.CreateOptions{})
		Expect(err).To(Succeed())

		// Wait for the Job's pods to be created
		wait.ForAtLeastNPodCreation(ctx, testCtx.ControllerClient, metav1.LabelSelector{
			MatchLabels: map[string]string{
				"job-name": createdJob.Name,
			},
		}, 2)

		// Wait for PodGroup to be created
		var pgName string
		Eventually(func(g Gomega) {
			podGroups, err := testCtx.KubeAiSchedClientset.SchedulingV2alpha2().PodGroups(namespace).List(ctx, metav1.ListOptions{})
			g.Expect(err).To(Succeed())

			for _, pg := range podGroups.Items {
				if pg.OwnerReferences != nil {
					for _, ref := range pg.OwnerReferences {
						if ref.UID == createdJob.UID {
							pgName = pg.Name
							return
						}
					}
				}
			}
			g.Expect(pgName).NotTo(BeEmpty(), "PodGroup for job not found")
		}, 30*time.Second, 1*time.Second).Should(Succeed())

		// Create a pod for the auth-proxy subgroup
		sidecarPod := createPodWithRequestedSubgroup(testCtx.Queues[0], pgName, "auth-proxy")
		_, err = rd.CreatePod(ctx, testCtx.KubeClientset, sidecarPod)
		Expect(err).To(Succeed())

		// Get all pods and verify they get scheduled
		// The gang should be satisfied: default subgroup needs 2, auth-proxy needs 1
		// Total minMember = 3, we have 3 pods (2 from job + 1 sidecar)
		allPods := []*v1.Pod{sidecarPod}
		jobPods := rd.GetJobPods(ctx, testCtx.KubeClientset, createdJob)
		for i := range jobPods {
			allPods = append(allPods, &jobPods[i])
		}

		// All pods should be scheduled since gang requirements are met
		wait.ForAtLeastNPodsScheduled(ctx, testCtx.ControllerClient, namespace, allPods, 3)
	})
})

func createJobWithCreateSubgroupAnnotation(podQueue *v2.Queue, subgroupName string, parallelism int32) *batchv1.Job {
	namespace := queue.GetConnectedNamespaceToQueue(podQueue)
	matchLabelValue := utils.GenerateRandomK8sName(10)

	return &batchv1.Job{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "batch/v1",
			Kind:       "Job",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      utils.GenerateRandomK8sName(10),
			Namespace: namespace,
			Labels: map[string]string{
				commonconsts.AppLabelName: "engine-e2e",
				rd.BatchJobAppLabel:       matchLabelValue,
			},
			Annotations: map[string]string{
				pluginconstants.CreateSubgroupAnnotationKey: subgroupName,
			},
		},
		Spec: batchv1.JobSpec{
			Parallelism: ptr.To(parallelism),
			Completions: ptr.To(parallelism),
			Template: v1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						commonconsts.AppLabelName: "engine-e2e",
						rd.BatchJobAppLabel:       matchLabelValue,
						"kai.scheduler/queue":     podQueue.Name,
					},
				},
				Spec: v1.PodSpec{
					Containers: []v1.Container{
						{
							Image: "ubuntu",
							Name:  "ubuntu-container",
							Args: []string{
								"sleep",
								"infinity",
							},
							Resources: v1.ResourceRequirements{
								Limits: map[v1.ResourceName]resource.Quantity{
									v1.ResourceCPU: resource.MustParse("100m"),
								},
							},
							SecurityContext: rd.DefaultSecurityContext(),
							ImagePullPolicy: v1.PullIfNotPresent,
						},
					},
					TerminationGracePeriodSeconds: ptr.To(int64(0)),
					SchedulerName:                 "kai-scheduler",
					RestartPolicy:                 v1.RestartPolicyNever,
					Tolerations: []v1.Toleration{
						{
							Key:      "nvidia.com/gpu",
							Operator: v1.TolerationOpExists,
							Effect:   v1.TaintEffectNoSchedule,
						},
					},
				},
			},
		},
	}
}

func createPodWithRequestedSubgroup(podQueue *v2.Queue, podGroupName string, subgroupName string) *v1.Pod {
	pod := rd.CreatePodWithPodGroupReference(podQueue, podGroupName, v1.ResourceRequirements{
		Limits: map[v1.ResourceName]resource.Quantity{
			v1.ResourceCPU: resource.MustParse("100m"),
		},
	})
	pod.Annotations[pluginconstants.RequestedSubgroupAnnotationKey] = subgroupName
	return pod
}
