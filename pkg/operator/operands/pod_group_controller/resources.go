// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package pod_group_controller

import (
	"context"
	"strconv"

	kaiConfigUtils "github.com/NVIDIA/KAI-scheduler/pkg/operator/config"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	kaiv1 "github.com/NVIDIA/KAI-scheduler/pkg/apis/kai/v1"
	"github.com/NVIDIA/KAI-scheduler/pkg/apis/kai/v1/pod_group_controller"
	"github.com/NVIDIA/KAI-scheduler/pkg/operator/operands/common"
)

const (
	deploymentName = "pod-group-controller"
)

func deploymentForKAIConfig(
	ctx context.Context, runtimeClient client.Reader, kaiConfig *kaiv1.Config,
) (client.Object, error) {

	deployment, err := common.DeploymentForKAIConfig(ctx, runtimeClient, kaiConfig, deploymentName)
	if err != nil {
		return nil, err
	}

	config := kaiConfig.Spec.PodGroupController
	deployment.Spec.Replicas = config.Replicas
	deployment.Spec.Template.Spec = v1.PodSpec{
		ServiceAccountName: deploymentName,
		Affinity:           kaiConfig.Spec.Global.Affinity,
		Tolerations:        kaiConfig.Spec.Global.Tolerations,
		Containers: []v1.Container{
			{
				Name:            deploymentName,
				Image:           config.Service.Image.Url(),
				ImagePullPolicy: *config.Service.Image.PullPolicy,
				Resources:       v1.ResourceRequirements(*config.Service.Resources),
				Args:            buildArgsList(config, *kaiConfig.Spec.Global.SchedulerName),
				SecurityContext: kaiConfig.Spec.Global.GetSecurityContext(),
			},
		},
		ImagePullSecrets: kaiConfigUtils.GetGlobalImagePullSecrets(kaiConfig.Spec.Global),
	}

	return deployment, nil
}

func serviceAccountForKAIConfig(
	ctx context.Context, k8sReader client.Reader, kaiConfig *kaiv1.Config,
) (client.Object, error) {
	sa, err := common.ObjectForKAIConfig(ctx, k8sReader, &v1.ServiceAccount{}, deploymentName,
		kaiConfig.Spec.Namespace)
	if err != nil {
		return nil, err
	}
	sa.(*v1.ServiceAccount).TypeMeta = metav1.TypeMeta{
		Kind:       "ServiceAccount",
		APIVersion: "v1",
	}
	return sa, err
}

func buildArgsList(config *pod_group_controller.PodGroupController, schedulerName string) []string {
	args := []string{
		"--scheduler-name", schedulerName,
	}

	if config.Service.K8sClientConfig != nil {
		k8sClientConfig := config.Service.K8sClientConfig
		if k8sClientConfig.QPS != nil {
			args = append(args, "--qps", strconv.Itoa(*k8sClientConfig.QPS))
		}
		if k8sClientConfig.Burst != nil {
			args = append(args, "--burst", strconv.Itoa(*k8sClientConfig.Burst))
		}
	}

	if config.MaxConcurrentReconciles != nil {
		args = append(args, "--max-concurrent-reconciles", strconv.Itoa(*config.MaxConcurrentReconciles))
	}

	if config.Args.InferencePreemptible != nil {
		args = append(args, "--inference-preemptible", strconv.FormatBool(*config.Args.InferencePreemptible))
	}

	if config.Replicas != nil && *config.Replicas > 1 {
		args = append(args, "--leader-elect")
	}

	return args
}
