// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package resources

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	resourceapi "k8s.io/api/resource/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

func TestCountNodeGPUsFromResourceSlices(t *testing.T) {
	tests := []struct {
		name     string
		nodeName string
		slices   []*resourceapi.ResourceSlice
		expected int64
	}{
		{
			name:     "Single DRA resource - 4 GPUs",
			nodeName: "node-1",
			slices: []*resourceapi.ResourceSlice{
				createResourceSlice("slice-1", "node-1", "nvidia.com/gpu", 4),
			},
			expected: 4,
		},
		{
			name:     "Two different DRA device classes - nvidia and amd",
			nodeName: "node-1",
			slices: []*resourceapi.ResourceSlice{
				createResourceSlice("slice-nvidia", "node-1", "nvidia.com/gpu", 4),
				createResourceSlice("slice-amd", "node-1", "generic.com/gpu", 2),
			},
			expected: 6,
		},
		{
			name:     "Multiple nodes - filter by nodeName",
			nodeName: "node-1",
			slices: []*resourceapi.ResourceSlice{
				createResourceSlice("slice-1", "node-1", "nvidia.com/gpu", 4),
				createResourceSlice("slice-2", "node-2", "nvidia.com/gpu", 8),
				createResourceSlice("slice-3", "node-3", "nvidia.com/gpu", 2),
			},
			expected: 4, // Only node-1's GPUs
		},
		{
			name:     "No ResourceSlices",
			nodeName: "node-1",
			slices:   []*resourceapi.ResourceSlice{},
			expected: 0,
		},
		{
			name:     "ResourceSlice for different node",
			nodeName: "node-1",
			slices: []*resourceapi.ResourceSlice{
				createResourceSlice("slice-1", "node-2", "nvidia.com/gpu", 4),
			},
			expected: 0,
		},
		{
			name:     "Non-GPU device class is ignored",
			nodeName: "node-1",
			slices: []*resourceapi.ResourceSlice{
				createResourceSlice("slice-1", "node-1", "nvidia.com/gpu", 4),
				createResourceSlice("slice-2", "node-1", "some.other/device", 8),
			},
			expected: 4, // Only GPU devices counted
		},
		{
			name:     "AllNodes ResourceSlice is not counted for specific node",
			nodeName: "node-1",
			slices: []*resourceapi.ResourceSlice{
				createAllNodesResourceSlice("slice-all", "nvidia.com/gpu", 2),
			},
			expected: 0, // AllNodes slices are handled separately via ListResourceSlicesByNode
		},
		{
			name:     "Mixed node-specific and AllNodes slices counts only node-specific",
			nodeName: "node-1",
			slices: []*resourceapi.ResourceSlice{
				createResourceSlice("slice-1", "node-1", "nvidia.com/gpu", 4),
				createAllNodesResourceSlice("slice-all", "nvidia.com/gpu", 2),
			},
			expected: 4, // Only node-specific slice is counted
		},
		{
			name:     "Nil slice in list is handled",
			nodeName: "node-1",
			slices: []*resourceapi.ResourceSlice{
				createResourceSlice("slice-1", "node-1", "nvidia.com/gpu", 4),
				nil,
				createResourceSlice("slice-2", "node-1", "nvidia.com/gpu", 2),
			},
			expected: 6,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CountNodeGPUsFromResourceSlices(tt.nodeName, tt.slices)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// createResourceSlice creates a ResourceSlice for testing.
func createResourceSlice(name, nodeName, driver string, deviceCount int) *resourceapi.ResourceSlice {
	devices := make([]resourceapi.Device, deviceCount)
	for i := 0; i < deviceCount; i++ {
		devices[i] = resourceapi.Device{
			Name: fmt.Sprintf("device-%d", i),
		}
	}

	return &resourceapi.ResourceSlice{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: resourceapi.ResourceSliceSpec{
			NodeName: ptr.To(nodeName),
			Driver:   driver,
			Devices:  devices,
		},
	}
}

func createAllNodesResourceSlice(name, driver string, deviceCount int) *resourceapi.ResourceSlice {
	devices := make([]resourceapi.Device, deviceCount)
	for i := 0; i < deviceCount; i++ {
		devices[i] = resourceapi.Device{
			Name: fmt.Sprintf("device-%d", i),
		}
	}

	return &resourceapi.ResourceSlice{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: resourceapi.ResourceSliceSpec{
			AllNodes: ptr.To(true),
			Driver:   driver,
			Devices:  devices,
		},
	}
}

func TestMapResourceSlicesToNodes(t *testing.T) {
	tests := []struct {
		name     string
		slices   []*resourceapi.ResourceSlice
		expected NodeGPUsByDeviceClass
	}{
		{
			name:     "Empty slices",
			slices:   []*resourceapi.ResourceSlice{},
			expected: NodeGPUsByDeviceClass{},
		},
		{
			name: "Single node with single device class",
			slices: []*resourceapi.ResourceSlice{
				createResourceSlice("slice-1", "node-1", "nvidia.com/gpu", 4),
			},
			expected: NodeGPUsByDeviceClass{
				"node-1": {"nvidia.com/gpu": 4},
			},
		},
		{
			name: "Single node with multiple device classes",
			slices: []*resourceapi.ResourceSlice{
				createResourceSlice("slice-nvidia", "node-1", "nvidia.com/gpu", 4),
				createResourceSlice("slice-amd", "node-1", "generic.com/gpu", 2),
			},
			expected: NodeGPUsByDeviceClass{
				"node-1": {"nvidia.com/gpu": 4, "generic.com/gpu": 2},
			},
		},
		{
			name: "Multiple nodes with different slices",
			slices: []*resourceapi.ResourceSlice{
				createResourceSlice("slice-1", "node-1", "nvidia.com/gpu", 4),
				createResourceSlice("slice-2", "node-2", "nvidia.com/gpu", 8),
				createResourceSlice("slice-3", "node-3", "generic.com/gpu", 2),
			},
			expected: NodeGPUsByDeviceClass{
				"node-1": {"nvidia.com/gpu": 4},
				"node-2": {"nvidia.com/gpu": 8},
				"node-3": {"generic.com/gpu": 2},
			},
		},
		{
			name: "Multiple slices for same node and device class are summed",
			slices: []*resourceapi.ResourceSlice{
				createResourceSlice("slice-1", "node-1", "nvidia.com/gpu", 4),
				createResourceSlice("slice-2", "node-1", "nvidia.com/gpu", 4),
			},
			expected: NodeGPUsByDeviceClass{
				"node-1": {"nvidia.com/gpu": 8},
			},
		},
		{
			name: "Non-GPU device classes are ignored",
			slices: []*resourceapi.ResourceSlice{
				createResourceSlice("slice-1", "node-1", "nvidia.com/gpu", 4),
				createResourceSlice("slice-2", "node-1", "some.other/device", 8),
			},
			expected: NodeGPUsByDeviceClass{
				"node-1": {"nvidia.com/gpu": 4},
			},
		},
		{
			name: "AllNodes slices are not included in node mapping",
			slices: []*resourceapi.ResourceSlice{
				createResourceSlice("slice-1", "node-1", "nvidia.com/gpu", 4),
				createAllNodesResourceSlice("slice-all", "nvidia.com/gpu", 2),
			},
			expected: NodeGPUsByDeviceClass{
				"node-1": {"nvidia.com/gpu": 4},
			},
		},
		{
			name: "Nil slices are handled",
			slices: []*resourceapi.ResourceSlice{
				createResourceSlice("slice-1", "node-1", "nvidia.com/gpu", 4),
				nil,
				createResourceSlice("slice-2", "node-2", "nvidia.com/gpu", 2),
			},
			expected: NodeGPUsByDeviceClass{
				"node-1": {"nvidia.com/gpu": 4},
				"node-2": {"nvidia.com/gpu": 2},
			},
		},
		{
			name: "Slices with zero devices are ignored",
			slices: []*resourceapi.ResourceSlice{
				createResourceSlice("slice-1", "node-1", "nvidia.com/gpu", 4),
				createResourceSlice("slice-empty", "node-1", "nvidia.com/gpu", 0),
			},
			expected: NodeGPUsByDeviceClass{
				"node-1": {"nvidia.com/gpu": 4},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := MapResourceSlicesToNodes(tt.slices)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetAllNodesSlices(t *testing.T) {
	tests := []struct {
		name          string
		slices        []*resourceapi.ResourceSlice
		expectedCount int
	}{
		{
			name:          "Empty slices",
			slices:        []*resourceapi.ResourceSlice{},
			expectedCount: 0,
		},
		{
			name: "No AllNodes slices",
			slices: []*resourceapi.ResourceSlice{
				createResourceSlice("slice-1", "node-1", "nvidia.com/gpu", 4),
				createResourceSlice("slice-2", "node-2", "nvidia.com/gpu", 2),
			},
			expectedCount: 0,
		},
		{
			name: "Single AllNodes slice",
			slices: []*resourceapi.ResourceSlice{
				createAllNodesResourceSlice("slice-all", "nvidia.com/gpu", 2),
			},
			expectedCount: 1,
		},
		{
			name: "Multiple AllNodes slices",
			slices: []*resourceapi.ResourceSlice{
				createAllNodesResourceSlice("slice-all-1", "nvidia.com/gpu", 2),
				createAllNodesResourceSlice("slice-all-2", "generic.com/gpu", 4),
			},
			expectedCount: 2,
		},
		{
			name: "Mixed node-specific and AllNodes slices",
			slices: []*resourceapi.ResourceSlice{
				createResourceSlice("slice-1", "node-1", "nvidia.com/gpu", 4),
				createAllNodesResourceSlice("slice-all", "nvidia.com/gpu", 2),
				createResourceSlice("slice-2", "node-2", "nvidia.com/gpu", 8),
			},
			expectedCount: 1,
		},
		{
			name: "Non-GPU AllNodes slices are ignored",
			slices: []*resourceapi.ResourceSlice{
				createAllNodesResourceSlice("slice-all-gpu", "nvidia.com/gpu", 2),
				createAllNodesResourceSlice("slice-all-other", "some.other/device", 4),
			},
			expectedCount: 1,
		},
		{
			name: "Nil slices are handled",
			slices: []*resourceapi.ResourceSlice{
				nil,
				createAllNodesResourceSlice("slice-all", "nvidia.com/gpu", 2),
				nil,
			},
			expectedCount: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetAllNodesSlices(tt.slices)
			assert.Len(t, result, tt.expectedCount)
		})
	}
}
