// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

// +kubebuilder:object:generate:=true
package admission

import (
	"k8s.io/utils/ptr"

	"github.com/NVIDIA/KAI-scheduler/pkg/apis/kai/v1/common"
)

const (
	imageName = "admission"
)

type Admission struct {
	Service *common.Service `json:"service,omitempty"`

	// Webhook defines configuration for the admission service
	// +kubebuilder:validation:Optional
	Webhook *Webhook `json:"webhook,omitempty"`

	// Replicas specifies the number of replicas of the admission controller
	// +kubebuilder:validation:Optional
	Replicas *int32 `json:"replicas,omitempty"`

	// GPUSharing enables GPU sharing functionality for the admission service
	// +kubebuilder:validation:Optional
	GPUSharing *bool `json:"gpuSharing,omitempty"`

	// QueueLabelSelector enables the queue label MatchExpression in webhooks
	// +kubebuilder:validation:Optional
	QueueLabelSelector *bool `json:"queueLabelSelector,omitempty"`
}

func (b *Admission) SetDefaultsWhereNeeded(replicaCount *int32) {
	if b.Service == nil {
		b.Service = &common.Service{}
	}
	b.Service.SetDefaultsWhereNeeded(imageName)

	if b.Service.Enabled == nil {
		b.Service.Enabled = ptr.To(true)
	}

	if b.Service.Image == nil {
		b.Service.Image = &common.Image{}
	}
	b.Service.Image.SetDefaultsWhereNeeded()
	if len(*b.Service.Image.Name) == 0 {
		b.Service.Image.Name = ptr.To(imageName)
	}

	if b.Service.Resources == nil {
		b.Service.Resources = &common.Resources{}
	}
	b.Service.Resources.SetDefaultsWhereNeeded()

	if b.Webhook == nil {
		b.Webhook = &Webhook{}
	}
	b.Webhook.SetDefaultsWhereNeeded()

	if b.Replicas == nil {
		b.Replicas = ptr.To(ptr.Deref(replicaCount, 1))
	}

	if b.GPUSharing == nil {
		b.GPUSharing = ptr.To(false)
	}

	if b.QueueLabelSelector == nil {
		b.QueueLabelSelector = ptr.To(false)
	}
}

// Webhook defines configuration for the admission webhook
type Webhook struct {
	// Port specifies the webhook service port
	// +kubebuilder:validation:Optional
	Port *int `json:"port,omitempty"`

	// TargetPort specifies the webhook service container port
	// +kubebuilder:validation:Optional
	TargetPort *int `json:"targetPort,omitempty"`

	// ProbePort specifies the health and readiness probe port
	ProbePort *int `json:"probePort,omitempty"`

	// MetricsPort specifies the metrics service port
	MetricsPort *int `json:"metricsPort,omitempty"`
}

// SetDefaultsWhereNeeded sets default fields for unset fields
func (w *Webhook) SetDefaultsWhereNeeded() {
	if w.Port == nil {
		w.Port = ptr.To(443)
	}

	if w.TargetPort == nil {
		w.TargetPort = ptr.To(9443)
	}

	if w.ProbePort == nil {
		w.ProbePort = ptr.To(8081)
	}

	if w.MetricsPort == nil {
		w.MetricsPort = ptr.To(8080)
	}
}
