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
	// Enabled defines whether the pod grouper should be deployed
	// +kubebuilder:validation:Optional
	Enabled *bool `json:"enabled,omitempty"`

	// Image is the configuration of the pod-grouper image
	// +kubebuilder:validation:Optional
	Image *common.Image `json:"image,omitempty"`

	// Resources describes the resource requirements for the admission pod
	// +kubebuilder:validation:Optional
	Resources *common.Resources `json:"resources,omitempty"`

	// Webhook defines configuration for the admission service
	// +kubebuilder:validation:Optional
	Webhook *Webhook `json:"webhook,omitempty"`

	// Replicas specifies the number of replicas of the admission controller
	// +kubebuilder:validation:Optional
	Replicas *int32 `json:"replicas,omitempty"`
}

func (b *Admission) SetDefaultsWhereNeeded(replicaCount *int32) {
	if b.Enabled == nil {
		b.Enabled = ptr.To(true)
	}

	if b.Image == nil {
		b.Image = &common.Image{}
	}
	b.Image.SetDefaultsWhereNeeded()
	if len(*b.Image.Name) == 0 {
		b.Image.Name = ptr.To(imageName)
	}

	if b.Resources == nil {
		b.Resources = &common.Resources{}
	}
	b.Resources.SetDefaultsWhereNeeded()

	if b.Webhook == nil {
		b.Webhook = &Webhook{}
	}
	b.Webhook.SetDefaultsWhereNeeded()

	if b.Replicas == nil {
		b.Replicas = ptr.To(ptr.Deref(replicaCount, 1))
	}
}

// Webhook defines configuration for the binder webhook
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
