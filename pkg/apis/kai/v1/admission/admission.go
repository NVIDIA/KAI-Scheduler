// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

// +kubebuilder:object:generate:=true
package admission

import (
	"k8s.io/utils/ptr"

	"github.com/NVIDIA/KAI-scheduler/pkg/apis/kai/v1/common"
	"github.com/NVIDIA/KAI-scheduler/pkg/common/constants"
)

const (
	imageName                    = "admission"
	defaultValidatingWebhookName = "validating-kai-admission"
	defaultMutatingWebhookName   = "mutating-kai-admission"
	defaultAverageRequestsPerPod = 100
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

	// ValidatingWebhookConfigurationName is the name of the ValidatingWebhookConfiguration for the admission service
	// +kubebuilder:validation:Optional
	ValidatingWebhookConfigurationName *string `json:"validatingWebhookConfigurationName,omitempty"`

	// MutatingWebhookConfigurationName is the name of the MutatingWebhookConfiguration for the admission service
	// +kubebuilder:validation:Optional
	MutatingWebhookConfigurationName *string `json:"mutatingWebhookConfigurationName,omitempty"`

	// GPUPodRuntimeClassName specifies the runtime class to be set for GPU pods
	// set to empty string to disable
	// +kubebuilder:validation:Optional
	GPUPodRuntimeClassName *string `json:"gpuPodRuntimeClassName,omitempty"`

	// Autoscaling defines HPA configuration for the admission controller
	// +kubebuilder:validation:Optional
	Autoscaling *Autoscaling `json:"autoscaling,omitempty"`
}

func (b *Admission) SetDefaultsWhereNeeded(replicaCount *int32) {
	b.Service = common.SetDefault(b.Service, &common.Service{})
	b.Service.SetDefaultsWhereNeeded(imageName)

	b.Service.Resources = common.SetDefault(b.Service.Resources, &common.Resources{})
	b.Service.Resources.SetDefaultsWhereNeeded()

	b.Webhook = common.SetDefault(b.Webhook, &Webhook{})
	b.Webhook.SetDefaultsWhereNeeded()

	b.Replicas = common.SetDefault(b.Replicas, ptr.To(ptr.Deref(replicaCount, 1)))
	b.GPUSharing = common.SetDefault(b.GPUSharing, ptr.To(false))
	b.QueueLabelSelector = common.SetDefault(b.QueueLabelSelector, ptr.To(false))

	b.ValidatingWebhookConfigurationName = common.SetDefault(b.ValidatingWebhookConfigurationName, ptr.To(defaultValidatingWebhookName))
	b.MutatingWebhookConfigurationName = common.SetDefault(b.MutatingWebhookConfigurationName, ptr.To(defaultMutatingWebhookName))

	b.GPUPodRuntimeClassName = common.SetDefault(b.GPUPodRuntimeClassName, ptr.To(constants.DefaultRuntimeClassName))

	b.Autoscaling = common.SetDefault(b.Autoscaling, &Autoscaling{})
	b.Autoscaling.SetDefaultsWhereNeeded()
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
	w.Port = common.SetDefault(w.Port, ptr.To(443))
	w.TargetPort = common.SetDefault(w.TargetPort, ptr.To(9443))
	w.ProbePort = common.SetDefault(w.ProbePort, ptr.To(8081))
	w.MetricsPort = common.SetDefault(w.MetricsPort, ptr.To(8080))
}

// Autoscaling defines HPA configuration for the admission controller
type Autoscaling struct {
	// Enabled specifies whether autoscaling is enabled
	// +kubebuilder:validation:Optional
	Enabled *bool `json:"enabled,omitempty"`

	// MinReplicas is the minimum number of replicas for autoscaling
	// +kubebuilder:validation:Optional
	MinReplicas *int32 `json:"minReplicas,omitempty"`

	// MaxReplicas is the maximum number of replicas for autoscaling
	// +kubebuilder:validation:Optional
	MaxReplicas *int32 `json:"maxReplicas,omitempty"`

	// AverageRequestsPerPod is the target average webhook requests per pod
	// +kubebuilder:validation:Optional
	AverageRequestsPerPod *int32 `json:"averageRequestsPerPod,omitempty"`
}

// SetDefaultsWhereNeeded sets default fields for unset fields
func (a *Autoscaling) SetDefaultsWhereNeeded() {
	a.Enabled = common.SetDefault(a.Enabled, ptr.To(false))
	a.MinReplicas = common.SetDefault(a.MinReplicas, ptr.To(int32(1)))
	a.MaxReplicas = common.SetDefault(a.MaxReplicas, ptr.To(int32(5)))
	a.AverageRequestsPerPod = common.SetDefault(a.AverageRequestsPerPod, ptr.To(int32(defaultAverageRequestsPerPod)))
}
