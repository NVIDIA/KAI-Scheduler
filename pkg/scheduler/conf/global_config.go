// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package conf

import (
	"sync"
)

var (
	config      *GlobalConfig
	configMutex sync.Mutex
)

type GlobalConfig struct {
	ResourceReservationAppLabelValue string
	ScalingPodAppLabelValue          string
}

func newGlobalConfig() *GlobalConfig {
	return &GlobalConfig{
		ResourceReservationAppLabelValue: "kai-resource-reservation",
		ScalingPodAppLabelValue:          "scaling-pod",
	}
}

func GetConfig() *GlobalConfig {
	configMutex.Lock()
	defer configMutex.Unlock()

	if config == nil {
		config = newGlobalConfig()
	}
	return config
}
