// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package subgroup_info

import (
	"testing"

	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/topology_info"
)

func TestNewSubGroupInfo(t *testing.T) {
	name := "my-subgroup"
	sgi := newSubGroupInfo(name)

	if sgi.name != name {
		t.Errorf("Expected name %s, got %s", name, sgi.name)
	}
}

func TestGetName(t *testing.T) {
	name := "test-subgroup"
	sgi := newSubGroupInfo(name)

	if got := sgi.GetName(); got != name {
		t.Errorf("GetName() = %q, want %q", got, name)
	}
}

func TestAddTopologyConstraint(t *testing.T) {
	sgi := newSubGroupInfo("test-subgroup")
	tc := &topology_info.TopologyConstraintInfo{}
	sgi.AddTopologyConstraint(tc)
	if sgi.GetTopologyConstraint() != tc {
		t.Error("AddTopologyConstraint() did not add the topology constraint")
	}
}
