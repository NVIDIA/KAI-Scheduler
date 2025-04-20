// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package podgrouper_test

import (
	"context"
	"fmt"
	"reflect"
	"testing"

	argov1alpha1 "github.com/argoproj/argo-workflows/v3/pkg/apis/workflow/v1alpha1"

	"github.com/NVIDIA/KAI-scheduler/pkg/podgrouper/podgroup"
	"github.com/NVIDIA/KAI-scheduler/pkg/podgrouper/podgrouper"
	"github.com/stretchr/testify/assert"
	"go.uber.org/multierr"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var pod = v1.Pod{
	TypeMeta: metav1.TypeMeta{
		Kind:       "Pod",
		APIVersion: "v1",
	},
	ObjectMeta: metav1.ObjectMeta{
		Name:      "pod",
		Namespace: "namespace",
		OwnerReferences: []metav1.OwnerReference{{
			APIVersion: "run.ai/v1",
			Kind:       "RunaiJob",
			Name:       "job",
		}},
	},
	Spec:   v1.PodSpec{},
	Status: v1.PodStatus{},
}

var podJob = v1.Pod{
	TypeMeta: metav1.TypeMeta{
		Kind:       "Pod",
		APIVersion: "v1",
	},
	ObjectMeta: metav1.ObjectMeta{
		Name:      "pod-job",
		Namespace: "namespace",
	},
	Spec:   v1.PodSpec{},
	Status: v1.PodStatus{},
}

var podJobSpawn = v1.Pod{
	TypeMeta: metav1.TypeMeta{
		Kind:       "Pod",
		APIVersion: "v1",
	},
	ObjectMeta: metav1.ObjectMeta{
		Name:      "pod-job-spawn",
		Namespace: "namespace",
		OwnerReferences: []metav1.OwnerReference{{
			APIVersion: "v1",
			Kind:       "Pod",
			Name:       "pod-job",
		}},
	},
	Spec:   v1.PodSpec{},
	Status: v1.PodStatus{},
}

var runaiTestResources = []runtime.Object{
	&unstructured.Unstructured{
		Object: map[string]interface{}{
			"kind":       "RunaiJob",
			"apiVersion": "run.ai/v1",
			"metadata": map[string]interface{}{
				"name":      "job",
				"namespace": "namespace",
			},
			"Spec":   map[string]interface{}{},
			"Status": map[string]interface{}{},
		},
	},
	&unstructured.Unstructured{
		Object: map[string]interface{}{
			"kind":       "RunaiJob",
			"apiVersion": "run.ai/v1",
			"metadata": map[string]interface{}{
				"name":      "job-2",
				"namespace": "namespace",
			},
			"Spec":   map[string]interface{}{},
			"Status": map[string]interface{}{},
		},
	},
}

var nativeK8sTestResources = []runtime.Object{
	&pod,
	&podJob,
	&podJobSpawn,
}

func TestNewPodgrouper(t *testing.T) {
	scheme := runtime.NewScheme()
	err := v1.AddToScheme(scheme)
	if err != nil {
		t.Fail()
	}

	resources := append(nativeK8sTestResources, runaiTestResources...)
	client := fake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(resources...).Build()

	grouper := podgrouper.NewPodgrouper(client, client, false, true)

	topOwner, owners, err := grouper.GetPodOwners(context.Background(), &pod)
	assert.Nil(t, err)
	_, err = grouper.GetPGMetadata(context.Background(), &pod, topOwner, owners)
	assert.Nil(t, err)

	topOwner, owners, err = grouper.GetPodOwners(context.Background(), &podJob)
	assert.Nil(t, err)
	podGroup, err := grouper.GetPGMetadata(context.Background(), &podJob, topOwner, owners)
	assert.Nil(t, err)
	assert.Equal(t, podJob.Name, podGroup.Owner.Name)
	assert.Equal(t, podJob.Kind, podGroup.Owner.Kind)

	topOwner, owners, err = grouper.GetPodOwners(context.Background(), &podJob)
	assert.Nil(t, err)
	podGroup, err = grouper.GetPGMetadata(context.Background(), &podJobSpawn, topOwner, owners)
	assert.Nil(t, err)
	assert.Equal(t, podJob.Name, podGroup.Owner.Name)
	assert.Equal(t, podJob.Kind, podGroup.Owner.Kind)
}

func Test_Podgrouper_Full_Flow(t *testing.T) {
	type podGrouperOptions struct {
		searchForLegacyPodGroups bool
		gangScheduleKnative      bool
	}

	tests := []struct {
		name              string
		podGrouperOptions *podGrouperOptions
		resources         []runtime.Object
		reconciledPod     *v1.Pod
		expectedMetadata  *podgroup.Metadata
	}{
		{
			name: "sanity - pod job",
			resources: []runtime.Object{
				&v1.Pod{
					TypeMeta: metav1.TypeMeta{
						Kind:       "Pod",
						APIVersion: "v1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:      "pod-job",
						Namespace: "namespace",
					},
					Spec:   v1.PodSpec{},
					Status: v1.PodStatus{},
				},
			},
			reconciledPod: &v1.Pod{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Pod",
					APIVersion: "v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "pod-job",
					Namespace: "namespace",
				},
				Spec:   v1.PodSpec{},
				Status: v1.PodStatus{},
			},
			expectedMetadata: &podgroup.Metadata{
				Annotations: map[string]string{
					"runai/job-id": "",
					"run.ai/top-owner-metadata": `name: pod-job
uid: ""
group: ""
version: v1
kind: Pod
`,
				},
				Labels:            map[string]string{},
				PriorityClassName: "train",
				Queue:             "default-queue",
				Namespace:         "namespace",
				Name:              "pg-pod-job-",
				MinAvailable:      1,
				Owner: metav1.OwnerReference{
					APIVersion: "v1",
					Kind:       "Pod",
					Name:       "pod-job",
					UID:        "",
				},
			},
		},
		{
			name: "argo - non gang schedule",
			podGrouperOptions: &podGrouperOptions{
				searchForLegacyPodGroups: false,
				gangScheduleKnative:      true,
			},
			resources: []runtime.Object{
				&v1.Pod{
					TypeMeta: metav1.TypeMeta{
						Kind:       "Pod",
						APIVersion: "v1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:      "argo-pod",
						Namespace: "namespace",
						UID:       "uid-pod",
						OwnerReferences: []metav1.OwnerReference{{
							APIVersion: "argoproj.io/v1alpha1",
							Kind:       "Workflow",
							Name:       "hello-world-workflow",
							UID:        "uid",
						}},
					},
					Spec:   v1.PodSpec{},
					Status: v1.PodStatus{},
				},
				&argov1alpha1.Workflow{
					TypeMeta: metav1.TypeMeta{
						Kind:       "Workflow",
						APIVersion: "argoproj.io/v1alpha1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:      "hello-world-workflow",
						Namespace: "namespace",
						UID:       "uid",
						Labels: map[string]string{
							"runai/queue": "test",
						},
					},
					Spec:   argov1alpha1.WorkflowSpec{},
					Status: argov1alpha1.WorkflowStatus{},
				},
			},
			reconciledPod: &v1.Pod{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Pod",
					APIVersion: "v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "argo-pod",
					Namespace: "namespace",
					UID:       "uid-pod",
					OwnerReferences: []metav1.OwnerReference{{
						APIVersion: "argoproj.io/v1alpha1",
						Kind:       "Workflow",
						Name:       "hello-world-workflow",
						UID:        "uid",
					}},
				},
				Spec:   v1.PodSpec{},
				Status: v1.PodStatus{},
			},
			expectedMetadata: &podgroup.Metadata{
				Annotations: map[string]string{
					"runai/job-id": "uid-pod",
					"run.ai/top-owner-metadata": `name: argo-pod
uid: uid-pod
group: ""
version: v1
kind: Pod
`,
				},
				Labels: map[string]string{
					"runai/queue": "test",
				},
				PriorityClassName: "train",
				Queue:             "test",
				Namespace:         "namespace",
				Name:              "pg-argo-pod-uid-pod",
				MinAvailable:      1,
				Owner: metav1.OwnerReference{
					APIVersion: "v1",
					Kind:       "Pod",
					Name:       "argo-pod",
					UID:        "uid-pod",
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scheme := runtime.NewScheme()
			err := v1.AddToScheme(scheme)
			if err != nil {
				t.Fail()
			}

			assert.Nil(t, argov1alpha1.AddToScheme(scheme))

			client := fake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(tt.resources...).Build()

			if tt.podGrouperOptions == nil {
				tt.podGrouperOptions = &podGrouperOptions{
					searchForLegacyPodGroups: false,
					gangScheduleKnative:      true,
				}
			}
			grouper := podgrouper.NewPodgrouper(client, client,
				tt.podGrouperOptions.searchForLegacyPodGroups,
				tt.podGrouperOptions.gangScheduleKnative,
			)

			topOwner, owners, err := grouper.GetPodOwners(context.Background(), tt.reconciledPod)
			assert.Nil(t, err)
			metadata, err := grouper.GetPGMetadata(context.Background(), tt.reconciledPod, topOwner, owners)
			assert.Nil(t, err)
			assert.Nil(t, compareMetadata(tt.expectedMetadata, metadata))
		})
	}
}

func compareMetadata(expected, got *podgroup.Metadata) error {
	var err error
	if !reflect.DeepEqual(expected.Annotations, got.Annotations) {
		err = multierr.Append(err, fmt.Errorf("annotations not equal:\n expected:\n%v\ngot:\n%v\n", expected.Annotations, got.Annotations))
	}

	if !reflect.DeepEqual(expected.Labels, got.Labels) {
		err = multierr.Append(err, fmt.Errorf("labels not equal:\n expected:\n%v\ngot:\n%v\n", expected.Labels, got.Labels))
	}

	if expected.PriorityClassName != got.PriorityClassName {
		err = multierr.Append(err, fmt.Errorf("priority class name not equal:\n expected:\n%v\ngot:\n%v\n", expected.PriorityClassName, got.PriorityClassName))
	}

	if expected.Queue != got.Queue {
		err = multierr.Append(err, fmt.Errorf("queue not equal:\n expected:\n%v\ngot:\n%v\n", expected.Queue, got.Queue))
	}

	if expected.Namespace != got.Namespace {
		err = multierr.Append(err, fmt.Errorf("namespace not equal:\n expected:\n%v\ngot:\n%v\n", expected.Namespace, got.Namespace))
	}

	if expected.Name != got.Name {
		err = multierr.Append(err, fmt.Errorf("name not equal:\n expected:\n%v\ngot:\n%v\n", expected.Name, got.Name))
	}

	if expected.MinAvailable != got.MinAvailable {
		err = multierr.Append(err, fmt.Errorf("min available not equal:\n expected:\n%v\ngot:\n%v\n", expected.MinAvailable, got.MinAvailable))
	}

	if !reflect.DeepEqual(expected.Owner, got.Owner) {
		err = multierr.Append(err, fmt.Errorf("owner not equal:\n expected:\n%v\ngot:\n%v\n", expected.Owner, got.Owner))
	}

	if !reflect.DeepEqual(expected, got) {
		err = multierr.Append(err, fmt.Errorf("metadata not equal:\n expected:\n%v\ngot:\n%v\n", expected, got))
	}

	return err
}
