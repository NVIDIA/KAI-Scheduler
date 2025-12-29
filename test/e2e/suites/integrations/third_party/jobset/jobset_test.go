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
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v2 "github.com/NVIDIA/KAI-scheduler/pkg/apis/scheduling/v2"
	"github.com/NVIDIA/KAI-scheduler/pkg/common/constants"
	"github.com/NVIDIA/KAI-scheduler/test/e2e/modules/constant"
	testcontext "github.com/NVIDIA/KAI-scheduler/test/e2e/modules/context"
	"github.com/NVIDIA/KAI-scheduler/test/e2e/modules/resources/rd/queue"
	"github.com/NVIDIA/KAI-scheduler/test/e2e/modules/utils"
	"github.com/NVIDIA/KAI-scheduler/test/e2e/modules/wait"
)

var (
	jobSetGVK = schema.GroupVersionKind{
		Group:   "jobset.x-k8s.io",
		Version: "v1alpha2",
		Kind:    "JobSet",
	}
)

var _ = Describe("JobSet integration", Ordered, func() {
	var (
		testCtx *testcontext.TestContext
	)

	BeforeAll(func(ctx context.Context) {
		testCtx = testcontext.GetConnectivity(ctx, Gomega)
		parentQueue := queue.CreateQueueObject(utils.GenerateRandomK8sName(10), "")
		childQueue := queue.CreateQueueObject(utils.GenerateRandomK8sName(10), parentQueue.Name)
		testCtx.InitQueues([]*v2.Queue{childQueue, parentQueue})

		skipIfJobSetNotInstalled(ctx, testCtx)
	})

	AfterEach(func(ctx context.Context) {
		testCtx.TestContextCleanup(ctx)
	})

	AfterAll(func(ctx context.Context) {
		testCtx.ClusterCleanup(ctx)
	})

	Context("JobSet submission", func() {
		It("should schedule pods with parallelism=2 and completions=3, first pod completes 5 seconds earlier", func(ctx context.Context) {
			testNamespace := queue.GetConnectedNamespaceToQueue(testCtx.Queues[0])
			jobSetName := "test-jobset-" + utils.GenerateRandomK8sName(10)

			jobSet := createJobSet(jobSetName, testNamespace, testCtx.Queues[0].Name, 2, 3)
			Expect(testCtx.ControllerClient.Create(ctx, jobSet)).To(Succeed())
			defer func() {
				Expect(testCtx.ControllerClient.Delete(ctx, jobSet)).To(Succeed())
			}()

			// Wait for pods to be created
			pods := waitForJobSetPods(ctx, testCtx, jobSetName, testNamespace, 3)

			// Wait for all pods to be scheduled
			wait.ForPodsScheduled(ctx, testCtx.ControllerClient, testNamespace, pods)

			// Wait for first pod to complete (should complete 5 seconds earlier)
			firstPod := pods[0]
			Eventually(func() v1.PodPhase {
				err := testCtx.ControllerClient.Get(ctx, client.ObjectKeyFromObject(firstPod), firstPod)
				if err != nil {
					return v1.PodUnknown
				}
				return firstPod.Status.Phase
			}, 30*time.Second, 1*time.Second).Should(Equal(v1.PodSucceeded))

			// Wait for remaining pods to complete
			for i := 1; i < len(pods); i++ {
				Eventually(func() v1.PodPhase {
					err := testCtx.ControllerClient.Get(ctx, client.ObjectKeyFromObject(pods[i]), pods[i])
					if err != nil {
						return v1.PodUnknown
					}
					return pods[i].Status.Phase
				}, 30*time.Second, 1*time.Second).Should(Equal(v1.PodSucceeded))
			}
		})

		It("should create separate PodGroups per replicatedJob when startupPolicyOrder is InOrder", func(ctx context.Context) {
			testNamespace := queue.GetConnectedNamespaceToQueue(testCtx.Queues[0])
			jobSetName := "inorder-jobset-" + utils.GenerateRandomK8sName(10)

			jobSet := createJobSetWithStartupPolicy(jobSetName, testNamespace, testCtx.Queues[0].Name, "InOrder")
			Expect(testCtx.ControllerClient.Create(ctx, jobSet)).To(Succeed())
			defer func() {
				Expect(testCtx.ControllerClient.Delete(ctx, jobSet)).To(Succeed())
			}()

			// Wait for pods and verify PodGroups are created separately
			pods := waitForJobSetPods(ctx, testCtx, jobSetName, testNamespace, 2)
			wait.ForPodsScheduled(ctx, testCtx.ControllerClient, testNamespace, pods)
		})

		It("should create a single PodGroup for all replicatedJobs when startupPolicyOrder is not InOrder", func(ctx context.Context) {
			testNamespace := queue.GetConnectedNamespaceToQueue(testCtx.Queues[0])
			jobSetName := "anyorder-jobset-" + utils.GenerateRandomK8sName(10)

			jobSet := createJobSetWithStartupPolicy(jobSetName, testNamespace, testCtx.Queues[0].Name, "AnyOrder")
			Expect(testCtx.ControllerClient.Create(ctx, jobSet)).To(Succeed())
			defer func() {
				Expect(testCtx.ControllerClient.Delete(ctx, jobSet)).To(Succeed())
			}()

			// Wait for pods and verify they all use the same PodGroup
			pods := waitForJobSetPods(ctx, testCtx, jobSetName, testNamespace, 2)
			wait.ForPodsScheduled(ctx, testCtx.ControllerClient, testNamespace, pods)
		})
	})
})

func skipIfJobSetNotInstalled(ctx context.Context, testCtx *testcontext.TestContext) {
	crdList, err := testCtx.KubeClientset.ApiextensionsV1().CustomResourceDefinitions().List(ctx, metav1.ListOptions{
		FieldSelector: "metadata.name=jobsets.jobset.x-k8s.io",
	})
	Expect(err).NotTo(HaveOccurred())
	if len(crdList.Items) == 0 {
		Skip("JobSet CRD not installed, skipping JobSet integration tests")
	}
}

func createJobSet(name, namespace, queueName string, parallelism, completions int32) *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "jobset.x-k8s.io/v1alpha2",
			"kind":       "JobSet",
			"metadata": map[string]interface{}{
				"name":      name,
				"namespace": namespace,
				"labels": map[string]interface{}{
					constants.AppLabelName: "engine-e2e",
					"kai.scheduler/queue":   queueName,
				},
			},
			"spec": map[string]interface{}{
				"replicatedJobs": []interface{}{
					map[string]interface{}{
						"name":     "worker",
						"replicas": int64(1),
						"template": map[string]interface{}{
							"spec": map[string]interface{}{
								"parallelism":  ptr.To(int64(parallelism)),
								"completions":  ptr.To(int64(completions)),
								"backoffLimit": ptr.To(int32(0)),
								"template": map[string]interface{}{
									"spec": map[string]interface{}{
										"restartPolicy": "Never",
										"schedulerName": constant.SchedulerName,
										"containers": []interface{}{
											map[string]interface{}{
												"name":  "worker",
												"image": "busybox:1.36",
												"command": []interface{}{
													"/bin/sh",
													"-c",
													// First pod sleeps 5 seconds, others sleep 10 seconds
													"if [ \"$JOB_COMPLETION_INDEX\" = \"0\" ]; then sleep 5; else sleep 10; fi && echo 'Job completed'",
												},
												"env": []interface{}{
													map[string]interface{}{
														"name":  "JOB_COMPLETION_INDEX",
														"valueFrom": map[string]interface{}{
															"fieldRef": map[string]interface{}{
																"fieldPath": "metadata.annotations['batch.kubernetes.io/job-completion-index']",
															},
														},
													},
												},
												"resources": map[string]interface{}{
													"requests": map[string]interface{}{
														"cpu":    resource.MustParse("100m").String(),
														"memory": resource.MustParse("128Mi").String(),
													},
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}
}

func createJobSetWithStartupPolicy(name, namespace, queueName, startupPolicyOrder string) *unstructured.Unstructured {
	jobSet := createJobSet(name, namespace, queueName, 1, 1)
	spec := jobSet.Object["spec"].(map[string]interface{})
	spec["startupPolicy"] = map[string]interface{}{
		"startupPolicyOrder": startupPolicyOrder,
	}
	return jobSet
}

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
		if len(podList.Items) < expectedCount {
			return fmt.Errorf("expected %d pods, got %d", expectedCount, len(podList.Items))
		}
		pods = make([]*v1.Pod, len(podList.Items))
		for i := range podList.Items {
			pods[i] = &podList.Items[i]
		}
		return nil
	}, 60*time.Second, 2*time.Second).Should(Succeed())
	return pods
}

