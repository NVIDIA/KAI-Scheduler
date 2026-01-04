// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package resourceslice_info

import (
	"testing"

	"github.com/stretchr/testify/assert"
	resourceapi "k8s.io/api/resource/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/common_info"
)

func TestNewResourceSliceInfo(t *testing.T) {
	slice := &resourceapi.ResourceSlice{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-slice",
			UID:  types.UID("slice-uid-123"),
		},
		Spec: resourceapi.ResourceSliceSpec{
			Driver: "nvidia.com/gpu",
		},
	}

	info := NewResourceSliceInfo(slice)

	assert.NotNil(t, info)
	assert.Equal(t, common_info.ResourceSliceID("slice-uid-123"), info.UID)
	assert.Equal(t, "test-slice", info.Name)
}

