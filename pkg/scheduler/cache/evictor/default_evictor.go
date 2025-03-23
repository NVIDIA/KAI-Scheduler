// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package evictor

import (
	"context"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

type defaultEvictor struct {
	kubeClient kubernetes.Interface
}

func New(kubeClient kubernetes.Interface) *defaultEvictor {
	return &defaultEvictor{
		kubeClient: kubeClient,
	}
}

// +kubebuilder:rbac:groups="",resources=pods,verbs=delete

func (de *defaultEvictor) Evict(p *v1.Pod) error {
	return de.kubeClient.CoreV1().Pods(p.Namespace).Delete(context.Background(), p.Name,
		metav1.DeleteOptions{})
}
