/*
Copyright 2017 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package cache

import (
	"context"
	"fmt"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/fake"
	faketesting "k8s.io/client-go/testing"
	"k8s.io/utils/ptr"

	kubeaischedulerfake "github.com/NVIDIA/KAI-scheduler/pkg/apis/client/clientset/versioned/fake"
	fakeschedulingv1alpha2 "github.com/NVIDIA/KAI-scheduler/pkg/apis/client/clientset/versioned/typed/scheduling/v1alpha2/fake"
	schedulingv1alpha2 "github.com/NVIDIA/KAI-scheduler/pkg/apis/scheduling/v1alpha2"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/pod_info"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/conf"
	kueuefake "sigs.k8s.io/kueue/client-go/clientset/versioned/fake"
)

func TestCache(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Test cache")
}

var _ = Describe("Cache", func() {
	Describe("Bind", func() {
		Context("failure to bind", func() {
			It("should return error", func() {
				objects := []runtime.Object{
					&v1.Node{
						ObjectMeta: metav1.ObjectMeta{
							Name: "node-1",
						},
					},
				}
				cache, stopCh := setupCacheWithObjects(true, objects, &schedulingv1alpha2.BindRequest{})
				defer close(stopCh)

				cache.(*SchedulerCache).kubeAiSchedulerClient.SchedulingV1alpha2().(*fakeschedulingv1alpha2.FakeSchedulingV1alpha2).PrependReactor(
					"create", "bindrequests",
					func(action faketesting.Action) (handled bool, ret runtime.Object, err error) {
						return true, nil, fmt.Errorf("failed to create bind request")
					},
				)

				pod := &v1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "pod-1",
						Namespace: "namespace-1",
						UID:       types.UID("pod-uid"),
					},
					Spec: v1.PodSpec{
						Containers: []v1.Container{
							{
								Name: "container-1",
								Resources: v1.ResourceRequirements{
									Requests: v1.ResourceList{
										"cpu":            resource.MustParse("2000m"),
										"memory":         resource.MustParse("5Gi"),
										"nvidia.com/gpu": resource.MustParse("2"),
									},
								},
							},
						},
					},
					Status: v1.PodStatus{
						Phase: v1.PodPending,
					},
				}

				taskInfo := pod_info.NewTaskInfo(pod)

				err := cache.Bind(taskInfo, "node-1", map[string]string{})
				Expect(err).To(HaveOccurred())
			})
		})
	})

	Describe("Stale BindRequests Cleanup", func() {
		It("Delete a single stale bind request",
			func() {
				cache, stopCh := setupCacheWithObjects(true, []runtime.Object{}, &schedulingv1alpha2.BindRequest{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "bind-request-1",
						Namespace: "namespace-1",
					},
					Spec: schedulingv1alpha2.BindRequestSpec{
						PodName:      "pod-1",
						SelectedNode: "node-1",
						BackoffLimit: ptr.To(int32(1)),
					},
					Status: schedulingv1alpha2.BindRequestStatus{
						Phase:          schedulingv1alpha2.BindRequestPhaseFailed,
						FailedAttempts: 1,
					},
				})
				defer close(stopCh)

				kubeAiSchedulerClient := cache.(*SchedulerCache).kubeAiSchedulerClient
				bindRequestsAfterCleanup, err := kubeAiSchedulerClient.SchedulingV1alpha2().BindRequests("namespace-1").List(context.TODO(), metav1.ListOptions{})
				Expect(err).NotTo(HaveOccurred())
				Expect(bindRequestsAfterCleanup.Items).To(HaveLen(0))
			},
		)

		It("Delete single stale bind and leaves on", func() {
			cache, stopCh := setupCacheWithObjects(
				true,
				[]runtime.Object{
					&v1.Node{
						ObjectMeta: metav1.ObjectMeta{
							Name: "node-1",
						},
					},
					&v1.Pod{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "pod-1",
							Namespace: "namespace-1",
						},
						Status: v1.PodStatus{
							Phase: v1.PodPending,
						},
					},
				},
				&schedulingv1alpha2.BindRequest{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "bind-request-1",
						Namespace: "namespace-1",
					},
					Spec: schedulingv1alpha2.BindRequestSpec{
						PodName:      "pod-1",
						SelectedNode: "node-1",
					},
				},
				&schedulingv1alpha2.BindRequest{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "bind-request-2",
						Namespace: "namespace-1",
					},
					Spec: schedulingv1alpha2.BindRequestSpec{
						PodName:      "pod-2",
						SelectedNode: "node-2",
						BackoffLimit: ptr.To(int32(1)),
					},
					Status: schedulingv1alpha2.BindRequestStatus{
						Phase:          schedulingv1alpha2.BindRequestPhaseFailed,
						FailedAttempts: 1,
					},
				},
			)
			defer close(stopCh)

			kubeAiSchedulerClient := cache.(*SchedulerCache).kubeAiSchedulerClient
			bindRequestsAfterCleanup, err := kubeAiSchedulerClient.SchedulingV1alpha2().BindRequests("namespace-1").List(context.TODO(), metav1.ListOptions{})
			Expect(err).NotTo(HaveOccurred())
			Expect(bindRequestsAfterCleanup.Items).To(HaveLen(1))
		})

		It("Reports all failed deletions", func() {
			cache, stopCh := setupCacheWithObjects(
				false,
				[]runtime.Object{},
				&schedulingv1alpha2.BindRequest{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "bind-request-1",
						Namespace: "namespace-1",
					},
					Spec: schedulingv1alpha2.BindRequestSpec{
						PodName:      "pod-1",
						SelectedNode: "node-1",
						BackoffLimit: ptr.To(int32(1)),
					},
					Status: schedulingv1alpha2.BindRequestStatus{
						Phase:          schedulingv1alpha2.BindRequestPhaseFailed,
						FailedAttempts: 1,
					},
				},
				&schedulingv1alpha2.BindRequest{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "bind-request-2",
						Namespace: "namespace-1",
					},
					Spec: schedulingv1alpha2.BindRequestSpec{
						PodName:      "pod-2",
						SelectedNode: "node-2",
						BackoffLimit: ptr.To(int32(1)),
					},
					Status: schedulingv1alpha2.BindRequestStatus{
						Phase:          schedulingv1alpha2.BindRequestPhaseFailed,
						FailedAttempts: 1,
					},
				},
			)
			defer close(stopCh)

			cache.(*SchedulerCache).kubeAiSchedulerClient.SchedulingV1alpha2().(*fakeschedulingv1alpha2.FakeSchedulingV1alpha2).PrependReactor(
				"delete", "bindrequests",
				func(action faketesting.Action) (handled bool, ret runtime.Object, err error) {
					return true, nil, fmt.Errorf("failed to delete bind request")
				},
			)

			_, err := cache.Snapshot()
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("bind-request-1"))
			Expect(err.Error()).To(ContainSubstring("bind-request-2"))

			kubeAiSchedulerClient := cache.(*SchedulerCache).kubeAiSchedulerClient
			bindRequestsAfterCleanup, err := kubeAiSchedulerClient.SchedulingV1alpha2().BindRequests("namespace-1").List(context.TODO(), metav1.ListOptions{})
			Expect(err).NotTo(HaveOccurred())
			Expect(bindRequestsAfterCleanup.Items).To(HaveLen(2))
		})

	})
})

func setupCacheWithObjects(snapshot bool, objects []runtime.Object, kaiSchedulerObjects ...runtime.Object) (Cache, chan struct{}) {
	kubeClient := fake.NewSimpleClientset(objects...)
	kubeAiSchedulerClient := kubeaischedulerfake.NewSimpleClientset(kaiSchedulerObjects...)
	kueueClient := kueuefake.NewSimpleClientset()

	cache := New(&SchedulerCacheParams{
		KubeClient:            kubeClient,
		KAISchedulerClient:    kubeAiSchedulerClient,
		KueueClient:           kueueClient,
		NodePoolParams:        &conf.SchedulingNodePoolParams{},
		FullHierarchyFairness: true,
	})

	stopCh := make(chan struct{})
	cache.Run(stopCh)
	cache.WaitForCacheSync(stopCh)

	if snapshot {
		_, err := cache.Snapshot()
		Expect(err).NotTo(HaveOccurred())
	}

	return cache, stopCh
}
