/*
Copyright 2025 NVIDIA CORPORATION
SPDX-License-Identifier: Apache-2.0
*/
package reclaim

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"sigs.k8s.io/controller-runtime/pkg/client"

	kaiv1 "github.com/NVIDIA/KAI-scheduler/pkg/apis/kai/v1"
	"github.com/NVIDIA/KAI-scheduler/test/e2e/modules/configurations/feature_flags"
	testcontext "github.com/NVIDIA/KAI-scheduler/test/e2e/modules/context"
	"github.com/NVIDIA/KAI-scheduler/test/e2e/modules/resources/rd/scheduling_shard"
	"github.com/NVIDIA/KAI-scheduler/test/e2e/modules/testconfig"
)

const (
	draShardName           = "dra-shard"
	draPartitionLabelValue = "dra"
	draNodeLabel           = "nvidia.com/gpu.deploy.dra-plugin-gpu"
)

func init() {
	cfg := testconfig.GetConfig()
	cfg.SetFullHierarchyFairness = func(ctx context.Context, testCtx any, value *bool) error {
		return feature_flags.SetFullHierarchyFairness(ctx, testCtx.(*testcontext.TestContext), value)
	}
	testconfig.SetConfig(cfg)
}

var _ = Describe("Hierarchy level fairness - setup", Ordered, func() {
	var testCtx *testcontext.TestContext

	BeforeAll(func(ctx context.Context) {
		testCtx = testcontext.GetConnectivity(ctx, Default)

		By("Creating SchedulingShard for DRA nodes")
		err := scheduling_shard.CreateShardForLabeledNodes(
			ctx,
			testCtx.ControllerClient,
			draShardName,
			client.MatchingLabels{draNodeLabel: "true"},
			kaiv1.SchedulingShardSpec{
				PartitionLabelValue: draPartitionLabelValue,
			},
		)
		Expect(err).NotTo(HaveOccurred())
	})

	AfterAll(func(ctx context.Context) {
		By("Deleting DRA SchedulingShard and removing labels")
		err := scheduling_shard.DeleteShardAndRemoveLabels(ctx, testCtx.ControllerClient, draShardName)
		Expect(err).NotTo(HaveOccurred())
	})

	DescribeHierarchyLevelFairnessSpecs()
})
