/*
Copyright 2025 NVIDIA CORPORATION
SPDX-License-Identifier: Apache-2.0
*/
package testconfig

import (
	"context"

	"k8s.io/client-go/kubernetes"
)

type TestConfig struct {
	SchedulerName           string
	SystemPodsNamespace     string
	ReservationNamespace    string
	SchedulerDeploymentName string
	QueueLabelKey           string
	QueueNamespacePrefix    string
	ContainerImage          string

	OnNamespaceCreated func(ctx context.Context, kubeClientset kubernetes.Interface, namespaceName, queueName string) error

	// Configuration callbacks — allow repos to plug in their own CRD-typed operations.
	// testCtx is typed as `any` to avoid a circular import (context -> testconfig -> context).
	// Callers pass *testcontext.TestContext; adapters type-assert it.
	SetFullHierarchyFairness func(ctx context.Context, testCtx any, value *bool) error
}

var activeConfig = TestConfig{
	SchedulerName:           "kai-scheduler",
	SystemPodsNamespace:     "kai-scheduler",
	ReservationNamespace:    "kai-resource-reservation",
	SchedulerDeploymentName: "kai-scheduler-default",
	QueueLabelKey:           "kai.scheduler/queue",
	QueueNamespacePrefix:    "kai-",
	ContainerImage:          "ubuntu",
}

func SetConfig(cfg TestConfig) { activeConfig = cfg }
func GetConfig() TestConfig    { return activeConfig }
