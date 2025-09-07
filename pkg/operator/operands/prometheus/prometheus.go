// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package prometheus

import (
	"context"

	"github.com/NVIDIA/KAI-scheduler/pkg/operator/operands/common"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	kaiv1 "github.com/NVIDIA/KAI-scheduler/pkg/apis/kai/v1"
)

type Prometheus struct {
	namespace        string
	lastDesiredState []client.Object
}

func (p *Prometheus) DesiredState(
	ctx context.Context, runtimeClient client.Reader, kaiConfig *kaiv1.Config,
) ([]client.Object, error) {
	p.namespace = kaiConfig.Spec.Namespace

	objects, err := prometheusForKAIConfig(ctx, runtimeClient, kaiConfig)
	if err != nil {
		log.FromContext(ctx).Error(err, "Failed to create Prometheus instance")
		return nil, err
	}

	p.lastDesiredState = objects
	return objects, nil
}

func (b *Prometheus) IsDeployed(ctx context.Context, readerClient client.Reader) (bool, error) {
	return common.AllObjectsExists(ctx, readerClient, b.lastDesiredState)
}

func (b *Prometheus) IsAvailable(ctx context.Context, readerClient client.Reader) (bool, error) {
	return common.AllControllersAvailable(ctx, readerClient, b.lastDesiredState)
}

func (b *Prometheus) Name() string {
	return "KAI-prometheus"
}
