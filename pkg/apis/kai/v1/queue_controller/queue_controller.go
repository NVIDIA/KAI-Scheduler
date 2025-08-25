// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

// +kubebuilder:object:generate:=true
package queue_controller

import (
	"github.com/NVIDIA/KAI-scheduler/pkg/apis/kai/v1/common"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/utils/ptr"
)

const (
	imageName = "queue-controller"
)

type QueueController struct {
	// Enabled defines whether the queue controller should be deployed
	// +kubebuilder:validation:Optional
	Enabled *bool `json:"enabled,omitempty"`

	// Image is the configuration of the queue controller image
	// +kubebuilder:validation:Optional
	Image *common.Image `json:"image,omitempty"`

	// Resources describes the resource requirements for the queue controller pod
	// +kubebuilder:validation:Optional
	Resources *common.Resources `json:"resources,omitempty"`

	// Service describes the service for the queue-controller
	// +kubebuilder:validation:Optional
	Service *Service `json:"service,omitempty"`

	// Webhooks describes the configuration of the queue controller webhooks
	// +kubebuilder:validation:Optional
	Webhooks *QueueControllerWebhooks `json:"webhooks,omitempty"`

	// Replicas specifies the number of replicas of the queue controller
	// +kubebuilder:validation:Optional
	Replicas *int32 `json:"replicas,omitempty"`
}

func (q *QueueController) ResourcesByRecommendationSize(size *common.ResourceRecommendation) *common.Resources {
	if q.Resources != nil {
		return q.Resources
	}

	if size == nil {
		size = ptr.To(common.ResourceRecommendationSmall)
	}

	switch *size {
	case common.ResourceRecommendationSmall:
		return &common.Resources{
			Requests: v1.ResourceList{
				v1.ResourceCPU:    resource.MustParse("20m"),
				v1.ResourceMemory: resource.MustParse("50Mi"),
			},
			Limits: v1.ResourceList{
				v1.ResourceCPU:    resource.MustParse("50m"),
				v1.ResourceMemory: resource.MustParse("100Mi"),
			},
		}
	case common.ResourceRecommendationMedium:
		return &common.Resources{
			Requests: v1.ResourceList{
				v1.ResourceCPU:    resource.MustParse("20m"),
				v1.ResourceMemory: resource.MustParse("200Mi"),
			},
			Limits: v1.ResourceList{
				v1.ResourceCPU:    resource.MustParse("70m"),
				v1.ResourceMemory: resource.MustParse("200Mi"),
			},
		}
	case common.ResourceRecommendationLarge:
		return &common.Resources{
			Requests: v1.ResourceList{
				v1.ResourceCPU:    resource.MustParse("50m"),
				v1.ResourceMemory: resource.MustParse("200Mi"),
			},
			Limits: v1.ResourceList{
				v1.ResourceCPU:    resource.MustParse("100m"),
				v1.ResourceMemory: resource.MustParse("200Mi"),
			},
		}
	default:
		return common.DefaultServiceResources()
	}
}

func (q *QueueController) SetDefaultsWhereNeeded(replicaCount *int32) {
	if q.Enabled == nil {
		q.Enabled = ptr.To(true)
	}

	if q.Image == nil {
		q.Image = &common.Image{}
	}
	if q.Image.Name == nil {
		q.Image.Name = ptr.To(imageName)
	}
	q.Image.SetDefaultsWhereNeeded()

	if q.Service == nil {
		q.Service = &Service{}
	}
	q.Service.SetDefaultsWhereNeeded()

	if q.Webhooks == nil {
		q.Webhooks = &QueueControllerWebhooks{}
	}
	if q.Replicas == nil {
		q.Replicas = ptr.To(ptr.Deref(replicaCount, 1))
	}
	q.Webhooks.SetDefaultsWhereNeeded()
}

type Service struct {
	// Metrics specifies the metrics service spec
	// +kubebuilder:validation:Optional
	Metrics *PortMapping `json:"metrics,omitempty"`

	// Webhook specifies the webhook service spec
	// +kubebuilder:validation:Optional
	Webhook *PortMapping `json:"webhook,omitempty"`
}

func (s *Service) SetDefaultsWhereNeeded() {
	if s.Metrics == nil {
		s.Metrics = &PortMapping{}
	}
	s.Metrics.SetDefaultsWhereNeeded()
	s.Metrics.Port = ptr.To(8080)
	s.Metrics.TargetPort = ptr.To(8080)
	s.Metrics.Name = ptr.To("metrics")

	if s.Webhook == nil {
		s.Webhook = &PortMapping{}
	}
	s.Webhook.SetDefaultsWhereNeeded()
	s.Webhook.Port = ptr.To(443)
	s.Webhook.TargetPort = ptr.To(9443)
	s.Webhook.Name = ptr.To("webhook")
}

type PortMapping struct {
	// Port specifies the service port
	// +kubebuilder:validation:Optional
	Port *int `json:"port,omitempty"`

	// TargetPort specifies the pod container port
	// +kubebuilder:validation:Optional
	TargetPort *int `json:"targetPort,omitempty"`

	// Name specifies the name of the port
	// +kubebuilder:validation:Optional
	Name *string `json:"name,omitempty"`
}

func (p *PortMapping) SetDefaultsWhereNeeded() {
	if p.Port == nil {
		p.Port = ptr.To(8080)
	}

	if p.TargetPort == nil {
		p.TargetPort = ptr.To(8080)
	}

	if p.Name == nil {
		p.Name = ptr.To("metrics")
	}
}

type QueueControllerWebhooks struct {
	EnableValidation *bool `json:"enableValidation,omitempty"`
}

func (q *QueueControllerWebhooks) SetDefaultsWhereNeeded() {
	if q.EnableValidation == nil {
		q.EnableValidation = ptr.To(true)
	}
}
