// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package resourcereservation

import (
	"context"
	"fmt"
	"testing"
	"time"

	runtimeClient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/NVIDIA/KAI-scheduler/pkg/common/constants"
)

func TestResourceReservation(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "resource reservation")
}

func nodeNameIndexer(rawObj runtimeClient.Object) []string {
	pod := rawObj.(*v1.Pod)
	return []string{pod.Spec.NodeName}
}

func initializeTestService(
	client runtimeClient.WithWatch,
) *service {
	service := NewService(false, client, "", 40*time.Millisecond)

	return service
}

var _ = Describe("ResourceReservationService", func() {
	const (
		existingGroup     = "bla-bla"
		nodeName          = "node-1"
		gpuGroup          = "gpu-group"
		gpuGroup2         = "gpu-group-2"
		failedToCreatePod = "failed to create reservation pod"
	)
	var (
		groupLabels = map[string]string{
			constants.GPUGroup: gpuGroup,
		}
		runningStatus = v1.PodStatus{
			Phase: v1.PodRunning,
		}
		exampleReservationPod = &v1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "example",
				Namespace: namespace,
				Labels:    groupLabels,
			},
			Spec: v1.PodSpec{
				NodeName: nodeName,
			},
			Status: runningStatus,
		}
		exampleRunningJob = &v1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "job-1-0-0",
				Namespace: "my-ns",
				Labels:    groupLabels,
			},
			Spec: v1.PodSpec{
				NodeName: nodeName,
			},
			Status: runningStatus,
		}
	)
	Context("ReserveGpuDevice", func() {
		for testName, testData := range map[string]struct {
			reservationPod        *v1.Pod
			groupName             string
			clientInterceptFuncs  interceptor.Funcs
			numReservationPods    int
			expectedGPUIndex      string
			expectedErrorContains string
		}{
			"reservation pod exists": {
				groupName: existingGroup,
				reservationPod: &v1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: namespace,
						Annotations: map[string]string{
							gpuIndexAnnotationName: "4",
						},
						Labels: map[string]string{
							constants.GPUGroup: existingGroup,
						},
					},
				},
				numReservationPods: 1,
				expectedGPUIndex:   "4",
			},
			"reservation pod missing gpu index annotation": {
				groupName: existingGroup,
				reservationPod: &v1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Namespace:   namespace,
						Annotations: map[string]string{},
						Labels: map[string]string{
							constants.GPUGroup: existingGroup,
						},
					},
				},
				numReservationPods:    1,
				expectedGPUIndex:      unknownGpuIndicator,
				expectedErrorContains: "annotation",
			},
			"error listing pods": {
				groupName: existingGroup,
				clientInterceptFuncs: interceptor.Funcs{
					List: func(ctx context.Context, client runtimeClient.WithWatch, list runtimeClient.ObjectList, opts ...runtimeClient.ListOption) error {
						return fmt.Errorf("failed to list pods")
					},
				},
				expectedGPUIndex:      unknownGpuIndicator,
				expectedErrorContains: "failed to list pods",
			},
			"reservation pod exists only for other group - create a new one": {
				groupName: "other-group",
				reservationPod: &v1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: namespace,
						Annotations: map[string]string{
							gpuIndexAnnotationName: "4",
						},
						Labels: map[string]string{
							constants.GPUGroup: "some-other-group",
						},
					},
				},
				clientInterceptFuncs: interceptor.Funcs{
					Watch: func(ctx context.Context, client runtimeClient.WithWatch, obj runtimeClient.ObjectList, opts ...runtimeClient.ListOption) (watch.Interface, error) {
						return exampleMockWatchPod("3", 0), nil
					},
				},
				numReservationPods: 2,
				expectedGPUIndex:   "3",
			},
			"no reservation pod - create new reservation pod - happy flow": {
				clientInterceptFuncs: interceptor.Funcs{
					Watch: func(ctx context.Context, client runtimeClient.WithWatch, obj runtimeClient.ObjectList, opts ...runtimeClient.ListOption) (watch.Interface, error) {
						return exampleMockWatchPod("5", 0), nil
					},
				},
				expectedGPUIndex:   "5",
				numReservationPods: 1,
			},
			"no reservation pod - create new reservation pod - delay with pod": {
				clientInterceptFuncs: interceptor.Funcs{
					Watch: func(ctx context.Context, client runtimeClient.WithWatch, obj runtimeClient.ObjectList, opts ...runtimeClient.ListOption) (watch.Interface, error) {
						return exampleMockWatchPod("6", time.Millisecond*100), nil
					},
				},
				expectedGPUIndex:      unknownGpuIndicator,
				expectedErrorContains: "failed waiting for GPU reservation pod to allocate",
				numReservationPods:    0,
			},
			"no reservation pod - create new reservation pod - reservation pod creation fails": {
				clientInterceptFuncs: interceptor.Funcs{
					Create: func(ctx context.Context, client runtimeClient.WithWatch, obj runtimeClient.Object, opts ...runtimeClient.CreateOption) error {
						return fmt.Errorf(failedToCreatePod)
					},
				},
				expectedGPUIndex:      unknownGpuIndicator,
				expectedErrorContains: failedToCreatePod,
			},
			"no reservation pod - create new reservation pod - no gpu index annotation and fail delete reservation pod": {
				clientInterceptFuncs: interceptor.Funcs{
					Delete: func(ctx context.Context, client runtimeClient.WithWatch, obj runtimeClient.Object, opts ...runtimeClient.DeleteOption) error {
						return fmt.Errorf("failed to delete pod")
					},
				},
				numReservationPods:    1,
				expectedGPUIndex:      unknownGpuIndicator,
				expectedErrorContains: "failed waiting for GPU reservation pod to allocate",
			},
			"no reservation pod - create new reservation pod - cluster scale up check fails to list pods": {
				clientInterceptFuncs: interceptor.Funcs{
					List: func(ctx context.Context, client runtimeClient.WithWatch, list runtimeClient.ObjectList, opts ...runtimeClient.ListOption) error {
						listOpts := runtimeClient.ListOptions{}
						listOpts.ApplyOptions(opts)
						if listOpts.Namespace == scalingPodsNamespace {
							return fmt.Errorf("failed to list pods")
						}
						return client.List(ctx, list, opts...)
					},
					Watch: func(ctx context.Context, client runtimeClient.WithWatch, obj runtimeClient.ObjectList, opts ...runtimeClient.ListOption) (watch.Interface, error) {
						return exampleMockWatchPod("5", 0), nil
					},
				},
				numReservationPods: 1,
				expectedGPUIndex:   "5",
			},
			"no reservation pod - create new reservation pod - cluster scale up is finished": {
				clientInterceptFuncs: interceptor.Funcs{
					Watch: func(ctx context.Context, client runtimeClient.WithWatch, obj runtimeClient.ObjectList, opts ...runtimeClient.ListOption) (watch.Interface, error) {
						return exampleMockWatchPod("5", 0), nil
					},
				},
				expectedGPUIndex:   "5",
				numReservationPods: 1,
				reservationPod: &v1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: scalingPodsNamespace,
						Name:      "scaleup-1",
					},
					Status: v1.PodStatus{
						Conditions: []v1.PodCondition{
							{
								Type:   v1.PodScheduled,
								Status: v1.ConditionTrue,
							},
						},
					},
				},
			},
			"no reservation pod - create new reservation pod - cluster is scaling up": {
				reservationPod: &v1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: scalingPodsNamespace,
						Name:      "scaleup-1",
					},
					Status: v1.PodStatus{
						Conditions: []v1.PodCondition{
							{
								Type:   v1.PodScheduled,
								Status: v1.ConditionFalse,
							},
						},
					},
				},
				expectedGPUIndex:      unknownGpuIndicator,
				expectedErrorContains: "scaling up",
			},
			"timeout waiting for gpu index": {
				numReservationPods:    0,
				expectedGPUIndex:      unknownGpuIndicator,
				expectedErrorContains: "failed waiting for GPU",
			},
			"failed to watch reservation pod": {
				clientInterceptFuncs: interceptor.Funcs{
					Watch: func(ctx context.Context, client runtimeClient.WithWatch, obj runtimeClient.ObjectList, opts ...runtimeClient.ListOption) (watch.Interface, error) {
						return nil, fmt.Errorf("failed to watch")
					},
				},
				expectedGPUIndex:      unknownGpuIndicator,
				expectedErrorContains: "failed waiting for GPU reservation pod to allocate",
			},
			"failed to update gpu group": {
				clientInterceptFuncs: interceptor.Funcs{
					Watch: func(ctx context.Context, client runtimeClient.WithWatch, obj runtimeClient.ObjectList, opts ...runtimeClient.ListOption) (watch.Interface, error) {
						return exampleMockWatchPod("3", 0), nil
					},
					Patch: func(ctx context.Context, client runtimeClient.WithWatch, obj runtimeClient.Object, patch runtimeClient.Patch, opts ...runtimeClient.PatchOption) error {
						return fmt.Errorf("failed to patch")
					},
				},
				expectedGPUIndex:      unknownGpuIndicator,
				expectedErrorContains: "failed to patch",
			},
			"failed to update gpu group and fail to delete reservation pod": {
				clientInterceptFuncs: interceptor.Funcs{
					Watch: func(ctx context.Context, client runtimeClient.WithWatch, obj runtimeClient.ObjectList, opts ...runtimeClient.ListOption) (watch.Interface, error) {
						return exampleMockWatchPod("3", 0), nil
					},
					Patch: func(ctx context.Context, client runtimeClient.WithWatch, obj runtimeClient.Object, patch runtimeClient.Patch, opts ...runtimeClient.PatchOption) error {
						return fmt.Errorf("failed to patch")
					},
					Delete: func(ctx context.Context, client runtimeClient.WithWatch, obj runtimeClient.Object, opts ...runtimeClient.DeleteOption) error {
						return fmt.Errorf("failed to delete")
					},
				},
				expectedGPUIndex:      unknownGpuIndicator,
				expectedErrorContains: "failed to patch",
				numReservationPods:    1,
			},
		} {
			testName := testName
			testData := testData
			It(testName, func() {
				fractionPod := &v1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "team-a",
						Name:      "fraction-pod",
					},
				}
				podsInCluster := []runtime.Object{fractionPod}
				if testData.reservationPod != nil {
					podsInCluster = append(podsInCluster, testData.reservationPod)
				}
				clientWithObjs := fake.NewClientBuilder().WithRuntimeObjects(podsInCluster...).
					WithIndex(&v1.Pod{}, "spec.nodeName", nodeNameIndexer).Build()
				fakeClient := interceptor.NewClient(clientWithObjs, testData.clientInterceptFuncs)
				rsc := initializeTestService(fakeClient)

				gpuIndex, err := rsc.ReserveGpuDevice(context.TODO(), fractionPod, nodeName, existingGroup)
				Expect(gpuIndex).To(Equal(testData.expectedGPUIndex))
				if testData.expectedErrorContains == "" {
					Expect(err).To(BeNil())
					Expect(fractionPod.Labels[constants.GPUGroup], testData.groupName)
				} else {
					Expect(err).NotTo(BeNil())
					Expect(err.Error()).To(ContainSubstring(testData.expectedErrorContains))
				}
				pods := &v1.PodList{}
				err = clientWithObjs.List(context.Background(), pods,
					runtimeClient.InNamespace(namespace),
				)
				Expect(err).To(Succeed())
				Expect(len(pods.Items)).To(Equal(testData.numReservationPods))
				if testData.numReservationPods == 1 {
					Expect(pods.Items[0].Labels[constants.GPUGroup], testData.groupName)
				}
			})
		}
	})

	Context("Sync", func() {
		for testName, testData := range map[string]struct {
			podsInCluster         []runtime.Object
			clientInterceptFuncs  interceptor.Funcs
			podsLeft              int
			expectedErrorContains string
		}{
			"solitary running reservation pod": {
				podsInCluster: []runtime.Object{
					exampleReservationPod,
				},
				podsLeft: 0,
			},
			"fail to list pods": {
				podsInCluster: []runtime.Object{
					exampleReservationPod,
				},
				clientInterceptFuncs: interceptor.Funcs{
					List: func(ctx context.Context, client runtimeClient.WithWatch, list runtimeClient.ObjectList, opts ...runtimeClient.ListOption) error {
						return fmt.Errorf("failed to list pods")
					},
				},
				podsLeft:              1,
				expectedErrorContains: "failed to list pods",
			},
			"fail to delete pod": {
				podsInCluster: []runtime.Object{
					exampleReservationPod,
				},
				clientInterceptFuncs: interceptor.Funcs{
					Delete: func(ctx context.Context, client runtimeClient.WithWatch, obj runtimeClient.Object, opts ...runtimeClient.DeleteOption) error {
						return fmt.Errorf("failed to delete pod")
					},
				},
				podsLeft:              1,
				expectedErrorContains: "failed to delete pod",
			},
		} {
			testName := testName
			testData := testData
			It(testName, func() {
				clientWithObjs := fake.NewClientBuilder().WithRuntimeObjects(testData.podsInCluster...).Build()
				fakeClient := interceptor.NewClient(clientWithObjs, testData.clientInterceptFuncs)
				rsc := initializeTestService(fakeClient)

				err := rsc.Sync(context.TODO())
				if testData.expectedErrorContains == "" {
					Expect(err).To(BeNil())
				} else {
					Expect(err).NotTo(BeNil())
					Expect(err.Error()).To(ContainSubstring(testData.expectedErrorContains))
				}
				pods := &v1.PodList{}
				err = clientWithObjs.List(context.Background(), pods)
				Expect(err).To(Succeed())
				Expect(len(pods.Items)).To(Equal(testData.podsLeft))
			})
		}
	})

	Context("SyncForGpuGroup", func() {
		for testName, testData := range map[string]struct {
			podsInCluster         []runtime.Object
			clientInterceptFuncs  interceptor.Funcs
			podsLeft              int
			expectedErrorContains string
		}{
			"solitary running reservation pod": {
				podsInCluster: []runtime.Object{
					exampleReservationPod,
				},
				podsLeft: 0,
			},
			"fail to list pods": {
				podsInCluster: []runtime.Object{
					exampleReservationPod,
				},
				clientInterceptFuncs: interceptor.Funcs{
					List: func(ctx context.Context, client runtimeClient.WithWatch, list runtimeClient.ObjectList, opts ...runtimeClient.ListOption) error {
						return fmt.Errorf("failed to list pods")
					},
				},
				podsLeft:              1,
				expectedErrorContains: "failed to list",
			},
			"fail to delete reservation pod": {
				podsInCluster: []runtime.Object{
					exampleReservationPod,
				},
				clientInterceptFuncs: interceptor.Funcs{
					Delete: func(ctx context.Context, client runtimeClient.WithWatch, obj runtimeClient.Object, opts ...runtimeClient.DeleteOption) error {
						return fmt.Errorf("failed to delete pod")
					},
				},
				podsLeft:              1,
				expectedErrorContains: "failed to delete",
			},
			"fail to delete non-reserved pod": {
				podsInCluster: []runtime.Object{
					&v1.Pod{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "job-1-0-0",
							Namespace: "my-ns",
							Labels:    groupLabels,
						},
						Spec: v1.PodSpec{
							NodeName: nodeName,
						},
						Status: v1.PodStatus{
							Phase: v1.PodRunning,
						},
					},
				},
				clientInterceptFuncs: interceptor.Funcs{
					Delete: func(ctx context.Context, client runtimeClient.WithWatch, obj runtimeClient.Object, opts ...runtimeClient.DeleteOption) error {
						return fmt.Errorf("failed to delete pod")
					},
				},
				podsLeft:              1,
				expectedErrorContains: "failed to delete",
			},
			"pending pod with gpu group": {
				podsInCluster: []runtime.Object{
					&v1.Pod{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "job-1-0-0",
							Namespace: "my-ns",
							Labels:    groupLabels,
						},
						Status: v1.PodStatus{
							Phase: v1.PodPending,
						},
					},
				},
				podsLeft: 1,
			},
			"running reservation pod with running job": {
				podsInCluster: []runtime.Object{
					exampleReservationPod,
					exampleRunningJob,
				},
				podsLeft: 2,
			},
			"running reservation pod with finished and failed jobs": {
				podsInCluster: []runtime.Object{
					exampleReservationPod,
					&v1.Pod{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "job-1-0-0",
							Namespace: "my-ns",
							Labels:    groupLabels,
						},
						Spec: v1.PodSpec{
							NodeName: nodeName,
						},
						Status: v1.PodStatus{
							Phase: v1.PodFailed,
						},
					},
					&v1.Pod{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "job-2-0-0",
							Namespace: "my-ns",
							Labels:    groupLabels,
						},
						Spec: v1.PodSpec{
							NodeName: nodeName,
						},
						Status: v1.PodStatus{
							Phase: v1.PodSucceeded,
						},
					},
				},
				podsLeft: 2,
			},
			"running reservation pod with Pending job": {
				podsInCluster: []runtime.Object{
					exampleReservationPod,
					&v1.Pod{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "job-1-0-0",
							Namespace: "my-ns",
							Labels:    groupLabels,
						},
						Status: v1.PodStatus{
							Phase: v1.PodPending,
						},
					},
				},
				podsLeft: 2,
			},
			"single running job with no reservation pod": {
				podsInCluster: []runtime.Object{
					&v1.Pod{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "job-1-0-0",
							Namespace: "my-ns",
							Labels:    groupLabels,
						},
						Spec: v1.PodSpec{
							NodeName: nodeName,
						},
						Status: v1.PodStatus{
							Phase: v1.PodRunning,
						},
					},
					&v1.Pod{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "job-2-0-0",
							Namespace: "my-ns",
							Labels:    groupLabels,
						},
						Status: v1.PodStatus{
							Phase: v1.PodPending,
						},
					},
				},
				podsLeft: 1,
			},
			"running jobs with no reservation pod": {
				podsInCluster: []runtime.Object{
					&v1.Pod{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "job-1-0-0",
							Namespace: "my-ns",
							Labels:    groupLabels,
						},
						Spec: v1.PodSpec{
							NodeName: nodeName,
						},
						Status: v1.PodStatus{
							Phase: v1.PodRunning,
						},
					},
					&v1.Pod{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "job-2-0-0",
							Namespace: "my-ns",
							Labels:    groupLabels,
						},
						Spec: v1.PodSpec{
							NodeName: nodeName,
						},
						Status: v1.PodStatus{
							Phase: v1.PodPending,
						},
					},
				},
				podsLeft: 1,
			},
		} {
			testName := testName
			testData := testData
			It(testName, func() {
				clientWithObjs := fake.NewClientBuilder().WithRuntimeObjects(testData.podsInCluster...).
					WithIndex(&v1.Pod{}, "spec.nodeName", nodeNameIndexer).Build()
				fakeClient := interceptor.NewClient(clientWithObjs, testData.clientInterceptFuncs)
				rsc := initializeTestService(fakeClient)

				err := rsc.SyncForGpuGroup(context.TODO(), gpuGroup)
				if testData.expectedErrorContains == "" {
					Expect(err).To(BeNil())
				} else {
					Expect(err).NotTo(BeNil())
					Expect(err.Error()).To(ContainSubstring(testData.expectedErrorContains))
				}
				pods := &v1.PodList{}
				err = clientWithObjs.List(context.Background(), pods)
				Expect(err).To(Succeed())
				Expect(len(pods.Items)).To(Equal(testData.podsLeft))
			})
		}
	})

	Context("SyncForNode", func() {
		for testName, testData := range map[string]struct {
			podsInCluster         []runtime.Object
			clientInterceptFuncs  interceptor.Funcs
			podsLeft              int
			expectedErrorContains string
		}{
			"solitary running reservation pod": {
				podsInCluster: []runtime.Object{
					exampleReservationPod,
				},
				podsLeft: 0,
			},
			"fail to list pods on node": {
				podsInCluster: []runtime.Object{
					exampleReservationPod,
				},
				clientInterceptFuncs: interceptor.Funcs{
					List: func(ctx context.Context, client runtimeClient.WithWatch, list runtimeClient.ObjectList, opts ...runtimeClient.ListOption) error {
						return fmt.Errorf("failed to list pods")
					},
				},
				podsLeft:              1,
				expectedErrorContains: "failed to list",
			},
			"fail to delete reservation pod": {
				podsInCluster: []runtime.Object{
					exampleReservationPod,
				},
				clientInterceptFuncs: interceptor.Funcs{
					Delete: func(ctx context.Context, client runtimeClient.WithWatch, obj runtimeClient.Object, opts ...runtimeClient.DeleteOption) error {
						return fmt.Errorf("failed to delete pod")
					},
				},
				podsLeft:              1,
				expectedErrorContains: "failed to delete",
			},
			"fail to delete non-reserved pod": {
				podsInCluster: []runtime.Object{
					&v1.Pod{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "job-1-0-0",
							Namespace: "my-ns",
							Labels:    groupLabels,
						},
						Spec: v1.PodSpec{
							NodeName: nodeName,
						},
						Status: v1.PodStatus{
							Phase: v1.PodRunning,
						},
					},
				},
				clientInterceptFuncs: interceptor.Funcs{
					Delete: func(ctx context.Context, client runtimeClient.WithWatch, obj runtimeClient.Object, opts ...runtimeClient.DeleteOption) error {
						return fmt.Errorf("failed to delete pod")
					},
				},
				podsLeft:              1,
				expectedErrorContains: "failed to delete",
			},
			"pending pod with gpu group": {
				podsInCluster: []runtime.Object{
					&v1.Pod{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "job-1-0-0",
							Namespace: "my-ns",
							Labels:    groupLabels,
						},
						Status: v1.PodStatus{
							Phase: v1.PodPending,
						},
					},
				},
				podsLeft: 1,
			},
			"running reservation pod with running job": {
				podsInCluster: []runtime.Object{
					exampleReservationPod,
					exampleRunningJob,
				},
				podsLeft: 2,
			},
			"running reservation pod with running multi fractions job": {
				podsInCluster: []runtime.Object{
					&v1.Pod{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "example",
							Namespace: namespace,
							Labels: map[string]string{
								constants.GPUGroup: gpuGroup,
							},
						},
						Spec: v1.PodSpec{
							NodeName: nodeName,
						},
						Status: runningStatus,
					},
					&v1.Pod{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "example2",
							Namespace: namespace,
							Labels: map[string]string{
								constants.GPUGroup: gpuGroup2,
							},
						},
						Spec: v1.PodSpec{
							NodeName: nodeName,
						},
						Status: runningStatus,
					},
					&v1.Pod{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "job-1-0-0",
							Namespace: "my-ns",
							Labels: map[string]string{
								constants.MultiGpuGroupLabelPrefix + gpuGroup:  gpuGroup,
								constants.MultiGpuGroupLabelPrefix + gpuGroup2: gpuGroup2,
							},
						},
						Spec: v1.PodSpec{
							NodeName: nodeName,
						},
						Status: runningStatus,
					},
				},
				podsLeft: 3,
			},
			"running reservation pod with finished and failed jobs": {
				podsInCluster: []runtime.Object{
					exampleReservationPod,
					&v1.Pod{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "job-1-0-0",
							Namespace: "my-ns",
							Labels:    groupLabels,
						},
						Spec: v1.PodSpec{
							NodeName: nodeName,
						},
						Status: v1.PodStatus{
							Phase: v1.PodFailed,
						},
					},
					&v1.Pod{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "job-2-0-0",
							Namespace: "my-ns",
							Labels:    groupLabels,
						},
						Spec: v1.PodSpec{
							NodeName: nodeName,
						},
						Status: v1.PodStatus{
							Phase: v1.PodSucceeded,
						},
					},
				},
				podsLeft: 2,
			},
			"running reservation pod with Pending job": {
				podsInCluster: []runtime.Object{
					exampleReservationPod,
					&v1.Pod{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "job-1-0-0",
							Namespace: "my-ns",
							Labels:    groupLabels,
						},
						Status: v1.PodStatus{
							Phase: v1.PodPending,
						},
					},
				},
				podsLeft: 2,
			},
			"single running job with no reservation pod": {
				podsInCluster: []runtime.Object{
					&v1.Pod{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "job-1-0-0",
							Namespace: "my-ns",
							Labels:    groupLabels,
						},
						Spec: v1.PodSpec{
							NodeName: nodeName,
						},
						Status: v1.PodStatus{
							Phase: v1.PodRunning,
						},
					},
					&v1.Pod{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "job-2-0-0",
							Namespace: "my-ns",
							Labels:    groupLabels,
						},
						Status: v1.PodStatus{
							Phase: v1.PodPending,
						},
					},
				},
				podsLeft: 1,
			},
			"running jobs with no reservation pod": {
				podsInCluster: []runtime.Object{
					&v1.Pod{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "job-1-0-0",
							Namespace: "my-ns",
							Labels:    groupLabels,
						},
						Spec: v1.PodSpec{
							NodeName: nodeName,
						},
						Status: v1.PodStatus{
							Phase: v1.PodRunning,
						},
					},
					&v1.Pod{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "job-2-0-0",
							Namespace: "my-ns",
							Labels:    groupLabels,
						},
						Spec: v1.PodSpec{
							NodeName: nodeName,
						},
						Status: v1.PodStatus{
							Phase: v1.PodPending,
						},
					},
				},
				podsLeft: 1,
			},
		} {
			testName := testName
			testData := testData
			It(testName, func() {
				clientWithObjs := fake.NewClientBuilder().WithRuntimeObjects(testData.podsInCluster...).
					WithIndex(&v1.Pod{}, "spec.nodeName", nodeNameIndexer).Build()
				fakeClient := interceptor.NewClient(clientWithObjs, testData.clientInterceptFuncs)
				rsc := initializeTestService(fakeClient)

				err := rsc.SyncForNode(context.TODO(), nodeName)
				if testData.expectedErrorContains == "" {
					Expect(err).To(BeNil())
				} else {
					Expect(err).NotTo(BeNil())
					Expect(err.Error()).To(ContainSubstring(testData.expectedErrorContains))
				}
				pods := &v1.PodList{}
				err = clientWithObjs.List(context.Background(), pods)
				Expect(err).To(Succeed())
				Expect(len(pods.Items)).To(Equal(testData.podsLeft))
			})
		}
	})
})

type FakeWatchPod struct {
	Delay   time.Duration
	channel chan watch.Event
	Pod     *v1.Pod
	ticker  *time.Ticker
}

func (w *FakeWatchPod) Stop() {
	if w.ticker != nil {
		w.ticker.Stop()
	}
}

func (w *FakeWatchPod) ResultChan() <-chan watch.Event {
	if w.channel == nil {
		w.channel = make(chan watch.Event)
	}

	go func() {
		time.Sleep(w.Delay)
		w.channel <- watch.Event{Type: watch.Added, Object: w.Pod}
	}()

	return w.channel
}

func exampleMockWatchPod(gpuIndex string, delay time.Duration) watch.Interface {
	return &FakeWatchPod{
		Delay: delay,
		Pod: &v1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: namespace,
				Annotations: map[string]string{
					gpuIndexAnnotationName: gpuIndex,
				},
			},
		},
	}
}
