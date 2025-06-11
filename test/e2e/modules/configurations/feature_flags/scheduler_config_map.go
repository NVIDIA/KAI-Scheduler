/*
Copyright 2025 NVIDIA CORPORATION
SPDX-License-Identifier: Apache-2.0
*/
package feature_flags

import (
	"context"

	"gopkg.in/yaml.v3"

	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/conf"
	"github.com/NVIDIA/KAI-scheduler/test/e2e/modules/constant"
	testContext "github.com/NVIDIA/KAI-scheduler/test/e2e/modules/context"
	"github.com/NVIDIA/KAI-scheduler/test/e2e/modules/wait"
)

const (
	schedulerConfigMapName = "scheduler-config"
	schedulerConfigDataKey = "config.yaml"
)

func updateKaiSchedulerConfigMap(ctx context.Context, testCtx *testContext.TestContext, config *conf.SchedulerConfiguration) error {
	schedulerConfig, err := testCtx.KubeClientset.CoreV1().ConfigMaps(constant.SystemPodsNamespace).
		Get(ctx, schedulerConfigMapName, metav1.GetOptions{})
	if err != nil {
		return err
	}
	data, marshalErr := yaml.Marshal(config)
	if marshalErr != nil {
		return marshalErr
	}
	schedulerConfig.Data = map[string]string{
		schedulerConfigDataKey: string(data),
	}

	_, err = testCtx.KubeClientset.CoreV1().ConfigMaps(constant.SystemPodsNamespace).
		Update(ctx, schedulerConfig, metav1.UpdateOptions{})
	if err != nil {
		return err
	}

	return wait.ForRolloutRestartDeployment(ctx, testCtx.ControllerClient, constant.SystemPodsNamespace,
		constant.SchedulerDeploymentName)
}
