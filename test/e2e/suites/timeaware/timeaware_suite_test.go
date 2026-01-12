/*
Copyright 2025 NVIDIA CORPORATION
SPDX-License-Identifier: Apache-2.0
*/
package timeaware

import (
	"context"
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
	defaultShardName     = "default"
	prometheusReadyTimeout = 2 * time.Minute
	schedulerRestartTimeout = 30 * time.Second
)

var (
	testCtx                  *testcontext.TestContext
	originalKAIConfig        *kaiv1.Config
	originalSchedulingShard  *kaiv1.SchedulingShard
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

	By("Enabling time-aware fairness with managed Prometheus")
	config := configurations.DefaultTimeAwareConfig()
	err = configurations.EnableTimeAwareFairness(ctx, testCtx, defaultShardName, config)
	Expect(err).NotTo(HaveOccurred(), "Failed to enable time-aware fairness")

	By("Waiting for Prometheus to be ready")
	wait.ForPrometheusReady(ctx, testCtx.ControllerClient, prometheusReadyTimeout)

	By("Waiting for scheduler to restart with new configuration")
	wait.ForRolloutRestartDeployment(ctx, testCtx.ControllerClient, "kai-scheduler", "kai-scheduler-default")
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
