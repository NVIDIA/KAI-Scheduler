// Copyright 2026 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package feature_flags

import (
	"context"

	kaiv1 "github.com/NVIDIA/KAI-scheduler/pkg/apis/kai/v1"
	. "github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/NVIDIA/KAI-scheduler/test/e2e/modules/constant"

	testContext "github.com/NVIDIA/KAI-scheduler/test/e2e/modules/context"
	"github.com/NVIDIA/KAI-scheduler/test/e2e/modules/wait"
)

type patchCallback func(shard *kaiv1.SchedulingShard)

func patchShard(
	ctx context.Context, testCtx *testContext.TestContext, shardName string, patcher patchCallback,
) error {
	shard := &kaiv1.SchedulingShard{}
	err := testCtx.ControllerClient.Get(ctx, types.NamespacedName{Name: shardName}, shard)
	if errors.IsNotFound(err) {
		err = nil
		shard = &kaiv1.SchedulingShard{
			ObjectMeta: metav1.ObjectMeta{
				Name: shardName,
			},
		}
	}
	if err != nil {
		return err
	}

	originalDefaultShard := shard.DeepCopy()
	patch := client.MergeFrom(originalDefaultShard)

	patcher(shard)

	Expect(testCtx.ControllerClient.Patch(ctx, shard, patch)).To(Succeed())

	wait.ForSchedulingShardStatusOK(ctx, testCtx.ControllerClient, "default")

	// These lines are here to workaround shard status issue - RUN-13930:
	engineConfig := &kaiv1.Config{}
	Expect(testCtx.ControllerClient.Get(ctx, types.NamespacedName{Name: "engine-config"}, engineConfig)).To(Succeed())

	schedulerAppName := "kai-scheduler-" + shardName
	err = testCtx.ControllerClient.DeleteAllOf(
		ctx, &v1.Pod{},
		client.InNamespace(engineConfig.Spec.Namespace),
		client.MatchingLabels{constant.AppLabelName: schedulerAppName},
		client.GracePeriodSeconds(0),
	)
	err = client.IgnoreNotFound(err)
	Expect(err).To(Succeed())

	wait.ForRunningSystemComponentEvent(ctx, testCtx.ControllerClient, schedulerAppName)

	return nil
}
