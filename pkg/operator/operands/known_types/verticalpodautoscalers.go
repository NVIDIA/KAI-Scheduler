// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package known_types

import (
	"context"

	vpav1 "k8s.io/autoscaler/vertical-pod-autoscaler/pkg/apis/autoscaling.k8s.io/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

func vpaIndexer(object client.Object) []string {
	vpa := object.(*vpav1.VerticalPodAutoscaler)
	owner := metav1.GetControllerOf(vpa)
	if !checkOwnerType(owner) {
		return nil
	}
	return []string{getOwnerKey(owner)}
}

func registerVerticalPodAutoscalers() {
	collectable := &Collectable{
		Collect: getCurrentVPAState,
		InitWithManager: func(ctx context.Context, mgr manager.Manager) error {
			return mgr.GetFieldIndexer().IndexField(ctx, &vpav1.VerticalPodAutoscaler{}, CollectableOwnerKey, vpaIndexer)
		},
		InitWithBuilder: func(builder *builder.Builder) *builder.Builder {
			return builder.Owns(&vpav1.VerticalPodAutoscaler{})
		},
		InitWithFakeClientBuilder: func(fakeClientBuilder *fake.ClientBuilder) {
			fakeClientBuilder.WithIndex(&vpav1.VerticalPodAutoscaler{}, CollectableOwnerKey, vpaIndexer)
		},
	}
	SetupKAIConfigOwned(collectable)
	SetupSchedulingShardOwned(collectable)
}

func getCurrentVPAState(ctx context.Context, runtimeClient client.Client, reconciler client.Object) (map[string]client.Object, error) {
	result := map[string]client.Object{}
	vpas := &vpav1.VerticalPodAutoscalerList{}
	reconcilerKey := getReconcilerKey(reconciler)

	err := runtimeClient.List(ctx, vpas, client.MatchingFields{CollectableOwnerKey: reconcilerKey})
	if err != nil {
		return nil, err
	}

	for _, vpa := range vpas.Items {
		result[GetKey(vpa.GroupVersionKind(), vpa.Namespace, vpa.Name)] = &vpa
	}

	return result, nil
}
