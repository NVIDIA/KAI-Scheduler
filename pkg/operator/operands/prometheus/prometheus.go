// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package prometheus

import (
	"context"
	"fmt"

	"github.com/NVIDIA/KAI-scheduler/pkg/operator/operands/common"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	kaiv1 "github.com/NVIDIA/KAI-scheduler/pkg/apis/kai/v1"
)

type Prometheus struct {
	namespace        string
	lastDesiredState []client.Object
	client           client.Client
}

func (p *Prometheus) DesiredState(
	ctx context.Context, runtimeClient client.Reader, kaiConfig *kaiv1.Config,
) ([]client.Object, error) {
	p.namespace = kaiConfig.Spec.Namespace
	p.client = runtimeClient.(client.Client)

	objects, err := prometheusForKAIConfig(ctx, runtimeClient, kaiConfig)
	if err != nil {
		log.FromContext(ctx).Error(err, "Failed to create Prometheus instance")
		return nil, err
	}

	p.lastDesiredState = objects
	// Only check for Prometheus operator if we actually have objects to deploy
	if len(objects) > 0 {
		err = p.GetPrometheusOperatorCondition(ctx, runtimeClient, 0)
		if err != nil {
			return nil, err
		}
	}

	return objects, nil
}

func (b *Prometheus) IsDeployed(ctx context.Context, readerClient client.Reader) (bool, error) {
	// If there are no objects to check, consider it deployed
	if len(b.lastDesiredState) == 0 {
		return true, nil
	}
	return common.AllObjectsExists(ctx, readerClient, b.lastDesiredState)
}

func (b *Prometheus) IsAvailable(ctx context.Context, readerClient client.Reader) (bool, error) {
	// If there are no objects to check, consider it available
	if len(b.lastDesiredState) == 0 {
		return true, nil
	}

	prometheus := &monitoringv1.Prometheus{}
	err := readerClient.Get(ctx, client.ObjectKey{
		Name:      mainResourceName,
		Namespace: b.namespace,
	}, prometheus)
	if err != nil {
		return false, err
	}

	// Check if there are any conditions and if the first one is Available
	if len(prometheus.Status.Conditions) > 0 {
		for _, condition := range prometheus.Status.Conditions {
			if condition.Type == monitoringv1.ConditionType("Available") {
				return condition.Status == monitoringv1.ConditionTrue, nil
			}
		}
	}
	return false, nil
}

func (b *Prometheus) Name() string {
	return "KAI-prometheus"
}

func (p *Prometheus) GetPrometheusOperatorCondition(ctx context.Context, readerClient client.Reader, gen int64) error {
	// Check if Prometheus operator is installed by looking for the CRD
	// returns an error if the CRD is not found to be handled by DeployableOperands
	crd := &metav1.PartialObjectMetadata{
		TypeMeta: metav1.TypeMeta{
			Kind:       "CustomResourceDefinition",
			APIVersion: "apiextensions.k8s.io/v1",
		},
	}

	err := readerClient.Get(ctx, client.ObjectKey{
		Name: "prometheuses.monitoring.coreos.com",
	}, crd)

	if err != nil {
		if client.IgnoreNotFound(err) != nil {
			return fmt.Errorf("failed to check for Prometheus Operator installation: %s", err.Error())
		}
		return fmt.Errorf("failed to check for Prometheus Operator installation: %s", err.Error())
	}

	return nil
}
