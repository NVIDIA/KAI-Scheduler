// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

// +kubebuilder:object:generate:=true
package scheduler

import (
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/utils/ptr"

	"github.com/NVIDIA/KAI-scheduler/pkg/apis/kai/v1/common"
)

const (
	defaultImageName = "kai-scheduler"
)

type Scheduler struct {
	// Enabled declare if the scheduler should be installed
	// +kubebuilder:validation:Optional
	Enabled *bool `json:"enabled,omitempty"`

	// Image is the image to use in the container
	// +kubebuilder:validation:Optional
	Image *common.Image `json:"image,omitempty"`

	// Resources describes the resource requirements for the microservice
	// +kubebuilder:validation:Optional
	Resources *common.Resources `json:"resources,omitempty"`

	// GOGC configures the GOGC environment variable for the scheduler container
	// +kubebuilder:validation:Optional
	GOGC *int `json:"GOGC,omitempty"`

	// Service specifies the service configuration for the kai-scheduler
	// +kubebuilder:validation:Optional
	Service *Service `json:"service,omitempty"`

	// Replicas specifies the number of replicas of the scheduler service
	// +kubebuilder:validation:Optional
	Replicas *int32 `json:"replicas,omitempty"`
}

func (s *Scheduler) ResourcesByRecommendationSize(size *common.ResourceRecommendation) *common.Resources {
	if s.Resources != nil {
		return s.Resources
	}

	if size == nil {
		size = ptr.To(common.ResourceRecommendationSmall)
	}

	switch *size {
	case common.ResourceRecommendationSmall:
		return &common.Resources{
			Requests: v1.ResourceList{
				v1.ResourceCPU:    resource.MustParse("250m"),
				v1.ResourceMemory: resource.MustParse("512Mi"),
			},
			Limits: v1.ResourceList{
				v1.ResourceCPU:    resource.MustParse("700m"),
				v1.ResourceMemory: resource.MustParse("512Mi"),
			},
		}
	case common.ResourceRecommendationMedium:
		return &common.Resources{
			Requests: v1.ResourceList{
				v1.ResourceCPU:    resource.MustParse("1"),
				v1.ResourceMemory: resource.MustParse("1Gi"),
			},
			Limits: v1.ResourceList{
				v1.ResourceCPU:    resource.MustParse("2"),
				v1.ResourceMemory: resource.MustParse("1Gi"),
			},
		}
	case common.ResourceRecommendationLarge:
		return &common.Resources{
			Requests: v1.ResourceList{
				v1.ResourceCPU:    resource.MustParse("3"),
				v1.ResourceMemory: resource.MustParse("7Gi"),
			},
			Limits: v1.ResourceList{
				v1.ResourceCPU:    resource.MustParse("5"),
				v1.ResourceMemory: resource.MustParse("7Gi"),
			},
		}
	default:
		return common.DefaultServiceResources()
	}
}

func (s *Scheduler) SetDefaultsWhereNeeded(replicaCount *int32) {
	if s.Enabled == nil {
		s.Enabled = ptr.To(true)
	}
	if s.Image == nil {
		s.Image = &common.Image{}
	}
	if s.Image.Name == nil {
		s.Image.Name = ptr.To(defaultImageName)
	}
	s.Image.SetDefaultsWhereNeeded()

	if s.GOGC == nil {
		s.GOGC = ptr.To(400)
	}

	if s.Service == nil {
		s.Service = &Service{}
	}
	s.Service.SetDefaultsWhereNeeded()

	if s.Replicas == nil {
		s.Replicas = ptr.To(ptr.Deref(replicaCount, 1))
	}
}

// Service defines configuration for the kai-scheduler service
type Service struct {
	// Type specifies the service type
	// +kubebuilder:validation:Optional
	Type v1.ServiceType `json:"type,omitempty"`

	// Port specifies the service port
	// +kubebuilder:validation:Optional
	Port *int `json:"port,omitempty"`

	// TargetPort specifies the target port in the container
	// +kubebuilder:validation:Optional
	TargetPort *int `json:"targetPort,omitempty"`
}

func (service *Service) SetDefaultsWhereNeeded() {
	if service.Type == "" {
		service.Type = v1.ServiceTypeClusterIP
	}
	if service.Port == nil {
		service.Port = ptr.To(8080)
	}
	if service.TargetPort == nil {
		service.TargetPort = ptr.To(8080)
	}
}
