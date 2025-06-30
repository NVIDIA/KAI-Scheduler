// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package gpusharingconfigmap

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

type configMapNameBCTest struct {
	pod                  *v1.Pod
	desiredConfigMapName string
	expectedError        bool
}

func TestGetDesiredConfigMapNameBC(t *testing.T) {
	tests := []configMapNameBCTest{
		{
			pod: &v1.Pod{
				Spec: v1.PodSpec{
					Volumes: []v1.Volume{
						{
							Name: "job-f57b32439da7-9lxzvkq-runai-sh-gpu-vol",
							VolumeSource: v1.VolumeSource{
								ConfigMap: &v1.ConfigMapVolumeSource{
									LocalObjectReference: v1.LocalObjectReference{
										Name: "job-f57b32439da7-9lxzvkq-runai-sh-gpu",
									},
								},
							},
						},
					},
				},
			},
			desiredConfigMapName: "job-f57b32439da7-9lxzvkq-runai-sh-gpu",
			expectedError:        false,
		},
		{
			pod:                  &v1.Pod{},
			desiredConfigMapName: "",
			expectedError:        true,
		},
		{
			pod: &v1.Pod{
				Spec: v1.PodSpec{
					Volumes: []v1.Volume{
						{
							Name: "job-f57b32439da7-9lxzvkq-runai-sh-gpu-vol",
							VolumeSource: v1.VolumeSource{
								ConfigMap: &v1.ConfigMapVolumeSource{
									LocalObjectReference: v1.LocalObjectReference{
										Name: "job-f57b32439da7-9lxzvkq-runai-sh-gpu-bla",
									},
								},
							},
						},
					},
				},
			},
			desiredConfigMapName: "",
			expectedError:        true,
		},
		{
			pod: &v1.Pod{
				Spec: v1.PodSpec{
					Volumes: []v1.Volume{
						{
							Name: "job-f57b32439da7-9lxzvkq-runai-sh-gpu-vol",
							VolumeSource: v1.VolumeSource{
								CSI: &v1.CSIVolumeSource{},
							},
						},
					},
				},
			},
			desiredConfigMapName: "",
			expectedError:        true,
		},
	}

	for _, test := range tests {
		_, err := ExtractCapabilitiesConfigMapName(test.pod, 0, RegularContainer)
		if err == nil {
			t.Errorf("Expected error but got none")
		}

		configMapName, err := GetDesiredConfigMapNameBC(test.pod)
		if test.expectedError && err == nil {
			t.Errorf("Expected error but got none")
		}
		if !test.expectedError && err != nil {
			t.Errorf("Expected no error but got %v", err)
		}
		if configMapName != test.desiredConfigMapName {
			t.Errorf("Expected config map name %v but got %v", test.desiredConfigMapName, configMapName)
		}
	}
}

type updateBCPodTest struct {
	name                   string
	pod                    *v1.Pod
	expectedCmName         string
	expectError            bool
	expectedPodAnnotations map[string]string
}

func TestUpdateBCPod(t *testing.T) {
	desiredCmName := fmt.Sprintf("configmap-name-%s", GPUSharingConfigMap)
	tests := []updateBCPodTest{
		{
			name: "legacy pod with no config map annotation",
			pod: &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:        "test-pod",
					Namespace:   "test-namespace",
					Annotations: map[string]string{},
				},
				Spec: v1.PodSpec{
					Volumes: []v1.Volume{
						{
							Name: fmt.Sprintf("%s-vol", desiredCmName),
							VolumeSource: v1.VolumeSource{
								ConfigMap: &v1.ConfigMapVolumeSource{
									LocalObjectReference: v1.LocalObjectReference{Name: desiredCmName},
								},
							},
						},
					},
				},
			},
			expectedCmName: desiredCmName,
			expectError:    false,
			expectedPodAnnotations: map[string]string{
				DesiredConfigMapPrefixKey: desiredCmName,
			},
		},
		{
			name: "legacy pod with no config map annotation and existing annotations",
			pod: &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pod",
					Namespace: "test-namespace",
					Annotations: map[string]string{
						"bla": "blu",
					},
				},
				Spec: v1.PodSpec{
					Volumes: []v1.Volume{
						{
							Name: fmt.Sprintf("%s-vol", desiredCmName),
							VolumeSource: v1.VolumeSource{
								ConfigMap: &v1.ConfigMapVolumeSource{
									LocalObjectReference: v1.LocalObjectReference{Name: desiredCmName},
								},
							},
						},
					},
				},
			},
			expectedCmName: desiredCmName,
			expectError:    false,
			expectedPodAnnotations: map[string]string{
				DesiredConfigMapPrefixKey: desiredCmName,
				"bla":                     "blu",
			},
		},
		{
			name: "legacy pod with no volume",
			pod: &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:        "test-pod",
					Namespace:   "test-namespace",
					Annotations: map[string]string{},
				},
			},
			expectedCmName:         "",
			expectError:            true,
			expectedPodAnnotations: nil,
		},
		{
			name: "pod with two volumes",
			pod: &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pod",
					Namespace: "test-namespace",
				},
				Spec: v1.PodSpec{
					Volumes: []v1.Volume{
						{
							Name: fmt.Sprintf("%s-vol", desiredCmName),
							VolumeSource: v1.VolumeSource{
								ConfigMap: &v1.ConfigMapVolumeSource{
									LocalObjectReference: v1.LocalObjectReference{Name: desiredCmName},
								},
							},
						},
						{
							Name: fmt.Sprintf("%s-2-vol", desiredCmName),
							VolumeSource: v1.VolumeSource{
								ConfigMap: &v1.ConfigMapVolumeSource{
									LocalObjectReference: v1.LocalObjectReference{Name: desiredCmName},
								},
							},
						},
					},
				},
			},
			expectedCmName:         "",
			expectError:            true,
			expectedPodAnnotations: nil,
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	for _, test := range tests {
		kubeClient := fake.NewClientBuilder().WithObjects(test.pod).Build()
		desiredName, err := HandleBCPod(ctx, kubeClient, test.pod)

		if test.expectError {
			assert.Error(t, err)
		} else {
			assert.Nil(t, err)
		}

		assert.Equal(t, test.expectedCmName, desiredName)

		updatedPod := &v1.Pod{}
		err = kubeClient.Get(ctx, client.ObjectKeyFromObject(test.pod), updatedPod)
		assert.Nil(t, err)

		assert.Equal(t, test.expectedPodAnnotations, updatedPod.Annotations)
	}
}
