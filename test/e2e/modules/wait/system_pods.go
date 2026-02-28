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
	"k8s.io/apimachinery/pkg/watch"
	runtimeClient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/NVIDIA/KAI-scheduler/pkg/common/constants"
	"github.com/NVIDIA/KAI-scheduler/test/e2e/modules/resources/rd"
	"github.com/NVIDIA/KAI-scheduler/test/e2e/modules/testconfig"
	"github.com/NVIDIA/KAI-scheduler/test/e2e/modules/wait/watcher"
)

func ForKaiComponentPod(
	ctx context.Context, client runtimeClient.WithWatch,
	appLabelComponentName string, condition checkCondition,
) {
	pw := watcher.NewGenericWatcher[v1.PodList](client, watcher.CheckCondition(condition),
		runtimeClient.InNamespace(testconfig.GetConfig().SystemPodsNamespace),
		runtimeClient.MatchingLabels{constants.AppLabelName: appLabelComponentName})

	if !watcher.ForEvent(ctx, client, pw) {
		Fail(fmt.Sprintf("Failed to wait for %s pod", appLabelComponentName))
	}
}

func ForRunningSystemComponentEvent(ctx context.Context, client runtimeClient.WithWatch, appLabelComponentName string) {
	expectedReplicas := getExpectedReplicaCount(ctx, client, appLabelComponentName)
	runningCondition := func(event watch.Event) bool {
		podListObj, ok := event.Object.(*v1.PodList)
		if !ok {
			Fail(fmt.Sprintf("Failed to process event for pod %s", event.Object))
		}

		if int32(len(podListObj.Items)) != expectedReplicas {
			return false
		}

		for i := range podListObj.Items {
			if !rd.IsPodRunning(&podListObj.Items[i]) {
				return false
			}
		}
		return true
	}
	ForKaiComponentPod(ctx, client, appLabelComponentName, runningCondition)
}

func getExpectedReplicaCount(ctx context.Context, client runtimeClient.WithWatch, appLabelComponentName string) int32 {
	deploymentList := &appsv1.DeploymentList{}
	listOpts := []runtimeClient.ListOption{
		runtimeClient.InNamespace(testconfig.GetConfig().SystemPodsNamespace),
		runtimeClient.MatchingLabels{constants.AppLabelName: appLabelComponentName},
	}
	if err := client.List(ctx, deploymentList, listOpts...); err != nil || len(deploymentList.Items) == 0 {
		return 1
	}
	if deploymentList.Items[0].Spec.Replicas != nil {
		return *deploymentList.Items[0].Spec.Replicas
	}
	return 1
}
