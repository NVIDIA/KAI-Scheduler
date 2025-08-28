// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

// +kubebuilder:object:generate:=true
package pod_group_controller

import (
	"github.com/NVIDIA/KAI-scheduler/pkg/apis/kai/v1/common"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/utils/ptr"
)

const (
	imageName = "pod-group-controller"
)

type PodGroupController struct {
	// Enabled defines whether the pod grouper should be deployed
	// +kubebuilder:validation:Optional
	Enabled *bool `json:"enabled,omitempty"`

	// Image is the configuration of the pod-grouper image
	// +kubebuilder:validation:Optional
	Image *common.Image `json:"image,omitempty"`

	// Resources describes the resource requirements for the pod grouper pod
	// +kubebuilder:validation:Optional
	Resources *common.Resources `json:"resources,omitempty"`

	// ClientConfig specifies the configuration of k8s client
	// +kubebuilder:validation:Optional
	K8sClientConfig *common.K8sClientConfig `json:"k8sClientConfig,omitempty"`

	// MaxConcurrentReconciles specifies the number of max concurrent reconcile workers
	// +kubebuilder:validation:Optional
	MaxConcurrentReconciles *int `json:"maxConcurrentReconciles,omitempty"`

	// Args specifies the CLI arguments for the pod-group-controller
	// +kubebuilder:validation:Optional
	Args *Args `json:"args,omitempty"`

	// Replicas specifies the number of replicas of the pod-group controller
	// +kubebuilder:validation:Optional
	Replicas *int32 `json:"replicas,omitempty"`
}

// Args defines command line arguments for the pod-group-controller
type Args struct {
	// InferencePreemptible should inference priority class be counted as preemptibile
	InferencePreemptible *bool `json:"inferencePreemptible,omitempty"`
}

func (pg *PodGroupController) ResourcesByRecommendationSize(size *common.ResourceRecommendation) *common.Resources {
	if pg.Resources != nil {
		return pg.Resources
	}

	if size == nil {
		size = ptr.To(common.ResourceRecommendationSmall)
	}

	switch *size {
	case common.ResourceRecommendationSmall:
		return &common.Resources{
			Requests: v1.ResourceList{
				v1.ResourceCPU:    resource.MustParse("20m"),
				v1.ResourceMemory: resource.MustParse("100Mi"),
			},
			Limits: v1.ResourceList{
				v1.ResourceCPU:    resource.MustParse("500m"),
				v1.ResourceMemory: resource.MustParse("100Mi"),
			},
		}
	case common.ResourceRecommendationMedium:
		return &common.Resources{
			Requests: v1.ResourceList{
				v1.ResourceCPU:    resource.MustParse("30m"),
				v1.ResourceMemory: resource.MustParse("400Mi"),
			},
			Limits: v1.ResourceList{
				v1.ResourceCPU:    resource.MustParse("800m"),
				v1.ResourceMemory: resource.MustParse("400Mi"),
			},
		}
	case common.ResourceRecommendationLarge:
		return &common.Resources{
			Requests: v1.ResourceList{
				v1.ResourceCPU:    resource.MustParse("50m"),
				v1.ResourceMemory: resource.MustParse("8000Mi"),
			},
			Limits: v1.ResourceList{
				v1.ResourceCPU:    resource.MustParse("200m"),
				v1.ResourceMemory: resource.MustParse("8000Mi"),
			},
		}
	default:
		return common.DefaultServiceResources()
	}
}

func (pg *PodGroupController) SetDefaultsWhereNeeded(replicaCount *int32) {
	if pg.Enabled == nil {
		pg.Enabled = ptr.To(true)
	}

	if pg.Image == nil {
		pg.Image = &common.Image{}
	}
	if pg.Image.Name == nil {
		pg.Image.Name = ptr.To(imageName)
	}
	pg.Image.SetDefaultsWhereNeeded()

	if pg.K8sClientConfig == nil {
		pg.K8sClientConfig = &common.K8sClientConfig{}
	}

	if pg.Args == nil {
		pg.Args = &Args{}
	}

	if pg.Replicas == nil {
		pg.Replicas = ptr.To(ptr.Deref(replicaCount, 1))
	}
}
