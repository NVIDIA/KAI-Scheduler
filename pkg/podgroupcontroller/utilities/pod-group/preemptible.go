// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package pod_group

import (
	"context"

	schedulingv1 "k8s.io/api/scheduling/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/NVIDIA/KAI-scheduler/pkg/apis/scheduling/v2alpha2"
)

const (
	PriorityBuildNumber = 100
)

func IsPreemptible(ctx context.Context, podGroup *v2alpha2.PodGroup, kubeClient client.Client) (bool, error) {
	priority, err := getPodGroupPriority(ctx, podGroup, kubeClient)
	if err != nil {
		return false, err
	}

	return priority < PriorityBuildNumber, nil
}

func getPodGroupPriority(ctx context.Context, podGroup *v2alpha2.PodGroup, kubeClient client.Client) (int32, error) {
	priorityClass := schedulingv1.PriorityClass{}
	err := kubeClient.Get(
		ctx,
		types.NamespacedName{Namespace: podGroup.Namespace, Name: podGroup.Spec.PriorityClassName},
		&priorityClass,
	)
	if err != nil {
		return -1, err
	}
	return priorityClass.Value, nil
}
