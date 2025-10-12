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

	// Configure TSDB storage if TSDB is configured
	if config != nil {
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
	}

	prometheus.Spec = prometheusSpec

	return []client.Object{prometheus}, nil
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
