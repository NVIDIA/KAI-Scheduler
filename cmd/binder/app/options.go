// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package app

import (
	"github.com/spf13/pflag"

	utilfeature "k8s.io/apiserver/pkg/util/feature"
)

type Options struct {
	SchedulerName                        string
	ResourceReservationNamespace         string
	ResourceReservationServiceAccount    string
	ResourceReservationPodImage          string
	ResourceReservationAppLabel          string
	ResourceReservationAllocationTimeout int
	ScalingPodNamespace                  string
	QPS                                  float64
	Burst                                int
	MaxConcurrentReconciles              int
	RateLimiterBaseDelaySeconds          int
	RateLimiterMaxDelaySeconds           int
	EnableLeaderElection                 bool
	MetricsAddr                          string
	ProbeAddr                            string
	WebhookPort                          int
	FakeGPUNodes                         bool
	GpuCdiEnabled                        bool
	VolumeBindingTimeoutSeconds          int
	GPUSharingEnabled                    bool
}

func InitOptions() *Options {
	options := &Options{}

	fs := pflag.CommandLine

	fs.StringVar(&options.SchedulerName,
		"scheduler-name", "kai-scheduler",
		"The scheduler name the workloads are scheduled with")
	fs.StringVar(&options.ResourceReservationNamespace,
		"resource-reservation-namespace", "kai-resource-reservation",
		"Namespace for resource reservation pods")
	fs.StringVar(&options.ResourceReservationServiceAccount,
		"resource-reservation-service-account", "kai-resource-reservation",
		"Service account name for resource reservation pods")
	fs.StringVar(&options.ResourceReservationPodImage,
		"resource-reservation-pod-image", "registry/local/kai-scheduler/resource-reservation",
		"Container image for the resource reservation pod")
	fs.StringVar(&options.ResourceReservationAppLabel,
		"resource-reservation-app-label", "kai-resource-reservation",
		"App label value of resource reservation pods")
	fs.IntVar(&options.ResourceReservationAllocationTimeout,
		"resource-reservation-allocation-timeout", 40,
		"Resource reservation allocation timeout in seconds")
	fs.StringVar(&options.ScalingPodNamespace,
		"scale-adjust-namespace", "kai-scale-adjust",
		"Scaling pods namespace")
	fs.Float64Var(&options.QPS,
		"qps", 50,
		"Queries per second to the K8s API server")
	fs.IntVar(&options.Burst,
		"burst", 300,
		"Burst to the K8s API server")
	fs.IntVar(&options.MaxConcurrentReconciles,
		"max-concurrent-reconciles", 10,
		"Max concurrent reconciles")
	fs.IntVar(&options.RateLimiterBaseDelaySeconds,
		"rate-limiter-base-delay", 1,
		"Base delay in seconds for the ExponentialFailureRateLimiter")
	fs.IntVar(&options.RateLimiterMaxDelaySeconds,
		"rate-limiter-max-delay", 60,
		"Max delay in seconds for the ExponentialFailureRateLimiter")
	fs.BoolVar(&options.EnableLeaderElection,
		"leader-elect", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	fs.StringVar(&options.MetricsAddr,
		"metrics-bind-address", ":8080",
		"The address the metric endpoint binds to.")
	fs.StringVar(&options.ProbeAddr,
		"health-probe-bind-address", ":8081",
		"The address the probe endpoint binds to.")
	fs.IntVar(&options.WebhookPort,
		"webhook-addr", 9443,
		"The port the webhook binds to.")
	fs.BoolVar(&options.FakeGPUNodes,
		"fake-gpu-nodes", false,
		"Enables running fractions on fake gpu nodes for testing")
	fs.BoolVar(&options.GpuCdiEnabled,
		"cdi-enabled", false,
		"Specifies if the gpu device plugin uses the cdi devices api to set gpu devices to the pods")
	fs.IntVar(&options.VolumeBindingTimeoutSeconds,
		"volume-binding-timeout-seconds", 120,
		"Volume binding timeout in seconds")
	fs.BoolVar(&options.GPUSharingEnabled,
		"gpu-sharing-enabled", false,
		"Specifies if the GPU sharing is enabled")

	utilfeature.DefaultMutableFeatureGate.AddFlag(fs)

	return options
}
