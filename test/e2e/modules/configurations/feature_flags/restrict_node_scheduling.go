/*
Copyright 2025 NVIDIA CORPORATION
SPDX-License-Identifier: Apache-2.0
*/
package feature_flags

import (
	"context"
	"fmt"

	"github.com/NVIDIA/KAI-scheduler/pkg/common/constants"
	"github.com/NVIDIA/KAI-scheduler/test/e2e/modules/configurations"
	"github.com/NVIDIA/KAI-scheduler/test/e2e/modules/constant"
	testContext "github.com/NVIDIA/KAI-scheduler/test/e2e/modules/context"
	"github.com/NVIDIA/KAI-scheduler/test/e2e/modules/wait"
	"k8s.io/utils/ptr"
)

func SetRestrictNodeScheduling(
	value *bool, testCtx *testContext.TestContext, ctx context.Context,
) error {
	var targetValue *string = nil
	if value != nil {
		targetValue = ptr.To(fmt.Sprint(*value))
	}
	if err := configurations.SetShardArg(ctx, testCtx, "default", "restrict-node-scheduling", targetValue); err != nil {
		return err
	}
	wait.WaitForDeploymentPodsRunning(ctx, testCtx.ControllerClient, constant.SchedulerDeploymentName, constants.DefaultKAINamespace)
	return nil
}
