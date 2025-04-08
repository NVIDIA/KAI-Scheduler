// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package discovery

import (
	"context"
	"fmt"

	"github.com/NVIDIA/go-nvml/pkg/nvml"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

func GetGPUDevice(ctx context.Context) (string, error) {
	logger := log.FromContext(ctx)

	ret := nvml.Init()
	if ret != nvml.SUCCESS {
		return "", fmt.Errorf("unable to initialize NVML: %v", nvml.ErrorString(ret))
	}
	defer func() {
		ret := nvml.Shutdown()
		if ret != nvml.SUCCESS {
			logger.Info("Unable to shutdown NVML", "error", nvml.ErrorString(ret))
		}
	}()

	count, ret := nvml.DeviceGetCount()
	if ret != nvml.SUCCESS {
		return "", fmt.Errorf("unable to get device count: %v", nvml.ErrorString(ret))
	}

	logger.Info(fmt.Sprintf("Found %d GPU devices", count))
	if count != 1 {
		return "", fmt.Errorf("found %d devices, 1 was expected", count)
	}

	device, ret := nvml.DeviceGetHandleByIndex(0)
	if ret != nvml.SUCCESS {
		return "", fmt.Errorf("unable to get device handle: %v", nvml.ErrorString(ret))
	}

	uuid, ret := device.GetUUID()
	if ret != nvml.SUCCESS {
		return "", fmt.Errorf("unable to get device uuid: %v", nvml.ErrorString(ret))
	}

	return uuid, nil
}
