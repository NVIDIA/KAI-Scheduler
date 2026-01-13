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
	e2econstant "github.com/NVIDIA/KAI-scheduler/test/e2e/modules/constant"
	testcontext "github.com/NVIDIA/KAI-scheduler/test/e2e/modules/context"
	"github.com/NVIDIA/KAI-scheduler/test/e2e/modules/utils"
	"github.com/NVIDIA/KAI-scheduler/test/e2e/modules/wait"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	defaultShardName        = "default"
	prometheusPodName       = "prometheus-prometheus-0"
	prometheusReadyTimeout  = 2 * time.Minute
	schedulerRestartTimeout = 30 * time.Second
)

var testCtx *testcontext.TestContext

func TestTimeAware(t *testing.T) {
	utils.SetLogger()
	RegisterFailHandler(Fail)
	RunSpecs(t, "Time Aware Fairness Suite")
}

var _ = BeforeSuite(func(ctx context.Context) {
	By("Setting up test context")
	testCtx = testcontext.GetConnectivity(ctx, Default)

	By("Saving original KAI config for restoration")
	originalKAIConfig := &kaiv1.Config{}
	err := testCtx.ControllerClient.Get(ctx, client.ObjectKey{Name: constants.DefaultKAIConfigSingeltonInstanceName}, originalKAIConfig)
	Expect(err).NotTo(HaveOccurred(), "Failed to get original KAI config")

	By("Saving original SchedulingShard for restoration")
	originalSchedulingShard := &kaiv1.SchedulingShard{}
	err = testCtx.ControllerClient.Get(ctx, client.ObjectKey{Name: defaultShardName}, originalSchedulingShard)
	Expect(err).NotTo(HaveOccurred(), "Failed to get original SchedulingShard")

	By("Enabling time-aware fairness")
	config := defaultTimeAwareConfig()
	err = configureTimeAwareFairness(ctx, testCtx, defaultShardName, config)
	Expect(err).NotTo(HaveOccurred(), "Failed to enable time-aware fairness")
	DeferCleanup(func(ctx context.Context) {
		By("Restoring original SchedulingShard configuration")
		if err := configurations.PatchSchedulingShard(ctx, testCtx, defaultShardName, func(shard *kaiv1.SchedulingShard) {
			shard.Spec = originalSchedulingShard.Spec
		}); err != nil {
			GinkgoWriter.Printf("Warning: Failed to restore original SchedulingShard: %v\n", err)
		}

		By("Restoring original KAI config")
		if err := configurations.PatchKAIConfig(ctx, testCtx, func(config *kaiv1.Config) {
			config.Spec = originalKAIConfig.Spec
		}); err != nil {
			GinkgoWriter.Printf("Warning: Failed to restore original KAI config: %v\n", err)
		}
	})

	By("Waiting for Prometheus pod to be ready (operator should have created it)")
	wait.ForPodReady(ctx, testCtx.ControllerClient, &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: e2econstant.SystemPodsNamespace,
			Name:      prometheusPodName,
		},
	})

	By("Waiting for scheduler to restart with new configuration (including auto-resolved prometheus URL)")
	err = wait.ForRolloutRestartDeployment(ctx, testCtx.ControllerClient, e2econstant.SystemPodsNamespace, e2econstant.SchedulerDeploymentName)
	Expect(err).NotTo(HaveOccurred(), "Failed waiting for scheduler rollout restart")
})
