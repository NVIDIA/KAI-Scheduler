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

		if !IsGPUDeviceClass(slice.Spec.Driver) {
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
	return false
}

func countDevicesInSlice(slice *resourceapi.ResourceSlice) int64 {
	if slice.Spec.Devices == nil {
		return 0
	}
	return int64(len(slice.Spec.Devices))
}

// NodeGPUsByDeviceClass maps node name to device class to GPU count
type NodeGPUsByDeviceClass map[string]map[string]int64

func MapResourceSlicesToNodes(slices []*resourceapi.ResourceSlice) NodeGPUsByDeviceClass {
	result := make(NodeGPUsByDeviceClass)

	for _, slice := range slices {
		if slice == nil {
			continue
		}

		if !IsGPUDeviceClass(slice.Spec.Driver) {
			continue
		}

		deviceCount := countDevicesInSlice(slice)
		if deviceCount == 0 {
			continue
		}

		nodeNames := getNodeNamesForSlice(slice)
		for _, nodeName := range nodeNames {
			if result[nodeName] == nil {
				result[nodeName] = make(map[string]int64)
			}
			result[nodeName][slice.Spec.Driver] += deviceCount
		}
	}

	return result
}

// getNodeNamesForSlice returns the node names this slice applies to.
// For AllNodes slices, returns empty slice (caller should handle specially).
// For node-specific slices, returns a single-element slice.
func getNodeNamesForSlice(slice *resourceapi.ResourceSlice) []string {
	if slice.Spec.NodeName != nil && *slice.Spec.NodeName != "" {
		return []string{*slice.Spec.NodeName}
	}
	// AllNodes slices are not returned here - they need special handling
	return nil
}

// GetAllNodesSlices returns slices that apply to all nodes
func GetAllNodesSlices(slices []*resourceapi.ResourceSlice) []*resourceapi.ResourceSlice {
	var allNodesSlices []*resourceapi.ResourceSlice
	for _, slice := range slices {
		if slice != nil && slice.Spec.AllNodes != nil && *slice.Spec.AllNodes {
			if IsGPUDeviceClass(slice.Spec.Driver) {
				allNodesSlices = append(allNodesSlices, slice)
			}
		}
	}
	return allNodesSlices
}
