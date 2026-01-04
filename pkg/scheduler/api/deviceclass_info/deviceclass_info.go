// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package deviceclass_info

import (
	resourceapi "k8s.io/api/resource/v1"

	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/common_info"
)

type DeviceClassInfo struct {
	UID  common_info.DeviceClassID
	Name string
}

func NewDeviceClassInfo(deviceClass *resourceapi.DeviceClass) *DeviceClassInfo {
	return &DeviceClassInfo{
		UID:  common_info.DeviceClassID(deviceClass.UID),
		Name: deviceClass.Name,
	}
}
