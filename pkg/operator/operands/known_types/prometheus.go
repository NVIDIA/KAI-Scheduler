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

	// Register ServiceMonitor collectable if CRD is available
	serviceMonitorCollectable := &Collectable{
		Collect: getCurrentServiceMonitorState,
		InitWithManager: func(ctx context.Context, mgr manager.Manager) error {
			// Check if ServiceMonitor CRD exists before registering the indexer
			if !isServiceMonitorCRDAvailable(ctx, mgr.GetClient()) {
				log.FromContext(ctx).Info("ServiceMonitor CRD not available, skipping ServiceMonitor resource management")
				return nil
			}
			log.FromContext(ctx).Info("ServiceMonitor CRD available, registering ServiceMonitor resource management")
			return mgr.GetFieldIndexer().IndexField(ctx, &monitoringv1.ServiceMonitor{}, CollectableOwnerKey, serviceMonitorIndexer)
		},
		InitWithBuilder: func(builder *builder.Builder) *builder.Builder {
			// Only register the watch if ServiceMonitor CRD is available
			// We'll check this at runtime in the InitWithManager
			return builder
		},
		InitWithFakeClientBuilder: func(fakeClientBuilder *fake.ClientBuilder) {
			fakeClientBuilder.WithIndex(&monitoringv1.ServiceMonitor{}, CollectableOwnerKey, serviceMonitorIndexer)
		},
	}
	SetupKAIConfigOwned(serviceMonitorCollectable)
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

func serviceMonitorIndexer(object client.Object) []string {
	serviceMonitor := object.(*monitoringv1.ServiceMonitor)
	owner := metav1.GetControllerOf(serviceMonitor)
	if !checkOwnerType(owner) {
		return nil
	}
	return []string{getOwnerKey(owner)}
}

func isServiceMonitorCRDAvailable(ctx context.Context, client client.Client) bool {
	crd := &metav1.PartialObjectMetadata{
		TypeMeta: metav1.TypeMeta{
			Kind:       "CustomResourceDefinition",
			APIVersion: "apiextensions.k8s.io/v1",
		},
	}

	err := client.Get(ctx, types.NamespacedName{
		Name: "servicemonitors.monitoring.coreos.com",
	}, crd)

	return err == nil
}

func getCurrentServiceMonitorState(ctx context.Context, runtimeClient client.Client, reconciler client.Object) (map[string]client.Object, error) {
	result := map[string]client.Object{}

	// Check if ServiceMonitor CRD is available before trying to list resources
	if !isServiceMonitorCRDAvailable(ctx, runtimeClient) {
		return result, nil
	}

	serviceMonitorList := &monitoringv1.ServiceMonitorList{}
	reconcilerKey := getReconcilerKey(reconciler)

	// Try to list with field selector first, but fall back to listing all if field indexer is not available
	err := runtimeClient.List(ctx, serviceMonitorList, client.MatchingFields{CollectableOwnerKey: reconcilerKey})
	if err != nil {
		// If field indexer is not available, fall back to listing all ServiceMonitor resources
		// and filter by owner reference manually
		log.FromContext(ctx).Info("Field indexer not available, falling back to manual filtering")
		err = runtimeClient.List(ctx, serviceMonitorList)
		if err != nil {
			return nil, err
		}

		// Filter by owner reference manually
		for _, serviceMonitor := range serviceMonitorList.Items {
			owner := metav1.GetControllerOf(&serviceMonitor)
			if owner != nil && checkOwnerType(owner) && getOwnerKey(owner) == reconcilerKey {
				result[GetKey(serviceMonitor.GroupVersionKind(), serviceMonitor.Namespace, serviceMonitor.Name)] = &serviceMonitor
			}
		}
		return result, nil
	}

	for _, serviceMonitor := range serviceMonitorList.Items {
		result[GetKey(serviceMonitor.GroupVersionKind(), serviceMonitor.Namespace, serviceMonitor.Name)] = &serviceMonitor
	}

	return result, nil
}
