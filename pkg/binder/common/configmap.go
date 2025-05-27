// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package common

import (
	"context"
	"fmt"

	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	ldPreloadVolumeMountSubPath = "ld.so.preload-key"
)

func GetConfigMapName(pod *v1.Pod, container *v1.Container) (string, error) {
	if mapName := getConfigMapNameByEnvVar(container); mapName != "" {
		return mapName, nil
	}
	if mapName := getConfigMapByMounts(pod, container); mapName != "" {
		return mapName, nil
	}
	return "", fmt.Errorf("failed to find configmap for pod %s/%s, container %s",
		pod.Namespace, pod.Name, container.Name)
}

func getConfigMapByMounts(pod *v1.Pod, container *v1.Container) string {
	for _, volumeMount := range container.VolumeMounts {
		if volumeMount.SubPath == ldPreloadVolumeMountSubPath && volumeMount.Name != "" {
			for _, volume := range pod.Spec.Volumes {
				if volume.Name == volumeMount.Name && volume.ConfigMap != nil {
					return volume.ConfigMap.Name
				}
			}
		}
	}
	return ""
}

func getConfigMapNameByEnvVar(container *v1.Container) string {
	for _, envVar := range container.Env {
		if envVar.Name == NumOfGpusEnvVar && envVar.ValueFrom != nil &&
			envVar.ValueFrom.ConfigMapKeyRef != nil {
			return envVar.ValueFrom.ConfigMapKeyRef.Name
		}
	}
	return ""
}

func GetDirectEnvVarConfigMapName(pod *v1.Pod, container *v1.Container) (string, error) {
	configNameBase, err := GetConfigMapName(pod, container)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s-evar", configNameBase), nil
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
