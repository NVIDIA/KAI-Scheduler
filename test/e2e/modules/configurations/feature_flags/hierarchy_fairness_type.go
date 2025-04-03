/*
Copyright 2025 NVIDIA CORPORATION
SPDX-License-Identifier: Apache-2.0
*/
package feature_flags

import (
	"context"
	"fmt"

	"github.com/NVIDIA/KAI-scheduler/test/e2e/modules/constant"
	testcontext "github.com/NVIDIA/KAI-scheduler/test/e2e/modules/context"
)

func SetFullHierarchyFairness(
	ctx context.Context, testCtx *testcontext.TestContext, value *bool,
) error {
	return PatchSystemDeploymentFeatureFlags(
		ctx,
		testCtx,
		constant.SystemPodsNamespace,
		constant.SchedulerDeploymentName,
		constant.SchedulerContainerName,
		func(args []string) []string {
			if value == nil {
				return args
			}
			return append(args, fmt.Sprintf("--full-hierarchy-fairness=%t", *value))
		},
	)

}
