// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package deviceclass_info

import (
	"testing"

	"github.com/stretchr/testify/assert"
	resourceapi "k8s.io/api/resource/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/common_info"
)

func TestNewDeviceClassInfo(t *testing.T) {
	deviceClass := &resourceapi.DeviceClass{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-device-class",
			UID:  types.UID("device-class-uid-456"),
		},
	}

	info := NewDeviceClassInfo(deviceClass)

	assert.NotNil(t, info)
	assert.Equal(t, common_info.DeviceClassID("device-class-uid-456"), info.UID)
	assert.Equal(t, "test-device-class", info.Name)
}

