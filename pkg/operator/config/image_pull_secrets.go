// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package config

import (
	kaiv1 "github.com/NVIDIA/KAI-scheduler/pkg/apis/kai/v1"
	v1 "k8s.io/api/core/v1"
)

func GetGlobalImagePullSecrets(globalConfig *kaiv1.GlobalConfig) []v1.LocalObjectReference {
	additionalImagePullSecrets := globalConfig.AdditionalImagePullSecrets
	var secretDeploymentObjs []v1.LocalObjectReference
	for _, secret := range additionalImagePullSecrets {
		deploymentObj := v1.LocalObjectReference{Name: secret}
		secretDeploymentObjs = append(secretDeploymentObjs, deploymentObj)
	}
	if globalConfig.ImagesPullSecret != nil && *globalConfig.ImagesPullSecret != "" {
		secretDeploymentObjs = append(secretDeploymentObjs, v1.LocalObjectReference{Name: *globalConfig.ImagesPullSecret})
	}
	return secretDeploymentObjs
}
