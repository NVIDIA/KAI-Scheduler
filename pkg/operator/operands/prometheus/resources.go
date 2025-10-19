// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package prometheus

import (
	"context"
	"fmt"

	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	kaiv1 "github.com/NVIDIA/KAI-scheduler/pkg/apis/kai/v1"
	"github.com/NVIDIA/KAI-scheduler/pkg/operator/operands/common"
	v1 "k8s.io/api/core/v1"
)

const (
	mainResourceName = "kai"
)

func prometheusForKAIConfig(
	ctx context.Context, runtimeClient client.Reader, kaiConfig *kaiv1.Config,
) ([]client.Object, error) {
	logger := log.FromContext(ctx)
	config := kaiConfig.Spec.Prometheus

	// Check if Prometheus is enabled
	if config == nil || config.Enabled == nil || !*config.Enabled {
		logger.Info("Prometheus is disabled in configuration")
		return []client.Object{}, nil
	}

	logger.Info("Prometheus is enabled, checking for Prometheus Operator installation")

	// Check if Prometheus Operator is installed by looking for the Prometheus CRD
	// This is a simple check - in production you might want to check for the operator deployment
	hasPrometheusOperator, err := common.CheckPrometheusCRDsAvailable(ctx, runtimeClient, "prometheus")
	if err != nil {
		logger.Error(err, "Failed to check for Prometheus Operator installation")
		return []client.Object{}, err
	}

	// If Prometheus Operator is not installed, we can't create a Prometheus CR
	if !hasPrometheusOperator {
		logger.Info("Prometheus Operator not found - Prometheus CRD is not available")
		return []client.Object{}, nil
	}
	prometheus, err := common.ObjectForKAIConfig(ctx, runtimeClient, &monitoringv1.Prometheus{}, mainResourceName, kaiConfig.Spec.Namespace)
	if err != nil {
		if !errors.IsNotFound(err) {
			logger.Error(err, "Failed to check for existing Prometheus instance")
			return nil, err
		}
	}
	var ok bool
	prometheus, ok = prometheus.(*monitoringv1.Prometheus)
	if !ok {
		logger.Error(nil, "Failed to cast object to Prometheus type")
		return nil, fmt.Errorf("failed to cast object to Prometheus type")
	}

	// Set the Prometheus spec from configuration
	prometheusSpec := monitoringv1.PrometheusSpec{
		// Basic configuration required for Prometheus Operator to create pods
		// Using minimal spec to avoid field name issues
	}

	// Configure TSDB storage
	storageSize, err := config.CalculateStorageSize(ctx, runtimeClient)
	if err != nil {
		logger.Error(err, "Failed to calculate storage size")
		return nil, err
	}
	prometheusSpec.Storage = &monitoringv1.StorageSpec{
		VolumeClaimTemplate: monitoringv1.EmbeddedPersistentVolumeClaim{
			Spec: v1.PersistentVolumeClaimSpec{
				StorageClassName: config.StorageClassName,
				AccessModes:      []v1.PersistentVolumeAccessMode{v1.ReadWriteOnce},
				Resources: v1.VolumeResourceRequirements{
					Requests: v1.ResourceList{
						v1.ResourceStorage: resource.MustParse(storageSize),
					},
				},
			},
		},
	}

	// Set retention period if specified
	if config.RetentionPeriod != nil {
		prometheusSpec.Retention = monitoringv1.Duration(*config.RetentionPeriod)
	}

	// Configure ServiceMonitor selector to match KAI ServiceMonitors
	if config.ServiceMonitor != nil && *config.ServiceMonitor.Enabled {
		prometheusSpec.ServiceMonitorSelector = &metav1.LabelSelector{
			MatchLabels: map[string]string{
				"accounting": mainResourceName,
			},
		}
		prometheusSpec.ServiceMonitorNamespaceSelector = &metav1.LabelSelector{}
	}

	// Set the service account name in the Prometheus spec
	prometheusSpec.ServiceAccountName = mainResourceName + "-prometheus"

	prometheus.(*monitoringv1.Prometheus).Spec = prometheusSpec
	return []client.Object{prometheus}, nil
}

func serviceMonitorsForKAIConfig(
	ctx context.Context, runtimeClient client.Reader, kaiConfig *kaiv1.Config,
) ([]client.Object, error) {
	logger := log.FromContext(ctx)
	config := kaiConfig.Spec.Prometheus

	// Check if ServiceMonitor CRD is available
	hasServiceMonitorCRD, err := common.CheckPrometheusCRDsAvailable(ctx, runtimeClient, "serviceMonitor")
	if err != nil {
		logger.Error(err, "Failed to check for ServiceMonitor CRD")
		return nil, err
	}

	if !hasServiceMonitorCRD {
		logger.Info("ServiceMonitor CRD not found - ServiceMonitor resources cannot be created")
		return []client.Object{}, nil
	}

	var serviceMonitors []client.Object

	// Create ServiceMonitor for each KAI service
	for _, kaiService := range common.KaiServicesForServiceMonitor {
		serviceMonitorObj, err := common.ObjectForKAIConfig(ctx, runtimeClient, &monitoringv1.ServiceMonitor{}, kaiService.Name, kaiConfig.Spec.Namespace)
		if err != nil {
			logger.Error(err, "Failed to check for existing ServiceMonitor instance", "service", kaiService.Name)
			return nil, err
		}

		// Set the ServiceMonitor spec from configuration
		serviceMonitorSpec := monitoringv1.ServiceMonitorSpec{
			JobLabel: kaiService.JobLabel,
			NamespaceSelector: monitoringv1.NamespaceSelector{
				MatchNames: []string{kaiConfig.Spec.Namespace},
			},
			Selector: metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": kaiService.Name,
				},
			},
			Endpoints: []monitoringv1.Endpoint{
				{
					Port: kaiService.Port,
				},
			},
		}

		// Apply ServiceMonitor configuration if available
		if config.ServiceMonitor != nil {
			if config.ServiceMonitor.Interval != nil {
				serviceMonitorSpec.Endpoints[0].Interval = monitoringv1.Duration(*config.ServiceMonitor.Interval)
			}
			if config.ServiceMonitor.ScrapeTimeout != nil {
				serviceMonitorSpec.Endpoints[0].ScrapeTimeout = monitoringv1.Duration(*config.ServiceMonitor.ScrapeTimeout)
			}
			if config.ServiceMonitor.BearerTokenFile != nil {
				serviceMonitorSpec.Endpoints[0].BearerTokenFile = *config.ServiceMonitor.BearerTokenFile
			}
		}
		serviceMonitorObj.(*monitoringv1.ServiceMonitor).Spec = serviceMonitorSpec
		serviceMonitors = append(serviceMonitors, serviceMonitorObj)
	}
	return serviceMonitors, nil
}

// createPrometheusServiceAccount creates a ServiceAccount for Prometheus
func prometheusServiceAccountForKAIConfig(
	ctx context.Context, runtimeClient client.Reader, kaiConfig *kaiv1.Config,
) ([]client.Object, error) {
	serviceAccountName := mainResourceName + "-prometheus"

	// Check if ServiceAccount already exists
	saObj, err := common.ObjectForKAIConfig(ctx, runtimeClient, &v1.ServiceAccount{}, serviceAccountName, kaiConfig.Spec.Namespace)
	if err != nil {
		return []client.Object{}, err
	}

	serviceAccount := saObj.(*v1.ServiceAccount)
	serviceAccount.TypeMeta = metav1.TypeMeta{
		Kind:       "ServiceAccount",
		APIVersion: "v1",
	}
	serviceAccount.ObjectMeta = metav1.ObjectMeta{
		Name:      serviceAccountName,
		Namespace: kaiConfig.Spec.Namespace,
		Labels: map[string]string{
			"app": serviceAccountName,
		},
	}

	return []client.Object{serviceAccount}, nil
}
