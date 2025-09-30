// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package prometheus

import (
	"context"

	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	kaiv1 "github.com/NVIDIA/KAI-scheduler/pkg/apis/kai/v1"
	"github.com/NVIDIA/KAI-scheduler/pkg/operator/operands/common"
	v1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
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
	hasPrometheusOperator, err := CheckPrometheusOperatorInstalled(ctx, runtimeClient)
	if err != nil {
		logger.Error(err, "Failed to check for Prometheus Operator installation")
		return nil, err
	}

	// If Prometheus Operator is not installed, we can't create a Prometheus CR
	if !hasPrometheusOperator {
		logger.Info("Prometheus Operator not found - Prometheus CRD is not available")
		return []client.Object{}, nil
	}

	// Create Prometheus CR
	prometheus := &monitoringv1.Prometheus{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Prometheus",
			APIVersion: "monitoring.coreos.com/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      mainResourceName,
			Namespace: kaiConfig.Spec.Namespace,
			Labels: map[string]string{
				"app": mainResourceName,
			},
		},
	}

	// Check if Prometheus already exists
	prom, err := common.ObjectForKAIConfig(ctx, runtimeClient, prometheus, mainResourceName, kaiConfig.Spec.Namespace)
	if err != nil {
		logger.Error(err, "Failed to check for existing Prometheus instance")
		return nil, err
	}
	prometheus = prom.(*monitoringv1.Prometheus)

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
	if config.ServiceMonitor != nil && config.ServiceMonitor.Enabled != nil && *config.ServiceMonitor.Enabled {
		prometheusSpec.ServiceMonitorSelector = &metav1.LabelSelector{
			MatchLabels: map[string]string{
				"accounting": mainResourceName,
			},
		}
		prometheusSpec.ServiceMonitorNamespaceSelector = &metav1.LabelSelector{}
	}

	prometheus.Spec = prometheusSpec

	var objects []client.Object

	// Create ServiceAccount for Prometheus
	serviceAccount, err := serviceAccountForKAIConfig(ctx, runtimeClient, kaiConfig)
	if err != nil {
		logger.Error(err, "Failed to create ServiceAccount for Prometheus")
		return nil, err
	}
	objects = append(objects, serviceAccount)

	// Create ClusterRole for Prometheus
	clusterRole, err := clusterRoleForKAIConfig(ctx, runtimeClient, kaiConfig)
	if err != nil {
		logger.Error(err, "Failed to create ClusterRole for Prometheus")
		return nil, err
	}
	objects = append(objects, clusterRole)

	// Create ClusterRoleBinding for Prometheus
	clusterRoleBinding, err := clusterRoleBindingForKAIConfig(ctx, runtimeClient, kaiConfig)
	if err != nil {
		logger.Error(err, "Failed to create ClusterRoleBinding for Prometheus")
		return nil, err
	}
	objects = append(objects, clusterRoleBinding)

	// Set ServiceAccountName in Prometheus spec
	prometheus.Spec.ServiceAccountName = mainResourceName

	objects = append(objects, prometheus)

	// Create ServiceMonitors if enabled
	if config.ServiceMonitor != nil && config.ServiceMonitor.Enabled != nil && *config.ServiceMonitor.Enabled {
		serviceMonitors, err := serviceMonitorsForKAIConfig(ctx, runtimeClient, kaiConfig)
		if err != nil {
			logger.Error(err, "Failed to create ServiceMonitor instances")
			return nil, err
		}
		objects = append(objects, serviceMonitors...)
	}

	return objects, nil
}

func CheckPrometheusOperatorInstalled(ctx context.Context, runtimeClient client.Reader) (bool, error) {
	logger := log.FromContext(ctx)

	// Check if the Prometheus CRD exists	// This is a simple way to check if the Prometheus Operator is installed
	crd := &metav1.PartialObjectMetadata{
		TypeMeta: metav1.TypeMeta{
			Kind:       "CustomResourceDefinition",
			APIVersion: "apiextensions.k8s.io/v1",
		},
	}

	err := runtimeClient.Get(ctx, types.NamespacedName{
		Name: "prometheuses.monitoring.coreos.com",
	}, crd)

	if err != nil {
		if errors.IsNotFound(err) {
			logger.Info("Prometheus CRD not found", "crd", "prometheuses.monitoring.coreos.com")
			return false, nil
		}
		logger.Error(err, "Failed to check for Prometheus CRD", "crd", "prometheuses.monitoring.coreos.com")
		return false, err
	}

	logger.Info("Prometheus CRD found", "crd", "prometheuses.monitoring.coreos.com")
	return true, nil
}

// KAI services that should be monitored
var kaiServices = []struct {
	name     string
	port     string
	jobLabel string
}{
	{"binder", "http-metrics", "binder"},
	{"scheduler", "http-metrics", "scheduler"},
	{"queuecontroller", "metrics", "queuecontroller"},
	{"podgrouper", "metrics", "podgrouper"},
	{"podgroupcontroller", "metrics", "podgroupcontroller"},
	{"admission", "metrics", "admission"},
	{"nodescaleadjuster", "metrics", "nodescaleadjuster"},
}

func serviceMonitorsForKAIConfig(
	ctx context.Context, runtimeClient client.Reader, kaiConfig *kaiv1.Config,
) ([]client.Object, error) {
	logger := log.FromContext(ctx)
	config := kaiConfig.Spec.Prometheus

	// Check if ServiceMonitor CRD is available
	hasServiceMonitorCRD, err := checkServiceMonitorCRDAvailable(ctx, runtimeClient)
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
	for _, service := range kaiServices {
		serviceMonitor := &monitoringv1.ServiceMonitor{
			TypeMeta: metav1.TypeMeta{
				Kind:       "ServiceMonitor",
				APIVersion: "monitoring.coreos.com/v1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      service.name,
				Namespace: kaiConfig.Spec.Namespace,
				Labels: map[string]string{
					"app":        mainResourceName,
					"accounting": mainResourceName,
				},
			},
		}

		// Check if ServiceMonitor already exists
		existingSM, err := common.ObjectForKAIConfig(ctx, runtimeClient, serviceMonitor, service.name, kaiConfig.Spec.Namespace)
		if err != nil {
			logger.Error(err, "Failed to check for existing ServiceMonitor instance", "service", service.name)
			return nil, err
		}
		serviceMonitor = existingSM.(*monitoringv1.ServiceMonitor)

		// Set the ServiceMonitor spec from configuration
		serviceMonitorSpec := monitoringv1.ServiceMonitorSpec{
			JobLabel: service.jobLabel,
			NamespaceSelector: monitoringv1.NamespaceSelector{
				MatchNames: []string{kaiConfig.Spec.Namespace},
			},
			Selector: metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": service.name,
				},
			},
			Endpoints: []monitoringv1.Endpoint{
				{
					Port: service.port,
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

		serviceMonitor.Spec = serviceMonitorSpec

		serviceMonitors = append(serviceMonitors, serviceMonitor)
	}

	return serviceMonitors, nil
}

func checkServiceMonitorCRDAvailable(ctx context.Context, runtimeClient client.Reader) (bool, error) {
	// Check if ServiceMonitor CRD exists
	crd := &metav1.PartialObjectMetadata{
		TypeMeta: metav1.TypeMeta{
			Kind:       "CustomResourceDefinition",
			APIVersion: "apiextensions.k8s.io/v1",
		},
	}

	err := runtimeClient.Get(ctx, types.NamespacedName{
		Name: "servicemonitors.monitoring.coreos.com",
	}, crd)

	if err != nil {
		if errors.IsNotFound(err) {
			return false, nil
		}
		return false, err
	}

	return true, nil
}

func serviceAccountForKAIConfig(
	ctx context.Context, runtimeClient client.Reader, kaiConfig *kaiv1.Config,
) (client.Object, error) {
	sa, err := common.ObjectForKAIConfig(ctx, runtimeClient, &v1.ServiceAccount{}, mainResourceName, kaiConfig.Spec.Namespace)
	if err != nil {
		return nil, err
	}
	sa.(*v1.ServiceAccount).TypeMeta = metav1.TypeMeta{
		Kind:       "ServiceAccount",
		APIVersion: "v1",
	}
	return sa, err
}

func clusterRoleForKAIConfig(
	ctx context.Context, runtimeClient client.Reader, kaiConfig *kaiv1.Config,
) (client.Object, error) {
	clusterRole, err := common.ObjectForKAIConfig(ctx, runtimeClient, &rbacv1.ClusterRole{}, mainResourceName, "")
	if err != nil {
		return nil, err
	}
	cr := clusterRole.(*rbacv1.ClusterRole)
	cr.TypeMeta = metav1.TypeMeta{
		Kind:       "ClusterRole",
		APIVersion: "rbac.authorization.k8s.io/v1",
	}
	cr.Rules = []rbacv1.PolicyRule{
		{
			APIGroups: []string{""},
			Resources: []string{"nodes", "nodes/proxy", "services", "endpoints", "pods"},
			Verbs:     []string{"get", "list", "watch"},
		},
		{
			APIGroups: []string{""},
			Resources: []string{"configmaps"},
			Verbs:     []string{"get"},
		},
	}
	return cr, nil
}

func clusterRoleBindingForKAIConfig(
	ctx context.Context, runtimeClient client.Reader, kaiConfig *kaiv1.Config,
) (client.Object, error) {
	clusterRoleBinding, err := common.ObjectForKAIConfig(ctx, runtimeClient, &rbacv1.ClusterRoleBinding{}, mainResourceName, "")
	if err != nil {
		return nil, err
	}
	crb := clusterRoleBinding.(*rbacv1.ClusterRoleBinding)
	crb.TypeMeta = metav1.TypeMeta{
		Kind:       "ClusterRoleBinding",
		APIVersion: "rbac.authorization.k8s.io/v1",
	}
	crb.RoleRef = rbacv1.RoleRef{
		APIGroup: "rbac.authorization.k8s.io",
		Kind:     "ClusterRole",
		Name:     mainResourceName,
	}
	crb.Subjects = []rbacv1.Subject{
		{
			Kind:      "ServiceAccount",
			Name:      mainResourceName,
			Namespace: kaiConfig.Spec.Namespace,
		},
	}
	return crb, nil
}
