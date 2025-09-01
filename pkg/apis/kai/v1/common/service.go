// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

// +kubebuilder:object:generate:=true
package common

import (
	v1 "k8s.io/api/core/v1"
	"k8s.io/utils/ptr"
)

type Service struct {
	// Enabled defines whether the service should be deployed
	// +kubebuilder:validation:Optional
	Enabled *bool `json:"enabled,omitempty"`

	// Image is the configuration of the service image
	// +kubebuilder:validation:Optional
	Image *Image `json:"image,omitempty"`

	// Resources describes the resource requirements for the service pods
	// +kubebuilder:validation:Optional
	Resources *Resources `json:"resources,omitempty"`

	// ClientConfig specifies the configuration of k8s client
	// +kubebuilder:validation:Optional
	K8sClientConfig *K8sClientConfig `json:"k8sClientConfig,omitempty"`
}

func (s *Service) SetDefaultsWhereNeeded(imageName string) {
	if s.Enabled == nil {
		s.Enabled = ptr.To(true)
	}

	if s.Image == nil {
		s.Image = &Image{}
	}
	if s.Image.Name == nil {
		s.Image.Name = ptr.To(imageName)
	}
	s.Image.SetDefaultsWhereNeeded()

	if s.Resources == nil {
		s.Resources = &Resources{}
	}
	if s.Resources.Requests == nil {
		s.Resources.Requests = v1.ResourceList{}
	}
	if s.Resources.Limits == nil {
		s.Resources.Limits = v1.ResourceList{}
	}

	if s.K8sClientConfig == nil {
		s.K8sClientConfig = &K8sClientConfig{}
	}
}
