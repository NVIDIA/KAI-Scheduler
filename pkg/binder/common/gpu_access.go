// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package common

import (
	"context"
	"fmt"

	v1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func AddVisibleDevicesEnvVars(container *v1.Container, sharedGpuConfigMapName string) {
	AddEnvVarToContainer(container, v1.EnvVar{
		Name: NvidiaVisibleDevices,
		ValueFrom: &v1.EnvVarSource{
			ConfigMapKeyRef: &v1.ConfigMapKeySelector{
				Key: VisibleDevices,
				LocalObjectReference: v1.LocalObjectReference{
					Name: sharedGpuConfigMapName,
				},
			},
		},
	})

	AddEnvVarToContainer(container, v1.EnvVar{
		Name: NumOfGpusEnvVar,
		ValueFrom: &v1.EnvVarSource{
			ConfigMapKeyRef: &v1.ConfigMapKeySelector{
				Key: NumOfGpusEnvVar,
				LocalObjectReference: v1.LocalObjectReference{
					Name: sharedGpuConfigMapName,
				},
			},
		},
	})
}

func SetNvidiaVisibleDevices(ctx context.Context, kubeClient client.Client, pod *v1.Pod,
	container *v1.Container, visibleDevicesValue string) error {
	nvidiaVisibleDevicesDefinedInSpec := false
	for _, envVar := range container.Env {
		if envVar.Name == NumOfGpusEnvVar && envVar.ValueFrom != nil &&
			envVar.ValueFrom.ConfigMapKeyRef != nil {
			nvidiaVisibleDevicesDefinedInSpec = true
		}
	}
	if nvidiaVisibleDevicesDefinedInSpec {
		capabilitiesMapName, err := GetConfigMapName(pod, container)
		if err != nil {
			return err
		}
		err = UpdateConfigMapEnvironmentVariable(ctx, kubeClient, pod, capabilitiesMapName,
			func(data map[string]string) error {
				data[VisibleDevices] = visibleDevicesValue
				return nil
			})
		return err
	}

	envVarMapName, err := GetDirectEnvVarConfigMapName(pod, container)
	if err != nil {
		return err
	}
	err = UpdateConfigMapEnvironmentVariable(ctx, kubeClient, pod, envVarMapName,
		func(data map[string]string) error {
			data[NvidiaVisibleDevices] = visibleDevicesValue
			return nil
		})
	return err
}

func SetNumOfGPUDevices(
	ctx context.Context, kubeClient client.Client, pod *v1.Pod, container *v1.Container, numOfGPUs string,
) error {
	updateFunc := func(data map[string]string) error {
		data[NumOfGpusEnvVar] = numOfGPUs
		return nil
	}
	configMapName, err := GetConfigMapName(pod, container)
	if err != nil {
		return err
	}

	err = UpdateConfigMapEnvironmentVariable(ctx, kubeClient, pod, configMapName, updateFunc)
	if err != nil {
		return fmt.Errorf("failed to update gpu sharing configmap for pod <%s/%s>: %v",
			pod.Namespace, pod.Name, err)
	}
	return nil
}
