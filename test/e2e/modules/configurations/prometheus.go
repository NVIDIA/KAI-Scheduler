// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package configurations

import (
	"context"
	"time"

	"k8s.io/utils/ptr"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	kaiv1 "github.com/NVIDIA/KAI-scheduler/pkg/apis/kai/v1"
	kaiprometheus "github.com/NVIDIA/KAI-scheduler/pkg/apis/kai/v1/prometheus"
	usagedbapi "github.com/NVIDIA/KAI-scheduler/pkg/scheduler/cache/usagedb/api"
	testcontext "github.com/NVIDIA/KAI-scheduler/test/e2e/modules/context"
)

// TimeAwareConfig holds configuration for time-aware fairness tests
type TimeAwareConfig struct {
	// PrometheusEnabled enables the internal Prometheus instance
	PrometheusEnabled bool
	// ServiceMonitorInterval is the scrape interval for ServiceMonitors (e.g., "5s")
	ServiceMonitorInterval string
	// WindowSize is the time window for fairness calculation
	WindowSize time.Duration
	// HalfLifePeriod is the decay period for historical usage
	HalfLifePeriod time.Duration
	// FetchInterval is how often to fetch usage data from Prometheus
	FetchInterval time.Duration
	// KValue controls fairness aggressiveness (default 1.0)
	KValue float64
}

// DefaultTimeAwareConfig returns a TimeAwareConfig suitable for fast e2e testing
func DefaultTimeAwareConfig() TimeAwareConfig {
	return TimeAwareConfig{
		PrometheusEnabled:      true,
		ServiceMonitorInterval: "5s",
		WindowSize:             30 * time.Second,
		HalfLifePeriod:         15 * time.Second,
		FetchInterval:          5 * time.Second,
		KValue:                 1.0,
	}
}

// EnableTimeAwareFairness configures KAI for time-aware fairness testing
// It enables prometheus and configures the scheduling shard with usageDBConfig
func EnableTimeAwareFairness(ctx context.Context, testCtx *testcontext.TestContext, shardName string, config TimeAwareConfig) error {
	// Enable prometheus in KAI config
	err := PatchKAIConfig(ctx, testCtx, func(kaiConfig *kaiv1.Config) {
		if kaiConfig.Spec.Prometheus == nil {
			kaiConfig.Spec.Prometheus = &kaiprometheus.Prometheus{}
		}
		kaiConfig.Spec.Prometheus.Enabled = ptr.To(config.PrometheusEnabled)
		if kaiConfig.Spec.Prometheus.ServiceMonitor == nil {
			kaiConfig.Spec.Prometheus.ServiceMonitor = &kaiprometheus.ServiceMonitor{}
		}
		kaiConfig.Spec.Prometheus.ServiceMonitor.Enabled = ptr.To(true)
		kaiConfig.Spec.Prometheus.ServiceMonitor.Interval = ptr.To(config.ServiceMonitorInterval)
	})
	if err != nil {
		return err
	}

	// Configure shard with usageDBConfig (no connectionString - should auto-resolve)
	return PatchSchedulingShard(ctx, testCtx, shardName, func(shard *kaiv1.SchedulingShard) {
		windowType := usagedbapi.SlidingWindow
		shard.Spec.UsageDBConfig = &usagedbapi.UsageDBConfig{
			ClientType: "prometheus",
			// No ConnectionString - should be auto-resolved to prometheus-operated
			UsageParams: &usagedbapi.UsageParams{
				WindowSize:     &metav1.Duration{Duration: config.WindowSize},
				HalfLifePeriod: &metav1.Duration{Duration: config.HalfLifePeriod},
				FetchInterval:  &metav1.Duration{Duration: config.FetchInterval},
				WindowType:     &windowType,
			},
		}
		shard.Spec.KValue = ptr.To(config.KValue)
	})
}

// DisableTimeAwareFairness removes time-aware fairness configuration
func DisableTimeAwareFairness(ctx context.Context, testCtx *testcontext.TestContext, shardName string) error {
	// Remove usageDBConfig from shard
	err := PatchSchedulingShard(ctx, testCtx, shardName, func(shard *kaiv1.SchedulingShard) {
		shard.Spec.UsageDBConfig = nil
		shard.Spec.KValue = nil
	})
	if err != nil {
		return err
	}

	// Disable prometheus in KAI config
	return PatchKAIConfig(ctx, testCtx, func(kaiConfig *kaiv1.Config) {
		if kaiConfig.Spec.Prometheus != nil {
			kaiConfig.Spec.Prometheus.Enabled = ptr.To(false)
		}
	})
}
