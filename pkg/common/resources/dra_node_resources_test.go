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
				createResourceSlice("slice-amd", "node-1", "amd.com/gpu", 2),
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
			name:     "AllNodes ResourceSlice applies to all nodes",
			nodeName: "node-1",
			slices: []*resourceapi.ResourceSlice{
				createAllNodesResourceSlice("slice-all", "nvidia.com/gpu", 2),
			},
			expected: 2,
		},
		{
			name:     "Mixed node-specific and AllNodes slices",
			nodeName: "node-1",
			slices: []*resourceapi.ResourceSlice{
				createResourceSlice("slice-1", "node-1", "nvidia.com/gpu", 4),
				createAllNodesResourceSlice("slice-all", "nvidia.com/gpu", 2),
			},
			expected: 6,
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

func TestCountNodeGPUsFromResourceSlicesByDeviceClass(t *testing.T) {
	tests := []struct {
		name     string
		nodeName string
		slices   []*resourceapi.ResourceSlice
		expected map[string]int64
	}{
		{
			name:     "Single device class",
			nodeName: "node-1",
			slices: []*resourceapi.ResourceSlice{
				createResourceSlice("slice-1", "node-1", "nvidia.com/gpu", 4),
			},
			expected: map[string]int64{"nvidia.com/gpu": 4},
		},
		{
			name:     "Two different device classes",
			nodeName: "node-1",
			slices: []*resourceapi.ResourceSlice{
				createResourceSlice("slice-nvidia", "node-1", "nvidia.com/gpu", 4),
				createResourceSlice("slice-amd", "node-1", "amd.com/gpu", 2),
			},
			expected: map[string]int64{
				"nvidia.com/gpu": 4,
				"amd.com/gpu":    2,
			},
		},
		{
			name:     "Multiple slices for same device class",
			nodeName: "node-1",
			slices: []*resourceapi.ResourceSlice{
				createResourceSlice("slice-1", "node-1", "nvidia.com/gpu", 4),
				createResourceSlice("slice-2", "node-1", "nvidia.com/gpu", 2),
			},
			expected: map[string]int64{"nvidia.com/gpu": 6},
		},
		{
			name:     "No slices for node",
			nodeName: "node-1",
			slices: []*resourceapi.ResourceSlice{
				createResourceSlice("slice-1", "node-2", "nvidia.com/gpu", 4),
			},
			expected: map[string]int64{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CountNodeGPUsFromResourceSlicesByDeviceClass(tt.nodeName, tt.slices)
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

// createAllNodesResourceSlice creates a ResourceSlice that applies to all nodes.
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
