// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package jax

import (
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/NVIDIA/KAI-scheduler/pkg/podgrouper/podgroup"
	"github.com/NVIDIA/KAI-scheduler/pkg/podgrouper/podgrouper/plugins/kubeflow"
)

const (
	ReplicaSpecName = "jaxReplicaSpecs"
	WorkerName      = "Worker"
	// Jax does not support master
)

type JaxGrouper struct {
	*kubeflow.KubeflowDistributedGrouper
}

func NewJaxGrouper(kubeflowGrouper *kubeflow.KubeflowDistributedGrouper) *JaxGrouper {
	return &JaxGrouper{
		KubeflowDistributedGrouper: kubeflowGrouper,
	}
}

func (jg *JaxGrouper) Name() string {
	return "JAX Grouper"
}

func (jg *JaxGrouper) GetPodGroupMetadata(
	topOwner *unstructured.Unstructured, pod *v1.Pod, _ ...*metav1.PartialObjectMetadata,
) (*podgroup.Metadata, error) {
	return jg.KubeflowDistributedGrouper.GetPodGroupMetadata(topOwner, pod, ReplicaSpecName, []string{WorkerName})
}
