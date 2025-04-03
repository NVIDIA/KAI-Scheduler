/*
Copyright 2025 NVIDIA CORPORATION
SPDX-License-Identifier: Apache-2.0
*/
package wait

import (
	"context"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"

	"github.com/NVIDIA/KAI-scheduler/test/e2e/modules/wait/watcher"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"
	runtimeClient "sigs.k8s.io/controller-runtime/pkg/client"
)

// WaitForDeploymentPodsRunning waits until all pods from a deployment are in the running state
func WaitForDeploymentPodsRunning(ctx context.Context, client runtimeClient.WithWatch, name, namespace string) {
	deployment := &appsv1.Deployment{}
	err := client.Get(ctx, types.NamespacedName{Namespace: namespace, Name: name}, deployment)
	if err != nil {
		Fail(fmt.Sprintf("failed to get deployment %s/%s: %v", namespace, name, err))
	}

	// Get the deployment's selector
	selector := deployment.Spec.Selector
	if selector == nil {
		Fail(fmt.Sprintf("deployment %s/%s has no selector", namespace, name))
	}

	// Get the desired replica count
	var replicas int32 = 1
	if deployment.Spec.Replicas != nil {
		replicas = *deployment.Spec.Replicas
	}

	isTheDeploymentReady := func(event watch.Event) bool {
		podListObj, ok := event.Object.(*v1.PodList)
		if !ok {
			return false
		}

		// Return false if no pods found
		if len(podListObj.Items) == 0 {
			return false
		}

		// Count running pods and check for non-running pods
		runningPods := 0
		for _, pod := range podListObj.Items {
			if pod.Status.Phase == v1.PodRunning {
				runningPods++
			} else {
				// If any pod is not running, return false
				return false
			}
		}

		return int32(runningPods) == replicas
	}

	// Create a watcher for the pods
	pw := watcher.NewGenericWatcher[v1.PodList](client, isTheDeploymentReady,
		runtimeClient.InNamespace(namespace),
		runtimeClient.MatchingLabels(selector.MatchLabels))

	if !watcher.ForEvent(ctx, client, pw) {
		Fail(fmt.Sprintf("timed out waiting for pods of deployment %s/%s to be running", namespace, name))
	}
}
