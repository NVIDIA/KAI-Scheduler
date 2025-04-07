/*
Copyright 2025 NVIDIA CORPORATION
SPDX-License-Identifier: Apache-2.0
*/
package feature_flags

import (
	"context"
	"fmt"
	"strings"

	"github.com/NVIDIA/KAI-scheduler/test/e2e/modules/constant"
	testcontext "github.com/NVIDIA/KAI-scheduler/test/e2e/modules/context"
	"github.com/NVIDIA/KAI-scheduler/test/e2e/modules/wait"
)

func SetFullHierarchyFairness(
	ctx context.Context, testCtx *testcontext.TestContext, value *bool,
) error {
	return wait.PatchSystemDeploymentFeatureFlags(
		ctx,
		testCtx.KubeClientset,
		testCtx.ControllerClient,
		constant.SystemPodsNamespace,
		constant.SchedulerDeploymentName,
		constant.SchedulerContainerName,
		func(args []string) []string {
			if value != nil {
				return append(args, fmt.Sprintf("--full-hierarchy-fairness=%t", *value))
			}
			for i, arg := range args {
				if strings.HasPrefix(arg, "--full-hierarchy-fairness=") {
					return append(args[:i], args[i+1:]...)
				}
			}
			return args
		},
	)

}
