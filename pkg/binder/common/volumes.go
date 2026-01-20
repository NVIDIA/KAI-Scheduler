// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package common

import (
	"fmt"

	v1 "k8s.io/api/core/v1"
)

func SetConfigMapVolume(pod *v1.Pod, configMapName string) {
	volumeName := GetConfigVolumeName(configMapName)
	addConfigMapVolume(&pod.Spec, volumeName, configMapName)
}

func addConfigMapVolume(podSpec *v1.PodSpec, volumeName string, configMapName string) {
	if podSpec.Volumes == nil {
		podSpec.Volumes = make([]v1.Volume, 0)
	}

	updatedVolumes := make([]v1.Volume, 0)
	for _, volume := range podSpec.Volumes {
		if volume.Name != volumeName {
			updatedVolumes = append(updatedVolumes, volume)
		}
	}
	podSpec.Volumes = updatedVolumes

	volume := v1.Volume{
		Name: volumeName,
		VolumeSource: v1.VolumeSource{
			ConfigMap: &v1.ConfigMapVolumeSource{
				LocalObjectReference: v1.LocalObjectReference{
					Name: configMapName,
				},
			},
		},
	}
	podSpec.Volumes = append(podSpec.Volumes, volume)
}

func GetConfigVolumeName(configMapName string) string {
	return fmt.Sprintf("%v-vol", configMapName)
}
