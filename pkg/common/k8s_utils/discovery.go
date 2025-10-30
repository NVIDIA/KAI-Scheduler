// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package k8s_utils

import (
	"context"
	"fmt"

	ocpconf "github.com/openshift/api/config/v1"
	nodev1 "k8s.io/api/node/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// RuntimeClassExists checks if a RuntimeClass with the given name exists in the cluster
func RuntimeClassExists(ctx context.Context, client client.Client, name string) (bool, error) {
	runtimeClass := &nodev1.RuntimeClass{}
	err := client.Get(ctx, types.NamespacedName{Name: name}, runtimeClass)
	if err != nil {
		if errors.IsNotFound(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func IsOpenshift(ctx context.Context, c client.Reader) (bool, error) {
	cv := &ocpconf.ClusterVersionList{}
	err := c.List(ctx, cv, &client.ListOptions{})

	if meta.IsNoMatchError(err) {
		return false, nil
	}

	if err != nil {
		return false, fmt.Errorf("failed to determine if running on openshift or not: %w", err)
	}

	if len(cv.Items) == 0 {
		return false, nil
	}

	return true, nil
}
