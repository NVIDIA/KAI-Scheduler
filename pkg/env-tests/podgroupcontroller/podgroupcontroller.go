// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package podgroupcontroller

import (
	"context"
	"fmt"

	"k8s.io/client-go/rest"

	"github.com/NVIDIA/KAI-scheduler/cmd/podgroupcontroller/app"
)

func RunPodGroupController(cfg *rest.Config, ctx context.Context) error {
	options := app.InitOptions()
	options.MetricsAddr = "0"
	options.ProbeAddr = "0"
	options.EnablePodGroupWebhook = false
	options.EnableLeaderElection = false

	go func() {
		if err := app.Run(options, cfg, ctx); err != nil {
			panic(fmt.Errorf("failed to run podgroupcontroller app: %w", err))
		}
	}()

	return nil
}
