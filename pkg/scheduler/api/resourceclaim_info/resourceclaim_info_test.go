// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package resourceclaim_info

import (
	"testing"

	"github.com/stretchr/testify/assert"
	resourceapi "k8s.io/api/resource/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/common_info"
)

func TestNewResourceClaimInfo(t *testing.T) {
	claim := &resourceapi.ResourceClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-claim",
			Namespace: "test-ns",
			UID:       types.UID("claim-uid"),
		},
	}

	info := NewResourceClaimInfo(claim)

	assert.NotNil(t, info)
	assert.Equal(t, common_info.ResourceClaimID("test-ns/test-claim"), info.UID)
	assert.Equal(t, "test-claim", info.Name)
	assert.Equal(t, "test-ns", info.Namespace)
}

func TestNewResourceClaimInfo_EmptyNamespace(t *testing.T) {
	claim := &resourceapi.ResourceClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-claim",
			UID:  types.UID("claim-uid"),
		},
	}

	info := NewResourceClaimInfo(claim)

	assert.NotNil(t, info)
	assert.Equal(t, common_info.ResourceClaimID("test-claim"), info.UID)
	assert.Equal(t, "test-claim", info.Name)
	assert.Equal(t, "", info.Namespace)
}

