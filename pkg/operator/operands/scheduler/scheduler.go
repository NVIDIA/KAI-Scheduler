// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package scheduler

import (
	"context"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	kaiv1 "github.com/NVIDIA/KAI-scheduler/pkg/apis/kai/v1"
	"github.com/NVIDIA/KAI-scheduler/pkg/operator/operands/common"
)

const (
	configMountPath = "/etc/config/config.yaml"
	binpackStrategy = "binpack"
	spreadStrategy  = "spread"
	gpuResource     = "gpu"
	cpuResource     = "cpu"
)

type config struct {
	Actions             string         `yaml:"actions"`
	Tiers               []tier         `yaml:"tiers,omitempty"`
	QueueDepthPerAction map[string]int `yaml:"queueDepthPerAction,omitempty"`
}

type tier struct {
	Plugins []plugin `yaml:"plugins"`
}

type plugin struct {
	Name      string            `yaml:"name"`
	Arguments map[string]string `yaml:"arguments,omitempty"`
}

type SchedulerForShard struct {
	schedulingShard *kaiv1.SchedulingShard

	lastDesiredState []client.Object
}

type SchedulerForConfig struct {
	lastDesiredState []client.Object
}

func NewSchedulerForShard(schedulingShard *kaiv1.SchedulingShard) *SchedulerForShard {
	return &SchedulerForShard{schedulingShard: schedulingShard}
}

type resourceForShard func(
	ctx context.Context, runtimeClient client.Reader, kaiConfig *kaiv1.Config, shardObj *kaiv1.SchedulingShard,
) (client.Object, error)

func (s *SchedulerForShard) DesiredState(
	ctx context.Context, readerClient client.Reader, kaiConfig *kaiv1.Config,
) ([]client.Object, error) {
	logger := log.FromContext(ctx)

	if !*kaiConfig.Spec.Scheduler.Service.Enabled {
		logger.Info("Scheduler operand is disabled")
		s.lastDesiredState = []client.Object{}

		return nil, nil
	}

	objects := []client.Object{}
	for _, resourceFunc := range []resourceForShard{
		deploymentForShard,
		configMapForShard,
		serviceForShard,
	} {
		object, err := resourceFunc(ctx, readerClient, kaiConfig, s.schedulingShard)
		if err != nil {
			return nil, err
		}
		objects = append(objects, object)
	}

	s.lastDesiredState = objects

	return s.lastDesiredState, nil
}

func (s *SchedulerForShard) IsAvailable(ctx context.Context, readerClient client.Reader) (bool, error) {
	return common.AllControllersAvailable(ctx, readerClient, s.lastDesiredState)
}

func (s *SchedulerForShard) IsDeployed(ctx context.Context, readerClient client.Reader) (bool, error) {
	return common.AllObjectsExists(ctx, readerClient, s.lastDesiredState)
}

func (s *SchedulerForShard) Name() string {
	return "SchedulerForShard"
}

func (s *SchedulerForConfig) DesiredState(
	ctx context.Context, readerClient client.Reader, kaiConfig *kaiv1.Config,
) ([]client.Object, error) {
	logger := log.FromContext(ctx)

	if !*kaiConfig.Spec.Scheduler.Service.Enabled {
		logger.Info("Scheduler operand is disabled")

		s.lastDesiredState = []client.Object{}

		return nil, nil
	}

	serviceAccount, err := serviceAccountForKAIConfig(ctx, readerClient, kaiConfig)
	if err != nil {
		return nil, err
	}

	s.lastDesiredState = []client.Object{serviceAccount}
	return s.lastDesiredState, nil
}

func (s *SchedulerForConfig) IsAvailable(ctx context.Context, readerClient client.Reader) (bool, error) {
	return common.AllControllersAvailable(ctx, readerClient, s.lastDesiredState)
}

func (s *SchedulerForConfig) IsDeployed(ctx context.Context, readerClient client.Reader) (bool, error) {
	return common.AllObjectsExists(ctx, readerClient, s.lastDesiredState)
}

func (s *SchedulerForConfig) Name() string {
	return "SchedulerForConfig"
}
