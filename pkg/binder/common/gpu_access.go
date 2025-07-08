// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package common

import (
	"context"
	"fmt"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/NVIDIA/KAI-scheduler/pkg/binder/common/gpusharingconfigmap"
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

func SetNvidiaVisibleDevices(
	ctx context.Context, kubeClient client.Client, pod *v1.Pod, containerRef *gpusharingconfigmap.PodContainerRef,
	visibleDevicesValue string,
) error {
	nvidiaVisibleDevicesDefinedInSpec := false
	for _, envVar := range containerRef.Container.Env {
		if envVar.Name == NvidiaVisibleDevices && envVar.ValueFrom != nil &&
			envVar.ValueFrom.ConfigMapKeyRef != nil {
			nvidiaVisibleDevicesDefinedInSpec = true
		}
	}

	var configMapName string
	var err error
	var updateFunc func(data map[string]string) error
	if nvidiaVisibleDevicesDefinedInSpec {
		configMapName, err = gpusharingconfigmap.ExtractCapabilitiesConfigMapName(pod, containerRef)
		updateFunc = func(data map[string]string) error {
			data[VisibleDevices] = visibleDevicesValue
			return nil
		}
	} else {
		configMapName, err = gpusharingconfigmap.ExtractDirectEnvVarsConfigMapName(pod, containerRef)
		updateFunc = func(data map[string]string) error {
			data[NvidiaVisibleDevices] = visibleDevicesValue
			return nil
		}
	}
	if err != nil {
		return err
	}
	err = UpdateConfigMapEnvironmentVariable(ctx, kubeClient, pod, configMapName, updateFunc)
	if err != nil {
		return fmt.Errorf("failed to update gpu sharing configmap %s for pod <%s/%s>: %v",
			configMapName, pod.Namespace, pod.Name, err)
	}
	return nil
}

func SetNumOfGPUDevices(
	ctx context.Context, kubeClient client.Client, pod *v1.Pod, containerRef *gpusharingconfigmap.PodContainerRef,
	numOfGPUs string,
) error {
	updateFunc := func(data map[string]string) error {
		data[NumOfGpusEnvVar] = numOfGPUs
		return nil
	}
	capabilitiesMapName, err := gpusharingconfigmap.ExtractCapabilitiesConfigMapName(pod, containerRef)
	if err != nil {
		return err
	}

	err = UpdateConfigMapEnvironmentVariable(ctx, kubeClient, pod, capabilitiesMapName, updateFunc)
	if err != nil {
		return fmt.Errorf("failed to update gpu sharing configmap for pod <%s/%s>: %v",
			pod.Namespace, pod.Name, err)
	}
	return nil
}

func UpdateConfigMapEnvironmentVariable(
	ctx context.Context, kubeclient client.Client, task *v1.Pod,
	configMapName string, changesFunc func(map[string]string) error,
) error {
	logger := log.FromContext(ctx)
	logger.Info("Updating config map for job", "namespace", task.Namespace, "name", task.Name,
		"configMapName", configMapName)

	configMap := &v1.ConfigMap{}
	err := kubeclient.Get(ctx, types.NamespacedName{
		Name:      configMapName,
		Namespace: task.Namespace,
	}, configMap)
	if err != nil {
		logger.Error(err, "failed to get configMap", "configMapName", configMapName)
		return err
	}
	if configMap.Data == nil {
		configMap.Data = map[string]string{}
	}

	origConfigMap := configMap.DeepCopy()
	if err = changesFunc(configMap.Data); err != nil {
		return err
	}

	err = kubeclient.Patch(ctx, configMap, client.MergeFrom(origConfigMap))
	if err != nil {
		logger.Error(err, "failed to update config map", "configMapName",
			configMapName, "error", err.Error())
		return err
	}

	return nil
}
