// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package v1

import (
	v1 "k8s.io/api/core/v1"
	"k8s.io/utils/ptr"

	"github.com/NVIDIA/KAI-scheduler/pkg/apis/kai/v1/common"
)

// GlobalConfig defines the global configuration of the system
type GlobalConfig struct {
	// Openshift configures the operator to install on Openshift
	// +kubebuilder:validation:Optional
	Openshift *bool `json:"openshift,omitempty"`

	// Affinity defined affinity to the all microservices
	// +kubebuilder:validation:Optional
	Affinity *v1.Affinity `json:"affinity,omitempty"`

	// SecurityContext defines security context for the kai containers
	// +kubebuilder:validation:Optional
	SecurityContext *v1.SecurityContext `json:"securityContext,omitempty"`

	// ImagesPullSecret defines the container registry kai secret credential
	// +kubebuilder:validation:Optional
	ImagesPullSecret *string `json:"imagesPullSecret,omitempty"`

	// AdditionalImagePullSecrets defines the container registry additional secret credentials
	// +kubebuilder:validation:Optional
	AdditionalImagePullSecrets []string `json:"additionalImagePullSecrets,omitempty"`

	// ResourceRecommendation defines the resource recommendation configuration for the kai services
	// +kubebuilder:validation:Optional
	ResourceRecommendation *common.ResourceRecommendation `json:"resourceRecommendation,omitempty"`

	// Tolerations defines tolerations for kai operators & services
	// +kubebuilder:validation:Optional
	Tolerations []v1.Toleration `json:"tolerations,omitempty"`

	// DaemonsetsTolerations defines additional tolerations for daemonsets in cluster
	// +kubebuilder:validation:Optional
	DaemonsetsTolerations []v1.Toleration `json:"daemonsetsTolerations,omitempty"`

	// ReplicaCount specifies the number of replicas of services that have no specific replicas configuration
	// +kubebuilder:validation:Optional
	ReplicaCount *int32 `json:"replicaCount,omitempty"`
}

func (g *GlobalConfig) SetDefaultWhereNeeded() {
	if g.Openshift == nil {
		g.Openshift = ptr.To(false)
	}
	if g.SecurityContext == nil {
		g.SecurityContext = &v1.SecurityContext{}
	}
	if g.SecurityContext.AllowPrivilegeEscalation == nil {
		g.SecurityContext.AllowPrivilegeEscalation = ptr.To(false)
	}
	if g.SecurityContext.RunAsNonRoot == nil {
		g.SecurityContext.RunAsNonRoot = ptr.To(true)
	}
	if g.SecurityContext.RunAsUser == nil {
		g.SecurityContext.RunAsUser = ptr.To(int64(10000))
	}
	if g.SecurityContext.Capabilities == nil {
		g.SecurityContext.Capabilities = &v1.Capabilities{}
	}
	if len(g.SecurityContext.Capabilities.Drop) == 0 {
		g.SecurityContext.Capabilities.Drop = []v1.Capability{"all"}
	}
	if g.ImagesPullSecret == nil {
		g.ImagesPullSecret = ptr.To("")
	}
	if g.AdditionalImagePullSecrets == nil {
		g.AdditionalImagePullSecrets = []string{}
	}
	if g.DaemonsetsTolerations == nil {
		g.DaemonsetsTolerations = []v1.Toleration{}
	}
}

func (g *GlobalConfig) GetSecurityContext() *v1.SecurityContext {
	if g.Openshift != nil {
		if *g.Openshift {
			return nil
		}
	}
	return g.SecurityContext
}
