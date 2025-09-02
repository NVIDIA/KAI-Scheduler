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
	Service *common.Service `json:"service,omitempty"`

	// ControllerService describes the service for the queue-controller
	// +kubebuilder:validation:Optional
	ControllerService *Service `json:"controllerService,omitempty"`

	// Webhooks describes the configuration of the queue controller webhooks
	// +kubebuilder:validation:Optional
	Webhooks *QueueControllerWebhooks `json:"webhooks,omitempty"`

	// Replicas specifies the number of replicas of the queue controller
	// +kubebuilder:validation:Optional
	Replicas *int32 `json:"replicas,omitempty"`

	// MetricsNamespace specifies the namespace where metrics are exposed for the queue controller
	// +kubebuilder:validation:Optional
	MetricsNamespace *string `json:"metricsNamespace,omitempty"`

	// QueueLabelToMetricLabel maps queue label keys to metric label keys for metrics exposure
	// +kubebuilder:validation:Optional
	QueueLabelToMetricLabel *string `json:"queueLabelToMetricLabel,omitempty"`

	// QueueLabelToDefaultMetricValue maps queue label keys to default metric values when the label is absent
	// +kubebuilder:validation:Optional
	QueueLabelToDefaultMetricValue *string `json:"queueLabelToDefaultMetricValue,omitempty"`
}

func (q *QueueController) SetDefaultsWhereNeeded(replicaCount *int32) {
	if q.Service == nil {
		q.Service = &common.Service{}
	}
	q.Service.SetDefaultsWhereNeeded(imageName)

	if q.Service.Enabled == nil {
		q.Service.Enabled = ptr.To(true)
	}

	if q.Service.Image == nil {
		q.Service.Image = &common.Image{}
	}
	if q.Service.Image.Name == nil {
		q.Service.Image.Name = ptr.To(imageName)
	}
	q.Service.Image.SetDefaultsWhereNeeded()

	if q.Service.Resources == nil {
		q.Service.Resources = &common.Resources{}
	}
	if q.Service.Resources.Requests == nil {
		q.Service.Resources.Requests = v1.ResourceList{}
	}
	if q.Service.Resources.Limits == nil {
		q.Service.Resources.Limits = v1.ResourceList{}
	}

	if _, found := q.Service.Resources.Requests[v1.ResourceCPU]; !found {
		q.Service.Resources.Requests[v1.ResourceCPU] = resource.MustParse("20m")
	}
	if _, found := q.Service.Resources.Requests[v1.ResourceMemory]; !found {
		q.Service.Resources.Requests[v1.ResourceMemory] = resource.MustParse("50Mi")
	}
	if _, found := q.Service.Resources.Limits[v1.ResourceCPU]; !found {
		q.Service.Resources.Limits[v1.ResourceCPU] = resource.MustParse("50m")
	}
	if _, found := q.Service.Resources.Limits[v1.ResourceMemory]; !found {
		q.Service.Resources.Limits[v1.ResourceMemory] = resource.MustParse("100Mi")
	}

	if q.ControllerService == nil {
		q.ControllerService = &Service{}
	}
	q.ControllerService.SetDefaultsWhereNeeded()

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
