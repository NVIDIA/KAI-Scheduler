// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"os"

	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/NVIDIA/KAI-scheduler/cmd/operator/app"
	"github.com/NVIDIA/KAI-scheduler/pkg/operator/controller"
)

var (
	setupLog = ctrl.Log.WithName("setup")
)

func main() {
	app, err := app.New()
	if err != nil {
		setupLog.Error(err, "failed to create app")
		os.Exit(1)
	}

	app.InitOperands(controller.ConfigReconcilerOperands)

	err = app.Run()
	if err != nil {
		setupLog.Error(err, "failed to run app")
		os.Exit(1)
	}
}
