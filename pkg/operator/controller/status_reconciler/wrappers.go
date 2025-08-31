// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package status_reconciler

import (
	kaiv1 "github.com/NVIDIA/KAI-scheduler/pkg/apis/kai/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type KAIConfigWithStatusWrapper struct {
	*kaiv1.Config
}

func (e *KAIConfigWithStatusWrapper) GetConditions() []metav1.Condition {
	return e.Status.Conditions
}

func (e *KAIConfigWithStatusWrapper) SetConditions(conditions []metav1.Condition) {
	e.Status.Conditions = conditions
}

func (e *KAIConfigWithStatusWrapper) DeepCopy() objectWithConditions {
	return &KAIConfigWithStatusWrapper{e.Config.DeepCopy()}
}

func (e *KAIConfigWithStatusWrapper) GetInternalObject() client.Object {
	return e.Config
}
