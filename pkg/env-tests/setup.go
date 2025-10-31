// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package env_tests

import (
	"fmt"
	"path/filepath"

	resourceapi "k8s.io/api/resource/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/util/flowcontrol"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"

	kaiv1alpha2 "github.com/NVIDIA/KAI-scheduler/pkg/apis/scheduling/v1alpha2"
	kaiv2 "github.com/NVIDIA/KAI-scheduler/pkg/apis/scheduling/v2"
	kaiv2v2alpha2 "github.com/NVIDIA/KAI-scheduler/pkg/apis/scheduling/v2alpha2"
	kueuev1alpha1 "sigs.k8s.io/kueue/apis/kueue/v1alpha1"
)

func SetupEnvTest(crdDirectoryPaths []string) (*rest.Config, client.Client, *envtest.Environment, error) {
	var (
		cfg        *rest.Config
		ctrlClient client.Client
		testEnv    *envtest.Environment
	)

	if crdDirectoryPaths == nil {
		crdDirectoryPaths = []string{
			filepath.Join("..", "..", "deployments", "kai-scheduler", "crds"),
			filepath.Join("..", "..", "deployments", "external-crds"),
		}
	}

	testEnv = &envtest.Environment{
		CRDDirectoryPaths:     crdDirectoryPaths,
		ErrorIfCRDPathMissing: true,
	}

	testEnv.ControlPlane.GetAPIServer().Configure().Append("feature-gates", "DynamicResourceAllocation=true")
	testEnv.ControlPlane.GetAPIServer().Configure().Append("runtime-config", "api/all=true")

	var err error
	// cfg is defined in this file globally
	cfg, err = testEnv.Start()
	if err != nil {
		return nil, nil, testEnv, fmt.Errorf("failed to start test env: %w", err)
	}
	cfg.ContentType = "application/json"

	// Effectively disable rate limiting
	cfg.RateLimiter = flowcontrol.NewFakeAlwaysRateLimiter()

	// Add any scheme registration here if needed for your custom CRDs
	err = kaiv2.AddToScheme(scheme.Scheme)
	if err != nil {
		return nil, nil, testEnv, fmt.Errorf("failed to add kaiv2 scheme: %w", err)
	}
	err = kaiv1alpha2.AddToScheme(scheme.Scheme)
	if err != nil {
		return nil, nil, testEnv, fmt.Errorf("failed to add kaiv1alpha2 scheme: %w", err)
	}
	err = kaiv2v2alpha2.AddToScheme(scheme.Scheme)
	if err != nil {
		return nil, nil, testEnv, fmt.Errorf("failed to add kaiv2v2alpha2 scheme: %w", err)
	}
	err = resourceapi.AddToScheme(scheme.Scheme)
	if err != nil {
		return nil, nil, testEnv, fmt.Errorf("failed to add resourceapi scheme: %w", err)
	}
	err = kueuev1alpha1.AddToScheme(scheme.Scheme)
	if err != nil {
		return nil, nil, testEnv, fmt.Errorf("failed to add kueuev1alpha1 scheme: %w", err)
	}
	// +kubebuilder:scaffold:scheme

	ctrlClient, err = client.New(cfg, client.Options{Scheme: scheme.Scheme})
	if err != nil {
		return nil, nil, testEnv, fmt.Errorf("Failed to get controller client: %v", err)
	}

	return cfg, ctrlClient, testEnv, nil
}
