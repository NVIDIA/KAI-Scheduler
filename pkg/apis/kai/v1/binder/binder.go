// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

// +kubebuilder:object:generate:=true
package binder

import (
	"k8s.io/utils/ptr"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"

	"github.com/NVIDIA/KAI-scheduler/pkg/apis/kai/v1/common"
)

const (
	imageName                           = "binder"
	defaultResourceReservationImageName = "resource-reservation"
)

type Binder struct {
	// Enabled defines whether the pod grouper should be deployed
	// +kubebuilder:validation:Optional
	Enabled *bool `json:"enabled,omitempty"`

	// Image is the configuration of the pod-grouper image
	// +kubebuilder:validation:Optional
	Image *common.Image `json:"image,omitempty"`

	// Resources describes the resource requirements for the binder pod
	// +kubebuilder:validation:Optional
	Resources *common.Resources `json:"resources,omitempty"`

	// Webhook defines configuration for the binder service
	// +kubebuilder:validation:Optional
	Webhook *Webhook `json:"webhook,omitempty"`

	// ResourceReservation controls configuration for the resource reservation functionality
	// +kubebuilder:validation:Optional
	ResourceReservation *ResourceReservation `json:"resourceReservation,omitempty"`

	// Args specifies the CLI arguments for the binder
	// +kubebuilder:validation:Optional
	Args *Args `json:"args,omitempty"`

	// Replicas specifies the number of replicas of the kai binder service
	// +kubebuilder:validation:Optional
	Replicas *int32 `json:"replicas,omitempty"`
}

func (b *Binder) ResourcesByRecommendationSize(size *common.ResourceRecommendation) *common.Resources {
	if b.Resources != nil {
		return b.Resources
	}

	if size == nil {
		size = ptr.To(common.ResourceRecommendationSmall)
	}

	switch *size {
	case common.ResourceRecommendationSmall:
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
	case common.ResourceRecommendationMedium:
		return &common.Resources{
			Requests: v1.ResourceList{
				v1.ResourceCPU:    resource.MustParse("50m"),
				v1.ResourceMemory: resource.MustParse("450Mi"),
			},
			Limits: v1.ResourceList{
				v1.ResourceCPU:    resource.MustParse("100m"),
				v1.ResourceMemory: resource.MustParse("450Mi"),
			},
		}
	case common.ResourceRecommendationLarge:
		return &common.Resources{
			Requests: v1.ResourceList{
				v1.ResourceCPU:    resource.MustParse("100m"),
				v1.ResourceMemory: resource.MustParse("2500Mi"),
			},
			Limits: v1.ResourceList{
				v1.ResourceCPU:    resource.MustParse("400m"),
				v1.ResourceMemory: resource.MustParse("2500Mi"),
			},
		}
	default:
		return common.DefaultServiceResources()
	}
}

func (b *Binder) SetDefaultsWhereNeeded(replicaCount *int32) {
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

	if b.Webhook == nil {
		b.Webhook = &Webhook{}
	}
	b.Webhook.SetDefaultsWhereNeeded()

	if b.Replicas == nil {
		b.Replicas = ptr.To(ptr.Deref(replicaCount, 1))
	}

	if b.ResourceReservation == nil {
		b.ResourceReservation = &ResourceReservation{}
	}
	b.ResourceReservation.SetDefaultsWhereNeeded()

	if b.Args == nil {
		b.Args = &Args{}
	}
	b.Args.SetDefaultsWhereNeeded()
}

// Args specifies the CLI arguments for the binder
type Args struct {
	// MaxConcurrentReconciles is the maximum number of concurrent reconciles for both pods and BindRequests
	// +kubebuilder:validation:Optional
	MaxConcurrentReconciles *int `json:"maxConcurrentReconciles,omitempty"`

	// VolumeBindingTimeoutSeconds specifies the timeout for volume binding in seconds
	// +kubebuilder:validation:Optional
	VolumeBindingTimeoutSeconds *int `json:"volumeBindingTimeoutSeconds,omitempty"`

	// QPS represents the Queries Per Second limit
	// +kubebuilder:validation:Optional
	QPS *int `json:"qps"`

	// Burst represents the maximum burst (number of requests) allowed at the QPS limit
	// +kubebuilder:validation:Optional
	Burst *int `json:"burst"`
}

// SetDefaultsWhereNeeded sets default fields for unset fields
func (a *Args) SetDefaultsWhereNeeded() {
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

type ResourceReservation struct {
	// Image is the image used by the resource reservation pods
	// +kubebuilder:validation:Optional
	Image *common.Image `json:"image,omitempty"`

	// AllocationTimeout specifies the timeout for resource reservation pod allocation in seconds
	AllocationTimeout *int `json:"allocationTimeout,omitempty"`
}

func (r *ResourceReservation) SetDefaultsWhereNeeded() {
	if r.Image == nil {
		r.Image = &common.Image{}
	}
	if r.Image.Name == nil {
		r.Image.Name = ptr.To(defaultResourceReservationImageName)
	}
	r.Image.SetDefaultsWhereNeeded()
}
