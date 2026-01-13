/*
Copyright 2025 NVIDIA CORPORATION
SPDX-License-Identifier: Apache-2.0
*/
package timeaware

import (
	"context"
	"os"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"sigs.k8s.io/controller-runtime/pkg/client"

	kaiv1 "github.com/NVIDIA/KAI-scheduler/pkg/apis/kai/v1"
	"github.com/NVIDIA/KAI-scheduler/pkg/common/constants"
	"github.com/NVIDIA/KAI-scheduler/test/e2e/modules/configurations"
	testcontext "github.com/NVIDIA/KAI-scheduler/test/e2e/modules/context"
	"github.com/NVIDIA/KAI-scheduler/test/e2e/modules/utils"
	"github.com/NVIDIA/KAI-scheduler/test/e2e/modules/wait"
)

const (
	defaultShardName        = "default"
	prometheusReadyTimeout  = 2 * time.Minute
	schedulerRestartTimeout = 30 * time.Second
)

var (
	testCtx                 *testcontext.TestContext
	originalKAIConfig       *kaiv1.Config
	originalSchedulingShard *kaiv1.SchedulingShard
)

func TestTimeAware(t *testing.T) {
	utils.SetLogger()
	RegisterFailHandler(Fail)
	RunSpecs(t, "Time Aware Fairness Suite")
}

var _ = BeforeSuite(func(ctx context.Context) {
	By("Setting up test context")
	testCtx = testcontext.GetConnectivity(ctx, Default)

	By("Saving original KAI config for restoration")
	originalKAIConfig = &kaiv1.Config{}
	err := testCtx.ControllerClient.Get(ctx, client.ObjectKey{Name: constants.DefaultKAIConfigSingeltonInstanceName}, originalKAIConfig)
	Expect(err).NotTo(HaveOccurred(), "Failed to get original KAI config")

	By("Saving original SchedulingShard for restoration")
	originalSchedulingShard = &kaiv1.SchedulingShard{}
	err = testCtx.ControllerClient.Get(ctx, client.ObjectKey{Name: defaultShardName}, originalSchedulingShard)
	Expect(err).NotTo(HaveOccurred(), "Failed to get original SchedulingShard")

	// SKIP_TIME_AWARE=true can be used to verify this test is not a false positive
	// When set, we EXPLICITLY DISABLE time-aware fairness (set kValue=0)
	// and the test should FAIL because reclaim won't happen
	if os.Getenv("SKIP_TIME_AWARE") == "true" {
		By("SKIP_TIME_AWARE=true: Explicitly disabling time-aware fairness (kValue=0) - test should FAIL")
		// We still enable Prometheus but set kValue=0 to disable time-aware effect
		config := configurations.DefaultTimeAwareConfig()
		config.KValue = 0 // Disable time-aware fairness effect
		err = configurations.EnableTimeAwareFairness(ctx, testCtx, defaultShardName, config)
		Expect(err).NotTo(HaveOccurred(), "Failed to configure scheduler")

		By("Waiting for Prometheus pod to be ready")
		wait.ForPrometheusReady(ctx, testCtx.ControllerClient, prometheusReadyTimeout)

		By("Waiting for scheduler to restart with kValue=0")
		err = wait.ForRolloutRestartDeployment(ctx, testCtx.ControllerClient, "kai-scheduler", "kai-scheduler-default")
		Expect(err).NotTo(HaveOccurred(), "Failed waiting for scheduler rollout restart")
		return
	}

	By("Enabling time-aware fairness: setting prometheus.enabled=true in KAI Config and usageDBConfig in shard")
	// This does two things:
	// 1. Sets prometheus.enabled=true in KAI Config -> operator creates Prometheus instance
	// 2. Sets usageDBConfig.clientType=prometheus (no URL) -> operator auto-resolves URL
	config := configurations.DefaultTimeAwareConfig()
	err = configurations.EnableTimeAwareFairness(ctx, testCtx, defaultShardName, config)
	Expect(err).NotTo(HaveOccurred(), "Failed to enable time-aware fairness")

	By("Waiting for Prometheus pod to be ready (operator should have created it)")
	wait.ForPrometheusReady(ctx, testCtx.ControllerClient, prometheusReadyTimeout)

	By("Waiting for scheduler to restart with new configuration (including auto-resolved prometheus URL)")
	err = wait.ForRolloutRestartDeployment(ctx, testCtx.ControllerClient, "kai-scheduler", "kai-scheduler-default")
	Expect(err).NotTo(HaveOccurred(), "Failed waiting for scheduler rollout restart")
})

var _ = AfterSuite(func(ctx context.Context) {
	if testCtx == nil {
		return
	}

	By("Restoring original SchedulingShard configuration")
	if originalSchedulingShard != nil {
		err := configurations.PatchSchedulingShard(ctx, testCtx, defaultShardName, func(shard *kaiv1.SchedulingShard) {
			shard.Spec.UsageDBConfig = originalSchedulingShard.Spec.UsageDBConfig
			shard.Spec.KValue = originalSchedulingShard.Spec.KValue
		})
		if err != nil {
			GinkgoWriter.Printf("Warning: Failed to restore original SchedulingShard: %v\n", err)
		}
	}

	By("Restoring original KAI config")
	if originalKAIConfig != nil {
		err := configurations.PatchKAIConfig(ctx, testCtx, func(kaiConfig *kaiv1.Config) {
			kaiConfig.Spec.Prometheus = originalKAIConfig.Spec.Prometheus
		})
		if err != nil {
			GinkgoWriter.Printf("Warning: Failed to restore original KAI config: %v\n", err)
		}
	}
})
