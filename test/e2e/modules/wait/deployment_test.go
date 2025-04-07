/*
Copyright 2025 NVIDIA CORPORATION
SPDX-License-Identifier: Apache-2.0
*/
package wait

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestPatchDeploymentArgs(t *testing.T) {
	// Set up test data
	testNamespace := "test-namespace"
	testDeploymentName := "test-deployment"
	testContainerName := "test-container"
	initialArgs := []string{"--initial-arg=value"}

	// Create a deployment with a container that has the initialArgs
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      testDeploymentName,
			Namespace: testNamespace,
		},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": "test"},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": "test"},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  testContainerName,
							Image: "test-image:latest",
							Args:  initialArgs,
						},
					},
				},
			},
		},
	}

	// Create a fake clientset and add the deployment to it
	clientset := fake.NewSimpleClientset(deployment)

	// Define an args updater function that adds a new arg
	newArg := "--new-arg=value"
	argsUpdater := func(args []string) []string {
		return append(args, newArg)
	}

	// Call the function under test
	err := PatchDeploymentArgs(
		context.TODO(),
		clientset,
		testNamespace,
		testDeploymentName,
		testContainerName,
		argsUpdater,
	)

	// Assert there was no error
	assert.NoError(t, err)

	// Get the updated deployment
	updatedDeployment, err := clientset.AppsV1().Deployments(testNamespace).Get(
		context.TODO(),
		testDeploymentName,
		metav1.GetOptions{},
	)
	assert.NoError(t, err)

	// Find the container and check its args
	var foundContainer bool
	for _, container := range updatedDeployment.Spec.Template.Spec.Containers {
		if container.Name == testContainerName {
			foundContainer = true

			// Verify the original arg is still there
			assert.Contains(t, container.Args, initialArgs[0])

			// Verify the new arg was added
			assert.Contains(t, container.Args, newArg)

			// Verify the length is correct (should be initial + 1)
			assert.Len(t, container.Args, len(initialArgs)+1)

			break
		}
	}

	// Verify the container was found
	assert.True(t, foundContainer, "Container was not found in updated deployment")
}

func TestPatchDeploymentArgs_ContainerNotFound(t *testing.T) {
	// Set up test data
	testNamespace := "test-namespace"
	testDeploymentName := "test-deployment"

	// Create a deployment with a different container name
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      testDeploymentName,
			Namespace: testNamespace,
		},
		Spec: appsv1.DeploymentSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name: "different-container",
							Args: []string{"--arg=value"},
						},
					},
				},
			},
		},
	}

	// Create a fake clientset and add the deployment to it
	clientset := fake.NewSimpleClientset(deployment)

	// Define a simple args updater function
	argsUpdater := func(args []string) []string {
		return append(args, "--new-arg=value")
	}

	// Call the function under test with a container name that doesn't exist
	err := PatchDeploymentArgs(
		context.TODO(),
		clientset,
		testNamespace,
		testDeploymentName,
		"non-existent-container",
		argsUpdater,
	)

	// Assert there was an error and it contains the expected message
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "container non-existent-container not found in deployment test-deployment")
}
