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
	"github.com/NVIDIA/KAI-scheduler/pkg/apis/scheduling/v2alpha2"
	"github.com/NVIDIA/KAI-scheduler/pkg/common/constants"
	"github.com/NVIDIA/KAI-scheduler/test/e2e/modules/constant"
	testcontext "github.com/NVIDIA/KAI-scheduler/test/e2e/modules/context"
	"github.com/NVIDIA/KAI-scheduler/test/e2e/modules/resources/rd/queue"
	"github.com/NVIDIA/KAI-scheduler/test/e2e/modules/utils"
	"github.com/NVIDIA/KAI-scheduler/test/e2e/modules/wait"
	runtimeClient "sigs.k8s.io/controller-runtime/pkg/client"
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

	Context("JobSet PodGroup creation scenarios", func() {
		It("should create PodGroup with MinMember=8 for single ReplicatedJob with high parallelism", func(ctx context.Context) {
			testNamespace := queue.GetConnectedNamespaceToQueue(testCtx.Queues[0])
			jobSetName := "high-parallelism-" + utils.GenerateRandomK8sName(10)

			jobSet := createJobSetWithHighParallelism(jobSetName, testNamespace, testCtx.Queues[0].Name)
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

				// Should have 1 PodGroup (default InOrder creates one per replicatedJob)
				g.Expect(len(podGroups.Items)).To(Equal(1))
				// MinMember should be replicas * parallelism = 1 * 8 = 8
				g.Expect(podGroups.Items[0].Spec.MinMember).To(Equal(int32(8)))
			}, time.Minute).Should(Succeed())
		})

		It("should create separate PodGroups for multiple ReplicatedJobs with different parallelism", func(ctx context.Context) {
			testNamespace := queue.GetConnectedNamespaceToQueue(testCtx.Queues[0])
			jobSetName := "multi-parallelism-" + utils.GenerateRandomK8sName(10)

			jobSet := createJobSetWithMultipleReplicatedJobs(jobSetName, testNamespace, testCtx.Queues[0].Name)
			Expect(testCtx.ControllerClient.Create(ctx, jobSet)).To(Succeed())
			defer func() {
				Expect(testCtx.ControllerClient.Delete(ctx, jobSet)).To(Succeed())
			}()

			// Wait for pods to be created (coordinator: 2 pods, worker: 8 pods)
			pods := waitForJobSetPods(ctx, testCtx, jobSetName, testNamespace, 10)
			wait.ForPodsScheduled(ctx, testCtx.ControllerClient, testNamespace, pods)

			// Verify PodGroups are created separately (default InOrder)
			Eventually(func(g Gomega) {
				podGroups := &v2alpha2.PodGroupList{}
				err := testCtx.ControllerClient.List(ctx, podGroups, runtimeClient.InNamespace(testNamespace))
				g.Expect(err).To(Succeed())

				// Should have 2 PodGroups (one per replicatedJob)
				g.Expect(len(podGroups.Items)).To(Equal(2))

				// Find PodGroups by name pattern and verify MinMember
				coordinatorPG := findPodGroupByName(podGroups.Items, "coordinator")
				workerPG := findPodGroupByName(podGroups.Items, "worker")
				g.Expect(coordinatorPG).NotTo(BeNil())
				g.Expect(workerPG).NotTo(BeNil())
				// Coordinator: replicas=1, parallelism=2 => MinMember=2
				g.Expect(coordinatorPG.Spec.MinMember).To(Equal(int32(2)))
				// Worker: replicas=2, parallelism=4 => MinMember=8
				g.Expect(workerPG.Spec.MinMember).To(Equal(int32(8)))
			}, time.Minute).Should(Succeed())
		})

		It("should create PodGroup with MinMember=1 for single replica with default parallelism", func(ctx context.Context) {
			testNamespace := queue.GetConnectedNamespaceToQueue(testCtx.Queues[0])
			jobSetName := "default-parallelism-" + utils.GenerateRandomK8sName(10)

			jobSet := createJobSetWithDefaultParallelism(jobSetName, testNamespace, testCtx.Queues[0].Name)
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
				// MinMember should default to 1 (replicas=1, parallelism defaults to 1)
				g.Expect(podGroups.Items[0].Spec.MinMember).To(Equal(int32(1)))
			}, time.Minute).Should(Succeed())
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

// createJobSetWithHighParallelism creates a JobSet with a single ReplicatedJob (parallelism=8).
func createJobSetWithHighParallelism(name, namespace, queueName string) *unstructured.Unstructured {
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
				"successPolicy": map[string]interface{}{
					"operator": "All",
				},
				"failurePolicy": map[string]interface{}{
					"maxRestarts": int64(3),
				},
				"replicatedJobs": []interface{}{
					map[string]interface{}{
						"name":     "worker",
						"replicas": int64(1),
						"template": map[string]interface{}{
							"spec": map[string]interface{}{
								"parallelism":  ptr.To(int64(8)),
								"completions":  ptr.To(int64(8)),
								"backoffLimit": ptr.To(int32(0)),
								"template": map[string]interface{}{
									"spec": map[string]interface{}{
										"restartPolicy": "Never",
										"schedulerName": constant.SchedulerName,
										"containers": []interface{}{
											map[string]interface{}{
												"name":  "worker",
												"image": "ubuntu",
												"command": []interface{}{
													"sleep",
													"3600",
												},
												"resources": map[string]interface{}{
													"requests": map[string]interface{}{
														"nvidia.com/gpu": "1",
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

// createJobSetWithMultipleReplicatedJobs creates a JobSet with multiple ReplicatedJobs.
func createJobSetWithMultipleReplicatedJobs(name, namespace, queueName string) *unstructured.Unstructured {
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
				"successPolicy": map[string]interface{}{
					"operator": "All",
				},
				"failurePolicy": map[string]interface{}{
					"maxRestarts": int64(3),
				},
				"replicatedJobs": []interface{}{
					map[string]interface{}{
						"name":     "coordinator",
						"replicas": int64(1),
						"template": map[string]interface{}{
							"spec": map[string]interface{}{
								"parallelism":  ptr.To(int64(2)),
								"completions":  ptr.To(int64(2)),
								"backoffLimit": ptr.To(int32(0)),
								"template": map[string]interface{}{
									"spec": map[string]interface{}{
										"restartPolicy": "Never",
										"schedulerName": constant.SchedulerName,
										"containers": []interface{}{
											map[string]interface{}{
												"name":  "coordinator",
												"image": "ubuntu",
												"command": []interface{}{
													"sleep",
													"3600",
												},
											},
										},
									},
								},
							},
						},
					},
					map[string]interface{}{
						"name":     "worker",
						"replicas": int64(2),
						"template": map[string]interface{}{
							"spec": map[string]interface{}{
								"parallelism":  ptr.To(int64(4)),
								"completions":  ptr.To(int64(4)),
								"backoffLimit": ptr.To(int32(0)),
								"template": map[string]interface{}{
									"spec": map[string]interface{}{
										"restartPolicy": "Never",
										"schedulerName": constant.SchedulerName,
										"containers": []interface{}{
											map[string]interface{}{
												"name":  "worker",
												"image": "ubuntu",
												"command": []interface{}{
													"sleep",
													"3600",
												},
												"resources": map[string]interface{}{
													"requests": map[string]interface{}{
														"nvidia.com/gpu": "1",
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

// createJobSetWithDefaultParallelism creates a JobSet with single replica and default parallelism.
func createJobSetWithDefaultParallelism(name, namespace, queueName string) *unstructured.Unstructured {
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
				"successPolicy": map[string]interface{}{
					"operator": "All",
				},
				"failurePolicy": map[string]interface{}{
					"maxRestarts": int64(3),
				},
				"replicatedJobs": []interface{}{
					map[string]interface{}{
						"name":     "single",
						"replicas": int64(1),
						"template": map[string]interface{}{
							"spec": map[string]interface{}{
								// No parallelism specified, should default to 1
								"completions":  ptr.To(int64(1)),
								"backoffLimit": ptr.To(int32(0)),
								"template": map[string]interface{}{
									"spec": map[string]interface{}{
										"restartPolicy": "Never",
										"schedulerName": constant.SchedulerName,
										"containers": []interface{}{
											map[string]interface{}{
												"name":  "single",
												"image": "ubuntu",
												"command": []interface{}{
													"sleep",
													"3600",
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

// findPodGroupByName finds a PodGroup by replicatedJob name suffix.
func findPodGroupByName(podGroups []v2alpha2.PodGroup, replicatedJobName string) *v2alpha2.PodGroup {
	for i := range podGroups {
		if podGroups[i].Name == "" {
			continue
		}
		// Extract the last part after the last dash
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

