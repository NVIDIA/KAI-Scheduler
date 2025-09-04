// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package v1

import (
	v1 "k8s.io/api/core/v1"
	"k8s.io/utils/ptr"

	"github.com/NVIDIA/KAI-scheduler/pkg/apis/kai/v1/common"
	"github.com/NVIDIA/KAI-scheduler/pkg/common/constants"
)

// GlobalConfig defines the global configuration of the system
type GlobalConfig struct {
	// Openshift configures the operator to install on Openshift
	// +kubebuilder:validation:Optional
	Openshift *bool `json:"openshift,omitempty"`

	// Affinity defined affinity to the all microservices
	// +kubebuilder:validation:Optional
	Affinity *v1.Affinity `json:"affinity,omitempty"`

	// SecurityContext defines security context for the KAI containers
	// +kubebuilder:validation:Optional
	SecurityContext *v1.SecurityContext `json:"securityContext,omitempty"`

	// ImagePullSecrets defines the container registry additional secret credentials
	// +kubebuilder:validation:Optional
	ImagePullSecrets []string `json:"additionalImagePullSecrets,omitempty"`

	// Tolerations defines tolerations for KAI operators & services
	// +kubebuilder:validation:Optional
	Tolerations []v1.Toleration `json:"tolerations,omitempty"`

	// DaemonsetsTolerations defines additional tolerations for daemonsets in cluster
	// +kubebuilder:validation:Optional
	DaemonsetsTolerations []v1.Toleration `json:"daemonsetsTolerations,omitempty"`

	// ReplicaCount specifies the number of replicas of services that have no specific replicas configuration
	// +kubebuilder:validation:Optional
	ReplicaCount *int32 `json:"replicaCount,omitempty"`

	// QueueLabelKey specifies the pod label key whose value will be the queue name of the pod.
	// +kubebuilder:validation:Optional
	QueueLabelKey *string `json:"queueLabelKey,omitempty"`

	// SchedulerName specifies the name of the KAI scheduler. Pods must set this value
	// in their `spec.schedulerName` field to be processed by the KAI scheduler.
	// +kubebuilder:validation:Optional
	SchedulerName *string `json:"schedulerName,omitempty"`

	// NodePoolLabelKey is the label name by with to filter nodes, pods and other resources that the scheduler is watching
	// +kubebuilder:validation:Optional
	NodePoolLabelKey *string `json:"nodePoolLabelKey,omitempty"`

	// NamespaceLabelSelector filters namespaces for webhooks and pod grouper
	// +kubebuilder:validation:Optional
	NamespaceLabelSelector map[string]string `json:"namespaceLabelSelector,omitempty"`

	// PodLabelSelector filters pods for webhooks and pod grouper
	// +kubebuilder:validation:Optional
	PodLabelSelector map[string]string `json:"podLabelSelector,omitempty"`
}

func (g *GlobalConfig) SetDefaultWhereNeeded() {
	g.Openshift = common.SetDefault(g.Openshift, ptr.To(false))
	g.SecurityContext = common.SetDefault(g.SecurityContext, &v1.SecurityContext{})
	g.SecurityContext.AllowPrivilegeEscalation = common.SetDefault(g.SecurityContext.AllowPrivilegeEscalation, ptr.To(false))
	g.SecurityContext.RunAsNonRoot = common.SetDefault(g.SecurityContext.RunAsNonRoot, ptr.To(true))
	g.SecurityContext.RunAsUser = common.SetDefault(g.SecurityContext.RunAsUser, ptr.To(int64(10000)))
	g.SecurityContext.Capabilities = common.SetDefault(g.SecurityContext.Capabilities, &v1.Capabilities{})
	if len(g.SecurityContext.Capabilities.Drop) == 0 {
		g.SecurityContext.Capabilities.Drop = []v1.Capability{"all"}
	}

	if g.ImagePullSecrets == nil {
		g.ImagePullSecrets = []string{}
	}
	if g.DaemonsetsTolerations == nil {
		g.DaemonsetsTolerations = []v1.Toleration{}
	}
	g.QueueLabelKey = common.SetDefault(g.QueueLabelKey, ptr.To(constants.DefaultQueueLabel))
	g.SchedulerName = common.SetDefault(g.SchedulerName, ptr.To(constants.DefaultSchedulerName))

	g.NodePoolLabelKey = common.SetDefault(g.NodePoolLabelKey, ptr.To(constants.DefaultNodePoolLabelKey))

	if g.NamespaceLabelSelector == nil {
		g.NamespaceLabelSelector = map[string]string{}
	}
	if g.PodLabelSelector == nil {
		g.PodLabelSelector = map[string]string{}
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
