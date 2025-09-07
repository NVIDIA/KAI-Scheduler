// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package node_scale_adjuster

import (
	"context"

	"sigs.k8s.io/controller-runtime/pkg/client"

	kaiv1 "github.com/NVIDIA/KAI-scheduler/pkg/apis/kai/v1"
	"github.com/NVIDIA/KAI-scheduler/pkg/operator/operands/common"
)

type NodeScaleAdjuster struct {
	namespace        string
	lastDesiredState []client.Object
}

type resourceForKAIConfig func(ctx context.Context, runtimeClient client.Reader, kaiConfig *kaiv1.Config) (client.Object, error)

func (nsa *NodeScaleAdjuster) DesiredState(
	ctx context.Context, runtimeClient client.Reader, kaiConfig *kaiv1.Config,
) ([]client.Object, error) {
	nsa.namespace = kaiConfig.Spec.Namespace

	if *kaiConfig.Spec.NodeScaleAdjuster.Service.Enabled == false {
		nsa.lastDesiredState = []client.Object{}
		return nil, nil
	}

	var objects []client.Object
	for _, resourceFunc := range []resourceForKAIConfig{
		deploymentForKAIConfig,
		serviceAccountForKAIConfig,
		scalingPodServiceAccountForKAIConfig,
	} {
		obj, err := resourceFunc(ctx, runtimeClient, kaiConfig)
		if err != nil {
			return nil, err
		}
		objects = append(objects, obj)
	}

	nsa.lastDesiredState = objects
	return objects, nil
}

func (nsa *NodeScaleAdjuster) IsDeployed(ctx context.Context, readerClient client.Reader) (bool, error) {
	return common.AllObjectsExists(ctx, readerClient, nsa.lastDesiredState)
}

func (nsa *NodeScaleAdjuster) IsAvailable(ctx context.Context, readerClient client.Reader) (bool, error) {
	return common.AllControllersAvailable(ctx, readerClient, nsa.lastDesiredState)
}

func (nsa *NodeScaleAdjuster) Name() string {
	return "NodeScaleAdjuster"
}
