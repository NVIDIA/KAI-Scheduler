// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

// +kubebuilder:object:generate:=true
package node_scale_adjuster

import (
	"k8s.io/utils/ptr"

	"github.com/NVIDIA/KAI-scheduler/pkg/apis/kai/v1/common"
)

const (
	imageName            = "node-scale-adjuster"
	scalingPodImageName  = "scaling-pod"
	scaleAdjustNamespace = "kai-scale-adjust"
)

type NodeScaleAdjuster struct {
	// Enabled defines whether the pod grouper should be deployed
	// +kubebuilder:validation:Optional
	Enabled *bool `json:"enabled,omitempty"`

	// Image is the configuration of the node-scale-adjuster image
	// +kubebuilder:validation:Optional
	Image *common.Image `json:"image,omitempty"`

	// Resources describes the resource requirements for the node-scale-adjuster pod
	// +kubebuilder:validation:Optional
	Resources *common.Resources `json:"resources,omitempty"`

	// ScalingPodImage is the image to use for the scaling pod
	// +kubebuilder:validation:Optional
	ScalingPodImage *common.Image `json:"scalingPodImage,omitempty"`

	// NodeScaleNamespace is the namespace in which the scaling pods will be created
	// +kubebuilder:validation:Optional
	NodeScaleNamespace *string `json:"nodeScaleNamespace,omitempty"`

	// Args specifies the CLI arguments for node-scale-adjuster
	// +kubebuilder:validation:Optional
	Args *Args `json:"args,omitempty"`
}

// Args specifies the CLI arguments for node-scale-adjuster
type Args struct {
	// GPUMemoryToFractionRatio is the ratio of GPU memory to fraction conversion
	// +kubebuilder:validation:Optional
	GPUMemoryToFractionRatio *float64 `json:"gpuMemoryToFractionRatio,omitempty"`
}

// SetDefaultsWhereNeeded sets default for unset fields
func (nsa *NodeScaleAdjuster) SetDefaultsWhereNeeded() {
	if nsa.Enabled == nil {
		nsa.Enabled = ptr.To(false)
	}

	if nsa.Image == nil {
		nsa.Image = &common.Image{}
	}
	if nsa.Image.Name == nil {
		nsa.Image.Name = ptr.To(imageName)
	}
	nsa.Image.SetDefaultsWhereNeeded()

	if nsa.Resources == nil {
		nsa.Resources = &common.Resources{}
	}
	nsa.Resources.SetDefaultsWhereNeeded()

	if nsa.ScalingPodImage == nil {
		nsa.ScalingPodImage = &common.Image{}
	}
	if nsa.ScalingPodImage.Name == nil {
		nsa.ScalingPodImage.Name = ptr.To(scalingPodImageName)
	}
	nsa.ScalingPodImage.SetDefaultsWhereNeeded()

	if nsa.NodeScaleNamespace == nil {
		nsa.NodeScaleNamespace = ptr.To(scaleAdjustNamespace)
	}

	if nsa.Args == nil {
		nsa.Args = &Args{}
	}
}
