// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package scheduler

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	kaiv1 "github.com/NVIDIA/KAI-scheduler/pkg/apis/kai/v1"
	"github.com/NVIDIA/KAI-scheduler/pkg/operator/operands/common"
)

const (
	serviceAccountName = "scheduler"
)

func serviceAccountForKAIConfig(
	ctx context.Context, readerClient client.Reader,
	kaiConfig *kaiv1.Config,
) (*corev1.ServiceAccount, error) {
	sa, err := common.ObjectForKAIConfig(
		ctx, readerClient, &corev1.ServiceAccount{},
		serviceAccountName, kaiConfig.Spec.Namespace,
	)
	if err != nil {
		return nil, err
	}
	sa.(*corev1.ServiceAccount).TypeMeta = metav1.TypeMeta{
		Kind:       "ServiceAccount",
		APIVersion: "v1",
	}
	return sa.(*corev1.ServiceAccount), err
}
