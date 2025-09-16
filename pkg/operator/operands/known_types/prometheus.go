// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package known_types

import (
	"context"

	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

func prometheusIndexer(object client.Object) []string {
	prometheus := object.(*monitoringv1.Prometheus)
	owner := metav1.GetControllerOf(prometheus)
	if !checkOwnerType(owner) {
		return nil
	}
	return []string{getOwnerKey(owner)}
}

func registerPrometheus() {
	// Only register Prometheus collectable if CRD is available
	// We'll check this at runtime during manager initialization
	collectable := &Collectable{
		Collect: getCurrentPrometheusState,
		InitWithManager: func(ctx context.Context, mgr manager.Manager) error {
			// Check if Prometheus CRD exists before registering the indexer
			if !isPrometheusCRDAvailable(ctx, mgr.GetClient()) {
				log.FromContext(ctx).Info("Prometheus CRD not available, skipping Prometheus resource management")
				return nil
			}
			log.FromContext(ctx).Info("Prometheus CRD available, registering Prometheus resource management")
			return mgr.GetFieldIndexer().IndexField(ctx, &monitoringv1.Prometheus{}, CollectableOwnerKey, prometheusIndexer)
		},
		InitWithBuilder: func(builder *builder.Builder) *builder.Builder {
			// Only register the watch if Prometheus CRD is available
			// We'll check this at runtime in the InitWithManager
			return builder
		},
		InitWithFakeClientBuilder: func(fakeClientBuilder *fake.ClientBuilder) {
			fakeClientBuilder.WithIndex(&monitoringv1.Prometheus{}, CollectableOwnerKey, prometheusIndexer)
		},
	}
	SetupKAIConfigOwned(collectable)
}

func isPrometheusCRDAvailable(ctx context.Context, client client.Client) bool {
	crd := &metav1.PartialObjectMetadata{
		TypeMeta: metav1.TypeMeta{
			Kind:       "CustomResourceDefinition",
			APIVersion: "apiextensions.k8s.io/v1",
		},
	}

	err := client.Get(ctx, types.NamespacedName{
		Name: "prometheuses.monitoring.coreos.com",
	}, crd)

	return err == nil
}

func getCurrentPrometheusState(ctx context.Context, runtimeClient client.Client, reconciler client.Object) (map[string]client.Object, error) {
	result := map[string]client.Object{}

	// Check if Prometheus CRD is available before trying to list resources
	if !isPrometheusCRDAvailable(ctx, runtimeClient) {
		return result, nil
	}

	prometheusList := &monitoringv1.PrometheusList{}
	reconcilerKey := getReconcilerKey(reconciler)

	// Try to list with field selector first, but fall back to listing all if field indexer is not available
	err := runtimeClient.List(ctx, prometheusList, client.MatchingFields{CollectableOwnerKey: reconcilerKey})
	if err != nil {
		// If field indexer is not available, fall back to listing all Prometheus resources
		// and filter by owner reference manually
		log.FromContext(ctx).Info("Field indexer not available, falling back to manual filtering")
		err = runtimeClient.List(ctx, prometheusList)
		if err != nil {
			return nil, err
		}

		// Filter by owner reference manually
		for _, prometheus := range prometheusList.Items {
			owner := metav1.GetControllerOf(&prometheus)
			if owner != nil && checkOwnerType(owner) && getOwnerKey(owner) == reconcilerKey {
				result[GetKey(prometheus.GroupVersionKind(), prometheus.Namespace, prometheus.Name)] = &prometheus
			}
		}
		return result, nil
	}

	for _, prometheus := range prometheusList.Items {
		result[GetKey(prometheus.GroupVersionKind(), prometheus.Namespace, prometheus.Name)] = &prometheus
	}

	return result, nil
}
