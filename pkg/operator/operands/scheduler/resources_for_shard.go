// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package scheduler

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"golang.org/x/exp/slices"

	"gopkg.in/yaml.v3"
	v1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/NVIDIA/KAI-scheduler/cmd/scheduler/app/options"
	kaiv1 "github.com/NVIDIA/KAI-scheduler/pkg/apis/kai/v1"
	kaiConfigUtils "github.com/NVIDIA/KAI-scheduler/pkg/operator/config"
	"github.com/NVIDIA/KAI-scheduler/pkg/operator/operands/common"
	"github.com/spf13/pflag"
)

const (
	invalidJobDepthMapError = "the scheduler's actions are %s. %s isn't one of them, making the queueDepthPerAction invalid"
)

func (s *SchedulerForShard) deploymentForShard(
	ctx context.Context, readerClient client.Reader,
	kaiConfig *kaiv1.Config, shard *kaiv1.SchedulingShard,
) (client.Object, error) {
	shardDeploymentName := deploymentName(kaiConfig, shard)
	config := kaiConfig.Spec.Scheduler

	deployment, err := common.DeploymentForKAIConfig(ctx, readerClient, kaiConfig, config.Service, shardDeploymentName)
	if err != nil {
		return nil, err
	}
	cmObject, err := common.ObjectForKAIConfig(ctx, readerClient, &corev1.ConfigMap{}, configMapName(kaiConfig, shard),
		kaiConfig.Spec.Namespace)
	if err != nil {
		return nil, err
	}
	schedulerConfig := cmObject.(*corev1.ConfigMap)

	containerArgs, err := buildArgsList(
		shard, kaiConfig, configMountPath,
	)
	if err != nil {
		return nil, err
	}

	deployment.Spec.Replicas = config.Replicas
	deployment.Spec.Selector = &metav1.LabelSelector{
		MatchLabels: map[string]string{
			"app": shardDeploymentName,
		},
	}
	deployment.Spec.Strategy.Type = v1.RecreateDeploymentStrategyType
	deployment.Spec.Strategy.RollingUpdate = nil
	deployment.Spec.Template.ObjectMeta = metav1.ObjectMeta{
		Name: shardDeploymentName,
		Labels: map[string]string{
			"app": shardDeploymentName,
		},
		Annotations: map[string]string{
			"configMapVersion": schedulerConfig.ResourceVersion,
		},
	}
	deployment.Spec.Template.Spec.ServiceAccountName = s.BaseResourceName
	deployment.Spec.Template.Spec.Containers[0].VolumeMounts = []corev1.VolumeMount{
		{
			MountPath: configMountPath,
			Name:      "config",
			SubPath:   "config.yaml",
		},
	}
	deployment.Spec.Template.Spec.Containers[0].Env = []corev1.EnvVar{
		{
			Name:  "GOGC",
			Value: fmt.Sprintf("%d", *config.GOGC),
		},
		{
			Name: "NAMESPACE",
			ValueFrom: &corev1.EnvVarSource{
				FieldRef: &corev1.ObjectFieldSelector{
					FieldPath: "metadata.namespace",
				},
			},
		},
	}
	deployment.Spec.Template.Spec.Containers[0].Args = containerArgs
	deployment.Spec.Template.Spec.Volumes = []corev1.Volume{
		{
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: configMapName(kaiConfig, shard),
					},
				},
			},
			Name: "config",
		},
	}

	return deployment, nil
}

func (s *SchedulerForShard) configMapForShard(
	ctx context.Context, readerClient client.Reader,
	kaiConfig *kaiv1.Config, shard *kaiv1.SchedulingShard,
) (client.Object, error) {
	cmObject, err := common.ObjectForKAIConfig(ctx, readerClient, &corev1.ConfigMap{}, configMapName(kaiConfig, shard),
		kaiConfig.Spec.Namespace)
	if err != nil {
		return nil, err
	}
	schedulerConfig := cmObject.(*corev1.ConfigMap)
	schedulerConfig.TypeMeta = metav1.TypeMeta{
		Kind:       "ConfigMap",
		APIVersion: "v1",
	}
	placementArguments := calculatePlacementArguments(shard.Spec.PlacementStrategy)
	innerConfig := config{}

	actions := []string{"allocate"}
	if placementArguments[gpuResource] != spreadStrategy && placementArguments[cpuResource] != spreadStrategy {
		actions = append(actions, "consolidation")
	}
	actions = append(actions, []string{"reclaim", "preempt", "stalegangeviction"}...)

	innerConfig.Actions = strings.Join(actions, ", ")

	innerConfig.Tiers = []tier{
		{
			Plugins: []plugin{
				{Name: "predicates"},
				{Name: "proportion"},
				{Name: "priority"},
				{Name: "nodeavailability"},
				{Name: "resourcetype"},
				{Name: "podaffinity"},
				{Name: "elastic"},
				{Name: "kubeflow"},
				{Name: "ray"},
				{Name: "subgrouporder"},
				{Name: "taskorder"},
				{Name: "nominatednode"},
				{Name: "dynamicresources"},
				{Name: "minruntime"},
				{Name: "topology"},
				{Name: "snapshot"},
			},
		},
	}

	innerConfig.Tiers[0].Plugins = append(
		innerConfig.Tiers[0].Plugins,
		plugin{Name: fmt.Sprintf("gpu%s", strings.Replace(placementArguments[gpuResource], "bin", "", 1))},
		plugin{
			Name:      "nodeplacement",
			Arguments: placementArguments,
		},
	)

	if placementArguments[gpuResource] == binpackStrategy {
		innerConfig.Tiers[0].Plugins = append(
			innerConfig.Tiers[0].Plugins,
			plugin{Name: "gpusharingorder"},
		)
	}

	addMinRuntimePluginIfNeeded(&innerConfig.Tiers[0].Plugins, shard.Spec.MinRuntime)

	if len(shard.Spec.QueueDepthPerAction) > 0 {
		if err = validateJobDepthMap(shard, innerConfig, actions); err != nil {
			return nil, err
		}
		// Set the validated map to the scheduler config
		innerConfig.QueueDepthPerAction = shard.Spec.QueueDepthPerAction
	}

	data, marshalErr := yaml.Marshal(&innerConfig)
	if marshalErr != nil {
		return nil, marshalErr
	}
	schedulerConfig.Data = map[string]string{
		"config.yaml": string(data),
	}

	return schedulerConfig, nil
}

func validateJobDepthMap(shard *kaiv1.SchedulingShard, innerConfig config, actions []string) error {
	for actionToConfigure := range shard.Spec.QueueDepthPerAction {
		if !slices.Contains(actions, actionToConfigure) {
			return fmt.Errorf(invalidJobDepthMapError, innerConfig.Actions, actionToConfigure)
		}
	}
	return nil
}

func (s *SchedulerForShard) serviceForShard(
	ctx context.Context, readerClient client.Reader,
	kaiConfig *kaiv1.Config, shard *kaiv1.SchedulingShard,
) (client.Object, error) {
	serviceName := fmt.Sprintf("%s-%s", *kaiConfig.Spec.Global.SchedulerName, shard.Name)
	serviceObj, err := common.ObjectForKAIConfig(ctx, readerClient, &corev1.Service{}, serviceName,
		kaiConfig.Spec.Namespace)
	if err != nil {
		return nil, err
	}
	schedulerConfig := kaiConfig.Spec.Scheduler

	service := serviceObj.(*corev1.Service)
	service.TypeMeta = metav1.TypeMeta{
		Kind:       "Service",
		APIVersion: "v1",
	}

	if service.Annotations == nil {
		service.Annotations = map[string]string{}
	}
	service.Annotations["prometheus.io/scrape"] = "true"

	service.Spec.ClusterIP = "None"
	service.Spec.Ports = []corev1.ServicePort{
		{
			Name:       "http-metrics",
			Port:       int32(*schedulerConfig.SchedulerService.Port),
			Protocol:   corev1.ProtocolTCP,
			TargetPort: intstr.FromInt(*schedulerConfig.SchedulerService.TargetPort),
		},
	}
	service.Spec.Selector = map[string]string{
		"app": serviceName,
	}
	service.Spec.SessionAffinity = corev1.ServiceAffinityNone
	service.Spec.Type = *schedulerConfig.SchedulerService.Type

	return service, err
}

func buildArgsList(
	shard *kaiv1.SchedulingShard, kaiConfig *kaiv1.Config, configName string,
) ([]string, error) {
	so := options.NewServerOption()
	flagSet := pflag.NewFlagSet("fake", pflag.ContinueOnError)
	so.AddFlags(flagSet)

	args := []string{
		fmt.Sprintf("--%s=%s", "scheduler-conf", configName),
		fmt.Sprintf("--%s=%s", "scheduler-name", *kaiConfig.Spec.Global.SchedulerName),
		fmt.Sprintf("--%s=%s", "namespace", kaiConfig.Spec.Namespace),
		fmt.Sprintf("--%s=%s", "nodepool-label-key", *kaiConfig.Spec.Global.NodePoolLabelKey),
		fmt.Sprintf("--%s=%s", "partition-label-value", shard.Spec.PartitionLabelValue),
		fmt.Sprintf("--%s=%s", "resource-reservation-app-label", *kaiConfig.Spec.Binder.ResourceReservation.AppLabel),
	}

	if kaiConfig.Spec.Scheduler.SchedulerService.Port != nil {
		portNumberString := strconv.Itoa(*kaiConfig.Spec.Scheduler.SchedulerService.Port)
		args = append(args, fmt.Sprintf("--%s=:%s", "listen-address", portNumberString))
	}

	if kaiConfig.Spec.QueueController.MetricsNamespace != nil {
		args = append(args, fmt.Sprintf("--%s=%s", "metrics-namespace", *kaiConfig.Spec.QueueController.MetricsNamespace))
	}

	// Dynamically apply valid scheduler flags from shard args, ignoring unknown flags
	flagSet.VisitAll(func(flag *pflag.Flag) {
		if value, found := shard.Spec.Args[flag.Name]; found {
			args = append(args, fmt.Sprintf("--%s=%v", flag.Name, value))
		}
	})

	if featureGates := kaiConfigUtils.FeatureGatesArg(); featureGates != "" {
		args = append(args, featureGates)
	}
	schedulerConfig := kaiConfig.Spec.Scheduler
	if schedulerConfig.Replicas != nil && *schedulerConfig.Replicas > 1 {
		args = append(args, "--leader-elect=true")
	}

	return args, nil
}

func calculatePlacementArguments(placementStrategy *kaiv1.PlacementStrategy) map[string]string {
	return map[string]string{
		gpuResource: *placementStrategy.GPU, cpuResource: *placementStrategy.CPU,
	}
}

func addMinRuntimePluginIfNeeded(plugins *[]plugin, minRuntime *kaiv1.MinRuntime) {
	if minRuntime == nil || (minRuntime.PreemptMinRuntime == nil && minRuntime.ReclaimMinRuntime == nil) {
		return
	}

	minRuntimePlugin := plugin{Name: "minruntime", Arguments: map[string]string{}}

	if minRuntime.PreemptMinRuntime != nil {
		minRuntimePlugin.Arguments["defaultPreemptMinRuntime"] = *minRuntime.PreemptMinRuntime
	}
	if minRuntime.ReclaimMinRuntime != nil {
		minRuntimePlugin.Arguments["defaultReclaimMinRuntime"] = *minRuntime.ReclaimMinRuntime
	}

	*plugins = append(*plugins, minRuntimePlugin)
}

func configMapName(config *kaiv1.Config, shard *kaiv1.SchedulingShard) string {
	return fmt.Sprintf("%s-%s", *config.Spec.Global.SchedulerName, shard.Name)
}

func deploymentName(config *kaiv1.Config, shard *kaiv1.SchedulingShard) string {
	return fmt.Sprintf("%s-%s", *config.Spec.Global.SchedulerName, shard.Name)
}
