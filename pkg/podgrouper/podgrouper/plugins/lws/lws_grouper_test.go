// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package leader_worker_set

import (
	"testing"

	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"github.com/NVIDIA/KAI-scheduler/pkg/podgrouper/podgrouper/plugins/defaultgrouper"
)

func TestGetPodGroupMetadata_Parallel(t *testing.T) {
	owner := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"kind":       "LeaderWorkerSet",
			"apiVersion": "lws.k8s.io/v1",
			"metadata": map[string]interface{}{
				"name":      "lws-test",
				"namespace": "default",
				"uid":       "lws-uid",
			},
			"spec": map[string]interface{}{
				"startupPolicy": "Parallel",
				"leaderWorkerTemplate": map[string]interface{}{
					"replicas": int64(3),
				},
			},
		},
	}

	pod := &v1.Pod{}

	lwsGrouper := NewLwsGrouper(defaultgrouper.NewDefaultGrouper("", ""))
	podGroupMetadata, err := lwsGrouper.GetPodGroupMetadata(owner, pod)

	assert.Nil(t, err)
	assert.Equal(t, "LeaderWorkerSet", podGroupMetadata.Owner.Kind)
	assert.Equal(t, "lws.k8s.io/v1", podGroupMetadata.Owner.APIVersion)
	assert.Equal(t, "lws-uid", string(podGroupMetadata.Owner.UID))
	assert.Equal(t, "lws-test", podGroupMetadata.Owner.Name)
	assert.Equal(t, int32(3), podGroupMetadata.MinAvailable)
	assert.Equal(t, "inference", podGroupMetadata.PriorityClassName)
}

func TestGetPodGroupMetadata_Serial(t *testing.T) {
	owner := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"kind":       "LeaderWorkerSet",
			"apiVersion": "lws.k8s.io/v1",
			"metadata": map[string]interface{}{
				"name":      "lws-test-serial",
				"namespace": "default",
				"uid":       "lws-serial-uid",
			},
			"spec": map[string]interface{}{
				"startupPolicy": "Serial",
				"leaderWorkerTemplate": map[string]interface{}{
					"replicas": int64(5),
				},
			},
		},
	}

	pod := &v1.Pod{}

	lwsGrouper := NewLwsGrouper(defaultgrouper.NewDefaultGrouper("", ""))
	podGroupMetadata, err := lwsGrouper.GetPodGroupMetadata(owner, pod)

	assert.Nil(t, err)
	assert.Equal(t, int32(1), podGroupMetadata.MinAvailable)
	assert.Equal(t, "inference", podGroupMetadata.PriorityClassName)
}