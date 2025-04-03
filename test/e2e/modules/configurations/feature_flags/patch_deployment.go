/*
Copyright 2025 NVIDIA CORPORATION
SPDX-License-Identifier: Apache-2.0
*/
package feature_flags

import (
	"context"
	"fmt"

	testcontext "github.com/NVIDIA/KAI-scheduler/test/e2e/modules/context"
	"github.com/NVIDIA/KAI-scheduler/test/e2e/modules/wait"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type ArgsUpdater func(args []string) []string

func PatchSystemDeploymentFeatureFlags(
	ctx context.Context,
	tc *testcontext.TestContext,
	namespace string,
	deploymentName string,
	containerName string,
	featureFlagsUpdater ArgsUpdater,
) error {

	err := patchDeploymentArgs(ctx, tc.KubeClientset, namespace, deploymentName, containerName, featureFlagsUpdater)
	if err != nil {
		return fmt.Errorf("failed to patch deployment %s: %w", deploymentName, err)
	}
	wait.WaitForDeploymentPodsRunning(ctx, tc.ControllerClient, deploymentName, namespace)

	return nil
}

func patchDeploymentArgs(
	ctx context.Context,
	clientset kubernetes.Interface,
	namespace string,
	deploymentName string,
	containerName string,
	argsUpdater ArgsUpdater,
) error {
	// Get the deployment
	deployment, err := clientset.AppsV1().Deployments(namespace).Get(
		ctx,
		deploymentName,
		metav1.GetOptions{},
	)
	if err != nil {
		return fmt.Errorf("failed to get deployment %s: %v", deploymentName, err)
	}

	// Create a copy to use as the base for our patch
	deploymentCopy := deployment.DeepCopy()

	// Find and update the container args in our copy
	containerFound := false
	for i, container := range deploymentCopy.Spec.Template.Spec.Containers {
		if container.Name == containerName {
			deploymentCopy.Spec.Template.Spec.Containers[i].Args = argsUpdater(container.Args)
			containerFound = true
			break
		}
	}

	if !containerFound {
		return fmt.Errorf("container %s not found in deployment %s", containerName, deploymentName)
	}

	// Create the patch data
	patchBytes, err := client.MergeFrom(deployment).Data(deploymentCopy)
	if err != nil {
		return fmt.Errorf("failed to create patch data: %v", err)
	}

	// Apply the patch
	_, err = clientset.AppsV1().Deployments(namespace).Patch(
		ctx,
		deploymentName,
		types.MergePatchType,
		patchBytes,
		metav1.PatchOptions{},
	)
	if err != nil {
		return fmt.Errorf("failed to patch deployment %s: %w", deploymentName, err)
	}

	return nil
}
