/*
Copyright 2025 NVIDIA CORPORATION
SPDX-License-Identifier: Apache-2.0
*/
package wait

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	kubernetes "k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func UpateDeploymentReplicas(
	ctx context.Context,
	clientset kubernetes.Interface,
	namespace string,
	name string,
	replicas int32,
) error {
	// Get the deployment
	deployment, err := clientset.AppsV1().Deployments(namespace).Get(
		ctx,
		name,
		metav1.GetOptions{},
	)
	if err != nil {
		return fmt.Errorf("failed to get deployment %s: %v", name, err)
	}

	// Chceck if we need to actually update the deployment
	if deployment.Spec.Replicas != nil && *deployment.Spec.Replicas == replicas {
		return nil
	}

	// Create a copy to use as the base for our patch
	deploymentCopy := deployment.DeepCopy()

	deploymentCopy.Spec.Replicas = &replicas

	// Create the patch data
	patchBytes, err := client.MergeFrom(deployment).Data(deploymentCopy)
	if err != nil {
		return fmt.Errorf("failed to create patch data: %v", err)
	}

	// Apply the patch
	_, err = clientset.AppsV1().Deployments(namespace).Patch(
		ctx,
		name,
		types.MergePatchType,
		patchBytes,
		metav1.PatchOptions{},
	)
	if err != nil {
		return fmt.Errorf("failed to patch deployment %s: %w", name, err)
	}

	return nil
}
