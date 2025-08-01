// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package scheduler

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/spf13/pflag"
	"k8s.io/client-go/rest"

	"github.com/NVIDIA/KAI-scheduler/cmd/scheduler/app"
	"github.com/NVIDIA/KAI-scheduler/cmd/scheduler/app/options"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/conf"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/log"
)

var loggerInitiated = false

func RunScheduler(cfg *rest.Config, stopCh chan struct{}, schedulerConfig *conf.SchedulerConfiguration) error {
	if !loggerInitiated {
		err := log.InitLoggers(0)
		if err != nil {
			return err
		}
		loggerInitiated = true
	}

	opt := options.NewServerOption()

	args := []string{
		"--schedule-period=10ms",
		"--feature-gates=DynamicResourceAllocation=true",
	}
	fs := pflag.NewFlagSet("flags", pflag.ExitOnError)
	opt.AddFlags(fs)
	err := fs.Parse(args)
	if err != nil {
		return err
	}

	configFilePath := ""
	if schedulerConfig != nil {
		tempDir, err := os.MkdirTemp("", "scheduler-config")
		if err != nil {
			return err
		}
		defer os.RemoveAll(tempDir)

		configFile := filepath.Join(tempDir, "scheduler-config.json")
		configBytes, err := json.Marshal(schedulerConfig)
		if err != nil {
			return err
		}

		err = os.WriteFile(configFile, configBytes, 0644)
		if err != nil {
			return err
		}
		configFilePath = configFile
	}

	params := app.BuildSchedulerParams(opt)
	s, err := scheduler.NewScheduler(cfg, configFilePath, params, nil)
	if err != nil {
		return err
	}

	s.Run(stopCh)
	return nil
}
