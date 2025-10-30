// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package runtimeenforcement

import (
	"context"
	"reflect"
	"testing"

	"github.com/NVIDIA/KAI-scheduler/pkg/apis/client/clientset/versioned/scheme"
	"github.com/NVIDIA/KAI-scheduler/pkg/common/constants"
	ocpconf "github.com/openshift/api/config/v1"
	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
	nodev1 "k8s.io/api/node/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
)

func TestMutate(t *testing.T) {
	sc := scheme.Scheme
	utilruntime.Must(ocpconf.AddToScheme(sc))

	tests := []struct {
		name                   string
		gpuPodRuntimeClassName string
		incomingPod            *v1.Pod
		k8s                    client.Client
		expectedOutboundPod    *v1.Pod
		expectedError          error
	}{
		{
			name:                   "pod without GPU requests and runtimeClass does not exist",
			gpuPodRuntimeClassName: constants.DefaultRuntimeClassName,
			k8s:                    nonOCPClientWithoutRuntimeClass(),
			incomingPod:            &v1.Pod{},
			expectedOutboundPod:    &v1.Pod{},
			expectedError:          nil,
		},
		{
			name:                   "pod without GPU requests and runtimeClass exists",
			gpuPodRuntimeClassName: constants.DefaultRuntimeClassName,
			k8s:                    nonOCPClientWithRuntimeClass(constants.DefaultRuntimeClassName),
			incomingPod:            &v1.Pod{},
			expectedOutboundPod:    &v1.Pod{},
			expectedError:          nil,
		},
		{
			name:                   "pod with a fractional GPU request and runtimeClass does not exist",
			gpuPodRuntimeClassName: constants.DefaultRuntimeClassName,
			k8s:                    nonOCPClientWithoutRuntimeClass(),
			incomingPod: &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{constants.GpuFraction: "0.5"},
				},
			},
			expectedOutboundPod: &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{constants.GpuFraction: "0.5"},
				},
			},
			expectedError: runtimeClassDoesNotExistError(constants.DefaultRuntimeClassName),
		},
		{
			name:                   "pod with a fractional GPU request and runtimeClass exists",
			gpuPodRuntimeClassName: constants.DefaultRuntimeClassName,
			k8s:                    nonOCPClientWithRuntimeClass(constants.DefaultRuntimeClassName),
			incomingPod: &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{constants.GpuFraction: "0.5"},
				},
			},
			expectedOutboundPod: &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{constants.GpuFraction: "0.5"},
				},
				Spec: v1.PodSpec{
					RuntimeClassName: ptr.To(constants.DefaultRuntimeClassName),
				},
			},
			expectedError: nil,
		},
		{
			name:                   "pod with a whole GPU request and runtimeClass does not exist",
			gpuPodRuntimeClassName: constants.DefaultRuntimeClassName,
			k8s:                    nonOCPClientWithoutRuntimeClass(),
			incomingPod: &v1.Pod{
				Spec: v1.PodSpec{
					Containers: []v1.Container{
						{
							Resources: v1.ResourceRequirements{
								Limits: v1.ResourceList{constants.GpuResource: resource.MustParse("1")},
							},
						},
					},
				},
			},
			expectedOutboundPod: &v1.Pod{
				Spec: v1.PodSpec{
					Containers: []v1.Container{
						{
							Resources: v1.ResourceRequirements{
								Limits: v1.ResourceList{constants.GpuResource: resource.MustParse("1")},
							},
						},
					},
				},
			},
			expectedError: runtimeClassDoesNotExistError(constants.DefaultRuntimeClassName),
		},
		{
			name:                   "pod with a whole GPU request and runtimeClass exists",
			gpuPodRuntimeClassName: constants.DefaultRuntimeClassName,
			k8s:                    nonOCPClientWithRuntimeClass(constants.DefaultRuntimeClassName),
			incomingPod: &v1.Pod{
				Spec: v1.PodSpec{
					Containers: []v1.Container{
						{
							Resources: v1.ResourceRequirements{
								Limits: v1.ResourceList{constants.GpuResource: resource.MustParse("1")},
							},
						},
					},
				},
			},
			expectedOutboundPod: &v1.Pod{
				Spec: v1.PodSpec{
					RuntimeClassName: ptr.To(constants.DefaultRuntimeClassName),
					Containers: []v1.Container{
						{
							Resources: v1.ResourceRequirements{
								Limits: v1.ResourceList{constants.GpuResource: resource.MustParse("1")},
							},
						},
					},
				},
			},
			expectedError: nil,
		},
		{
			name:                   "OCP - pod without GPU requests",
			gpuPodRuntimeClassName: constants.DefaultRuntimeClassName,
			k8s:                    ocpClient(),
			incomingPod:            &v1.Pod{},
			expectedOutboundPod:    &v1.Pod{},
			expectedError:          nil,
		},
		{
			name:                   "OCP - pod with a fractional GPU request",
			gpuPodRuntimeClassName: constants.DefaultRuntimeClassName,
			k8s:                    ocpClient(),
			incomingPod: &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{constants.GpuFraction: "0.5"},
				},
			},
			expectedOutboundPod: &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{constants.GpuFraction: "0.5"},
				},
			},
			expectedError: nil,
		},
		{
			name:                   "OCP - pod with a whole GPU request",
			gpuPodRuntimeClassName: constants.DefaultRuntimeClassName,
			k8s:                    ocpClient(),
			incomingPod: &v1.Pod{
				Spec: v1.PodSpec{
					Containers: []v1.Container{
						{
							Resources: v1.ResourceRequirements{
								Limits: v1.ResourceList{constants.GpuResource: resource.MustParse("1")},
							},
						},
					},
				},
			},
			expectedOutboundPod: &v1.Pod{
				Spec: v1.PodSpec{
					Containers: []v1.Container{
						{
							Resources: v1.ResourceRequirements{
								Limits: v1.ResourceList{constants.GpuResource: resource.MustParse("1")},
							},
						},
					},
				},
			},
			expectedError: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := New(tt.k8s, tt.gpuPodRuntimeClassName)
			err := p.Mutate(tt.incomingPod)
			if tt.expectedError != nil {
				assert.Error(t, err)
				assert.Equal(t, tt.expectedError.Error(), err.Error())
			} else {
				assert.NoError(t, err)
			}
			assert.Equal(t, tt.expectedOutboundPod, tt.incomingPod)
		})
	}
}

func ocpClient() client.Client {
	sc := scheme.Scheme
	utilruntime.Must(ocpconf.AddToScheme(sc))
	utilruntime.Must(nodev1.AddToScheme(sc))

	return fake.NewClientBuilder().WithScheme(sc).WithObjects(&ocpconf.ClusterVersion{
		ObjectMeta: metav1.ObjectMeta{
			Name: "cluster",
		},
	}).Build()
}

var nonOCPInterceptorFuncs = interceptor.Funcs{
	List: func(ctx context.Context, c client.WithWatch, list client.ObjectList, opts ...client.ListOption) error {
		if reflect.TypeOf(list) == reflect.TypeOf(&ocpconf.ClusterVersionList{}) {
			return &meta.NoResourceMatchError{}
		}

		return c.List(ctx, list, opts...)
	},
}

func nonOCPClientWithoutRuntimeClass() client.Client {
	sc := scheme.Scheme
	utilruntime.Must(ocpconf.AddToScheme(sc))
	utilruntime.Must(nodev1.AddToScheme(sc))

	return fake.NewClientBuilder().WithScheme(sc).WithInterceptorFuncs(nonOCPInterceptorFuncs).Build()
}

func nonOCPClientWithRuntimeClass(runtimeClassName string) client.Client {
	sc := scheme.Scheme
	utilruntime.Must(ocpconf.AddToScheme(sc))
	utilruntime.Must(nodev1.AddToScheme(sc))

	return fake.NewClientBuilder().WithScheme(sc).WithInterceptorFuncs(nonOCPInterceptorFuncs).WithObjects(&nodev1.RuntimeClass{
		ObjectMeta: metav1.ObjectMeta{
			Name: runtimeClassName,
		},
	}).Build()
}
