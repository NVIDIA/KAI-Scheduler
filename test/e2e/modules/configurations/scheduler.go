/*
Copyright 2025 NVIDIA CORPORATION
SPDX-License-Identifier: Apache-2.0
*/
package configurations

import (
	"context"
	"fmt"

	. "github.com/onsi/ginkgo/v2"

	"github.com/NVIDIA/KAI-scheduler/pkg/common/constants"
	"github.com/NVIDIA/KAI-scheduler/test/e2e/modules/constant"
	testcontext "github.com/NVIDIA/KAI-scheduler/test/e2e/modules/context"
	"github.com/NVIDIA/KAI-scheduler/test/e2e/modules/wait"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func DisableScheduler(ctx context.Context, testCtx *testcontext.TestContext) {
	err := wait.UpateDeploymentReplicas(ctx, testCtx.KubeClientset, constant.SystemPodsNamespace, constant.SchedulerDeploymentName, 0)
	if err != nil {
		Fail(fmt.Sprintf("Failed to disable scheduler with 0 replicas: %v", err))
	}

	wait.ForPodsToBeDeleted(
		ctx, testCtx.ControllerClient,
		client.InNamespace(constant.SystemPodsNamespace),
		client.MatchingLabels{constants.AppLabelName: "scheduler"},
	)
}

func EnableScheduler(ctx context.Context, testCtx *testcontext.TestContext) {
	err := wait.UpateDeploymentReplicas(ctx, testCtx.KubeClientset, constant.SystemPodsNamespace, constant.SchedulerDeploymentName, 1)
	if err != nil {
		Fail(fmt.Sprintf("Failed to enable scheduler with 1 replicas: %v", err))
	}

	wait.ForRunningSchedulerPodEvent(ctx, testCtx.ControllerClient, constant.SchedulerDeploymentName)
}
