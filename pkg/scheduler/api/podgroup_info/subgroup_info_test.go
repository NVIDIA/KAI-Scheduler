// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package podgroup_info

import (
	"testing"

	"github.com/NVIDIA/KAI-scheduler/pkg/apis/scheduling/v2alpha2"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/pod_info"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/pod_status"
)

func TestNewSubGroupInfo(t *testing.T) {
	name := "my-subgroup"
	minAvailable := int32(4)
	sgi := newSubGroupInfo(name, minAvailable)

	if sgi.Name != name {
		t.Errorf("Expected Name %s, got %s", name, sgi.Name)
	}
	if sgi.MinAvailable != minAvailable {
		t.Errorf("Expected MinAvailable %d, got %d", minAvailable, sgi.MinAvailable)
	}
	if len(sgi.PodInfos) != 0 {
		t.Errorf("Expected empty PodInfos, got %d items", len(sgi.PodInfos))
	}
}

func TestFromSubGroup(t *testing.T) {
	subGroup := &v2alpha2.SubGroup{
		Name:      "test-subgroup",
		MinMember: 3,
	}

	sgi := fromSubGroup(subGroup)
	if sgi.Name != subGroup.Name {
		t.Errorf("Expected name %s, got %s", subGroup.Name, sgi.Name)
	}
	if sgi.MinAvailable != subGroup.MinMember {
		t.Errorf("Expected MinAvailable %d, got %d", subGroup.MinMember, sgi.MinAvailable)
	}
	if len(sgi.PodInfos) != 0 {
		t.Errorf("Expected empty PodInfos, got %d items", len(sgi.PodInfos))
	}
}

func TestAddTaskInfoToSubGroup(t *testing.T) {
	sgi := newSubGroupInfo("test", 1)
	podInfo := &pod_info.PodInfo{
		UID:    "pod-1",
		Status: pod_status.Pending,
	}

	sgi.assignTask(podInfo)
	if len(sgi.PodInfos) != 1 {
		t.Errorf("Expected 1 pod info object, got %d", len(sgi.PodInfos))
	}
	if sgi.PodInfos[podInfo.UID] != podInfo {
		t.Error("Pod info not properly stored in map")
	}

	podInfo2 := &pod_info.PodInfo{
		UID:    "pod-2",
		Status: pod_status.Pending,
	}
	sgi.assignTask(podInfo2)
	if len(sgi.PodInfos) != 2 {
		t.Errorf("Expected 2 pod info objects, got %d", len(sgi.PodInfos))
	}

	if sgi.PodInfos[podInfo2.UID] != podInfo2 {
		t.Error("Pod info 2 not properly stored in map")
	}
}
