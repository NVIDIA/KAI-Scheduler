// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package spark

import (
	"testing"

	"golang.org/x/exp/maps"

	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

func TestIsSparkPod(t *testing.T) {
	pod := &v1.Pod{
		TypeMeta: metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "pod-1",
			Namespace: "test_namespace",
			Labels: map[string]string{
				"runai/queue": "test_queue",
			},
			UID: "3",
		},
	}

	assert.False(t, IsSparkPod(pod))

	maps.Copy(pod.Labels, map[string]string{
		sparkAppLabelName: "spark-app",
	})

	assert.False(t, IsSparkPod(pod))

	maps.Copy(pod.Labels, map[string]string{
		sparkAppSelectorLabelName: "spark-selector",
	})

	assert.True(t, IsSparkPod(pod))
}

func TestGetPodGroupMetadata(t *testing.T) {
	pod := &v1.Pod{
		TypeMeta: metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "pod-1",
			Namespace: "test_namespace",
			Labels: map[string]string{
				sparkAppLabelName:         "spark-app",
				sparkAppSelectorLabelName: "spark-selector",
			},
			UID: "3",
		},
	}

	// ownerReference to unstructured
	rawObjectMap, err := runtime.DefaultUnstructuredConverter.ToUnstructured(pod)
	assert.NoError(t, err)

	unstructuredPod := &unstructured.Unstructured{
		Object: rawObjectMap,
	}

	podGroupMetadata, err := GetPodGroupMetadata(unstructuredPod, pod)
	assert.NoError(t, err)
	assert.Equal(t, "spark-selector", podGroupMetadata.Name)
}
