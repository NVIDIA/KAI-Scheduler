// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"os"

	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/NVIDIA/KAI-scheduler/cmd/binder/app"

	admissionplugins "github.com/NVIDIA/KAI-scheduler/pkg/admission/plugins"
	admissiongpusharing "github.com/NVIDIA/KAI-scheduler/pkg/admission/webhook/v1alpha2/gpusharing"
	bindingplugins "github.com/NVIDIA/KAI-scheduler/pkg/binder/plugins"
	bindinggpusharing "github.com/NVIDIA/KAI-scheduler/pkg/binder/plugins/gpusharing"
	k8s_plugins "github.com/NVIDIA/KAI-scheduler/pkg/binder/plugins/k8s-plugins"
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
	binderPlugins := bindingplugins.New()
	admissionPlugins := admissionplugins.New()
	k8sPlugins, err := k8s_plugins.New(app.K8sInterface, app.InformerFactory,
		int64(app.Options.VolumeBindingTimeoutSeconds))
	if err != nil {
		return err
	}
	binderPlugins.RegisterPlugin(k8sPlugins)

	bindingGpuSharingPlugin := bindinggpusharing.New(app.Client,
		app.Options.GpuCdiEnabled, app.Options.GPUSharingEnabled)
	admissionGpuSharingPlugin := admissiongpusharing.New(app.Client,
		app.Options.GpuCdiEnabled, app.Options.GPUSharingEnabled)

	binderPlugins.RegisterPlugin(bindingGpuSharingPlugin)
	admissionPlugins.RegisterPlugin(admissionGpuSharingPlugin)
	app.RegisterPlugins(admissionPlugins, binderPlugins)
	return nil
}
