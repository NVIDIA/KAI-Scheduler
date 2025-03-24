/*
Copyright 2025 NVIDIA CORPORATION
SPDX-License-Identifier: Apache-2.0
*/
package environment

import (
	"context"

	v1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var isFakeGpuEnv *bool

func HasFakeGPUNodes(ctx context.Context, k8sClient client.Reader) (bool, error) {
	if isFakeGpuEnv != nil {
		return *isFakeGpuEnv, nil
	}

	var nodes v1.NodeList

	err := k8sClient.List(
		ctx, &nodes,
		client.MatchingLabels{
			"run.ai/fake.gpu": "true",
		})
	if err != nil {
		return false, err
	}

	return len(nodes.Items) > 0, nil
}
