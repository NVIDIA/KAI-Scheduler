// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package resources

import (
	resourceapi "k8s.io/api/resource/v1"
)

func CountNodeGPUsFromResourceSlices(nodeName string, slices []*resourceapi.ResourceSlice) int64 {
	totalGPUs := int64(0)

	for _, slice := range slices {
		if slice == nil {
			continue
		}

		if !isSliceForNode(slice, nodeName) {
			continue
		}

		if !isGPUDeviceClass(slice.Spec.Driver) {
			continue
		}

		totalGPUs += countDevicesInSlice(slice)
	}

	return totalGPUs
}

func isSliceForNode(slice *resourceapi.ResourceSlice, nodeName string) bool {
	if slice.Spec.NodeName != nil && *slice.Spec.NodeName != "" {
		return *slice.Spec.NodeName == nodeName
	}
	if slice.Spec.AllNodes != nil && *slice.Spec.AllNodes {
		return true
	}
	return false
}

// countDevicesInSlice counts the number of devices in a ResourceSlice.
func countDevicesInSlice(slice *resourceapi.ResourceSlice) int64 {
	if slice.Spec.Devices == nil {
		return 0
	}
	return int64(len(slice.Spec.Devices))
}

func CountNodeGPUsFromResourceSlicesByDeviceClass(nodeName string, slices []*resourceapi.ResourceSlice) map[string]int64 {
	gpusByClass := make(map[string]int64)

	for _, slice := range slices {
		if slice == nil {
			continue
		}

		// Check if this slice belongs to the target node
		if !isSliceForNode(slice, nodeName) {
			continue
		}

		// Check if this is a GPU device class
		if !isGPUDeviceClass(slice.Spec.Driver) {
			continue
		}

		// Count devices in this slice and add to the appropriate device class
		deviceCount := countDevicesInSlice(slice)
		if deviceCount > 0 {
			gpusByClass[slice.Spec.Driver] += deviceCount
		}
	}

	return gpusByClass
}
