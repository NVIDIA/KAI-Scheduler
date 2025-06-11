/*
Copyright 2025 NVIDIA CORPORATION
SPDX-License-Identifier: Apache-2.0
*/
package feature_flags

import (
	"context"
	"fmt"
	"strings"

	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/conf"
	testContext "github.com/NVIDIA/KAI-scheduler/test/e2e/modules/context"
)

const (
	binpackStrategy = "binpack"
	SpreadStrategy  = "spread"
	DefaultStrategy = binpackStrategy
	gpuResource     = "gpu"
	cpuResource     = "cpu"
)

func SetPlacementStrategy(
	ctx context.Context, testCtx *testContext.TestContext, strategy string,
) error {
	placementArguments := map[string]string{
		gpuResource: strategy, cpuResource: strategy,
	}

	config := conf.SchedulerConfiguration{}

	actions := []string{"allocate"}
	if placementArguments[gpuResource] != SpreadStrategy && placementArguments[cpuResource] != SpreadStrategy {
		actions = append(actions, "consolidation")
	}
	actions = append(actions, []string{"reclaim", "preempt", "stalegangeviction"}...)

	config.Actions = strings.Join(actions, ", ")

	config.Tiers = conf.GetDefaultSchedulerConfiguration().Tiers
	config.Tiers[0].Plugins = append(
		config.Tiers[0].Plugins,
		conf.PluginOption{Name: fmt.Sprintf("gpu%s", strings.Replace(placementArguments[gpuResource], "bin", "", 1))},
		conf.PluginOption{
			Name:      "nodeplacement",
			Arguments: placementArguments,
		},
	)

	if placementArguments[gpuResource] == binpackStrategy {
		config.Tiers[0].Plugins = append(
			config.Tiers[0].Plugins,
			conf.PluginOption{Name: "gpusharingorder"},
		)
	}

	return updateKaiSchedulerConfigMap(ctx, testCtx, &config)
}
