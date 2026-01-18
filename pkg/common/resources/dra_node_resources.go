// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package resources

import (
	resourceapi "k8s.io/api/resource/v1"
)

// CountNodeGPUsFromResourceSlices counts GPU devices from ResourceSlices for a specific node.
// It filters ResourceSlices by nodeName and counts devices where the DeviceClass is a GPU class.
// Returns the total GPU count for that node.
func CountNodeGPUsFromResourceSlices(nodeName string, slices []*resourceapi.ResourceSlice) int64 {
	totalGPUs := int64(0)

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

		// Count devices in this slice
		totalGPUs += countDevicesInSlice(slice)
	}

	return totalGPUs
}

// isSliceForNode checks if a ResourceSlice is associated with a specific node.
func isSliceForNode(slice *resourceapi.ResourceSlice, nodeName string) bool {
	// Check if NodeName is set and matches
	if slice.Spec.NodeName != nil && *slice.Spec.NodeName != "" {
		return *slice.Spec.NodeName == nodeName
	}
	// If AllNodes is set and true, this slice applies to all nodes
	if slice.Spec.AllNodes != nil && *slice.Spec.AllNodes {
		return true
	}
	// NodeSelector case - for simplicity, we don't evaluate complex selectors here
	// This would require the full node object to match against
	return false
}

// countDevicesInSlice counts the number of devices in a ResourceSlice.
func countDevicesInSlice(slice *resourceapi.ResourceSlice) int64 {
	if slice.Spec.Devices == nil {
		return 0
	}
	return int64(len(slice.Spec.Devices))
}

// CountNodeGPUsFromResourceSlicesByDeviceClass counts GPU devices from ResourceSlices for a specific node,
// grouped by device class. This is useful when you need to track different GPU types separately.
// Returns a map of device class name to GPU count.
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
