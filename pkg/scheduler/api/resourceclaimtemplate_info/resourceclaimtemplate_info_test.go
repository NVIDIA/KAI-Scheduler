// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package resourceclaimtemplate_info

import (
	"testing"

	"github.com/stretchr/testify/assert"
	resourceapi "k8s.io/api/resource/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/common_info"
)

func TestNewResourceClaimTemplateInfo(t *testing.T) {
	template := &resourceapi.ResourceClaimTemplate{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-template",
			Namespace: "test-ns",
			UID:       types.UID("template-uid"),
		},
		Spec: resourceapi.ResourceClaimTemplateSpec{
			Spec: resourceapi.ResourceClaimSpec{
				Devices: resourceapi.DeviceClaim{
					Requests: []resourceapi.DeviceRequest{},
				},
			},
		},
	}

	info := NewResourceClaimTemplateInfo(template)

	assert.NotNil(t, info)
	assert.Equal(t, common_info.ResourceClaimTemplateID("test-ns/test-template"), info.UID)
	assert.Equal(t, "test-template", info.Name)
	assert.Equal(t, "test-ns", info.Namespace)
}

func TestNewResourceClaimTemplateInfo_EmptyNamespace(t *testing.T) {
	template := &resourceapi.ResourceClaimTemplate{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-template",
			UID:  types.UID("template-uid"),
		},
		Spec: resourceapi.ResourceClaimTemplateSpec{
			Spec: resourceapi.ResourceClaimSpec{
				Devices: resourceapi.DeviceClaim{
					Requests: []resourceapi.DeviceRequest{},
				},
			},
		},
	}

	info := NewResourceClaimTemplateInfo(template)

	assert.NotNil(t, info)
	assert.Equal(t, common_info.ResourceClaimTemplateID("test-template"), info.UID)
	assert.Equal(t, "test-template", info.Name)
	assert.Equal(t, "", info.Namespace)
}

