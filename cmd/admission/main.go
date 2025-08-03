package admission

// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

import (
	"os"

	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/NVIDIA/KAI-scheduler/cmd/admission/app"

	admissionplugins "github.com/NVIDIA/KAI-scheduler/pkg/admission/plugins"
	admissiongpusharing "github.com/NVIDIA/KAI-scheduler/pkg/admission/webhook/v1alpha2/gpusharing"
)

var (
	setupLog = ctrl.Log.WithName("admission-setup")
)

func main() {
	app, err := app.New()
	if err != nil {
		setupLog.Error(err, "failed to create app")
		os.Exit(1)
	}

	err = registerPlugins(app)
	if err != nil {
		setupLog.Error(err, "failed to register plugins")
		os.Exit(1)
	}

	err = app.Run()
	if err != nil {
		setupLog.Error(err, "failed to run app")
		os.Exit(1)
	}
}

func registerPlugins(app *app.App) error {
	admissionPlugins := admissionplugins.New()

	admissionGpuSharingPlugin := admissiongpusharing.New(app.Client,
		app.Options.GpuCdiEnabled, app.Options.GPUSharingEnabled)

	admissionPlugins.RegisterPlugin(admissionGpuSharingPlugin)
	app.RegisterPlugins(admissionPlugins)
	return nil
}
