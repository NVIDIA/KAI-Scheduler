// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package prometheus

import (
	"context"

	"github.com/NVIDIA/KAI-scheduler/pkg/operator/operands/common"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	kaiv1 "github.com/NVIDIA/KAI-scheduler/pkg/apis/kai/v1"
)

type Prometheus struct {
	namespace        string
	lastDesiredState []client.Object
	client           client.Client
}
type promethuesResourceForKAIConfig func(ctx context.Context, runtimeClient client.Reader, kaiConfig *kaiv1.Config) ([]client.Object, error)

func (p *Prometheus) DesiredState(
	ctx context.Context, runtimeClient client.Reader, kaiConfig *kaiv1.Config,
) ([]client.Object, error) {
	p.namespace = kaiConfig.Spec.Namespace
	p.client = runtimeClient.(client.Client)

	var objects []client.Object
	for _, resourceFunc := range []promethuesResourceForKAIConfig{
		prometheusForKAIConfig,
		prometheusServiceAccountForKAIConfig,
		serviceMonitorsForKAIConfig,
	} {
		obj, err := resourceFunc(ctx, runtimeClient, kaiConfig)
		if err != nil {
			return nil, err
		}
		objects = append(objects, obj...)
	}

	p.lastDesiredState = objects
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
