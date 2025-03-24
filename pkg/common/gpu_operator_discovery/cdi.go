// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package gpu_operator_discovery

import (
	"context"
	"fmt"

	nvidiav1 "github.com/NVIDIA/gpu-operator/api/nvidia/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

func IsCdiEnabled(ctx context.Context, readerClient client.Reader) (bool, error) {
	nvidiaClusterPolicies := &nvidiav1.ClusterPolicyList{}
	err := readerClient.List(ctx, nvidiaClusterPolicies)
	if err != nil {
		if meta.IsNoMatchError(err) || kerrors.IsNotFound(err) {
			return false, nil
		}
		log := logf.FromContext(ctx)
		log.Error(err, "cannot list nvidia cluster policy")
		return false, err
	}

	if len(nvidiaClusterPolicies.Items) == 0 {
		return false, nil
	}
	if len(nvidiaClusterPolicies.Items) > 1 {
		log := logf.FromContext(ctx)
		log.Info(fmt.Sprintf("Cluster has %d clusterpolicies.nvidia.com/v1 objects."+
			" First one is queried for the cdi configuration", len(nvidiaClusterPolicies.Items)))
	}

	nvidiaClusterPolicy := nvidiaClusterPolicies.Items[0]
	if nvidiaClusterPolicy.Spec.CDI.Enabled != nil && *nvidiaClusterPolicy.Spec.CDI.Enabled {
		if nvidiaClusterPolicy.Spec.CDI.Default != nil && *nvidiaClusterPolicy.Spec.CDI.Default {
			return true, nil
		}
	}

	return false, nil
}
