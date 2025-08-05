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
	sgi := NewSubGroupInfo(name, minAvailable)

	if sgi.name != name {
		t.Errorf("Expected name %s, got %s", name, sgi.name)
	}
	if sgi.minAvailable != minAvailable {
		t.Errorf("Expected minAvailable %d, got %d", minAvailable, sgi.minAvailable)
	}
	if len(sgi.podInfos) != 0 {
		t.Errorf("Expected empty podInfos, got %d items", len(sgi.podInfos))
	}
}

func TestFromSubGroup(t *testing.T) {
	subGroup := &v2alpha2.SubGroup{
		Name:      "test-subgroup",
		MinMember: 3,
	}

	sgi := FromSubGroup(subGroup)
	if sgi.name != subGroup.Name {
		t.Errorf("Expected name %s, got %s", subGroup.Name, sgi.name)
	}
	if sgi.minAvailable != subGroup.MinMember {
		t.Errorf("Expected minAvailable %d, got %d", subGroup.MinMember, sgi.minAvailable)
	}
	if len(sgi.podInfos) != 0 {
		t.Errorf("Expected empty podInfos, got %d items", len(sgi.podInfos))
	}
}

func TestGetName(t *testing.T) {
	name := "test-subgroup"
	sgi := NewSubGroupInfo(name, 1)

	if got := sgi.GetName(); got != name {
		t.Errorf("GetName() = %q, want %q", got, name)
	}
}

func TestGetMinAvailable(t *testing.T) {
	minAvailable := int32(3)
	sgi := NewSubGroupInfo("test", minAvailable)

	if got := sgi.GetMinAvailable(); got != minAvailable {
		t.Errorf("GetMinAvailable() = %d, want %d", got, minAvailable)
	}
}

func TestSetMinAvailable(t *testing.T) {
	sgi := NewSubGroupInfo("test", 8)
	newMinAvailable := int32(5)
	sgi.SetMinAvailable(newMinAvailable)

	if got := sgi.GetMinAvailable(); got != newMinAvailable {
		t.Errorf("After SetMinAvailable(%d), GetMinAvailable() = %d", newMinAvailable, got)
	}
}

func TestGetPodInfos(t *testing.T) {
	sgi := NewSubGroupInfo("test", 2)

	// Should be empty initially
	if podInfos := sgi.GetPodInfos(); len(podInfos) != 0 {
		t.Errorf("Expected empty podInfos from GetPodInfos, got %d items", len(podInfos))
	}

	// Add a pod and check GetPodInfos
	pod := &pod_info.PodInfo{UID: "test-pod", Status: pod_status.Pending}
	sgi.AssignTask(pod)
	podInfos := sgi.GetPodInfos()
	if len(podInfos) != 1 {
		t.Errorf("Expected 1 pod in GetPodInfos, got %d", len(podInfos))
	}
	if podInfos["test-pod"] != pod {
		t.Error("GetPodInfos did not return expected pod mapping")
	}
}

func TestAddTaskInfoToSubGroup(t *testing.T) {
	sgi := NewSubGroupInfo("test", 1)
	podInfo := &pod_info.PodInfo{
		UID:    "pod-1",
		Status: pod_status.Pending,
	}

	sgi.AssignTask(podInfo)
	if len(sgi.podInfos) != 1 {
		t.Errorf("Expected 1 pod info object, got %d", len(sgi.podInfos))
	}
	if sgi.podInfos[podInfo.UID] != podInfo {
		t.Error("Pod info not properly stored in map")
	}

	podInfo2 := &pod_info.PodInfo{
		UID:    "pod-2",
		Status: pod_status.Pending,
	}
	sgi.AssignTask(podInfo2)
	if len(sgi.podInfos) != 2 {
		t.Errorf("Expected 2 pod info objects, got %d", len(sgi.podInfos))
	}

	if sgi.podInfos[podInfo2.UID] != podInfo2 {
		t.Error("Pod info 2 not properly stored in map")
	}
}

func TestIsReadyForScheduling(t *testing.T) {
	tests := []struct {
		name         string
		minAvailable int32
		pods         []*pod_info.PodInfo
		expected     bool
	}{
		{
			name:         "ready with exact minimum",
			minAvailable: 2,
			pods: []*pod_info.PodInfo{
				{UID: "1", Status: pod_status.Pending},
				{UID: "2", Status: pod_status.Pending},
			},
			expected: true,
		},
		{
			name:         "ready with more than minimum",
			minAvailable: 1,
			pods: []*pod_info.PodInfo{
				{UID: "1", Status: pod_status.Pending},
				{UID: "2", Status: pod_status.Pending},
			},
			expected: true,
		},
		{
			name:         "not ready with gated pods",
			minAvailable: 2,
			pods: []*pod_info.PodInfo{
				{UID: "1", Status: pod_status.Pending},
				{UID: "2", Status: pod_status.Gated},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sgi := NewSubGroupInfo("test", tt.minAvailable)
			for _, pod := range tt.pods {
				sgi.AssignTask(pod)
			}
			if got := sgi.IsReadyForScheduling(); got != tt.expected {
				t.Errorf("IsReadyForScheduling() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestIsGangSatisfied(t *testing.T) {
	tests := []struct {
		name         string
		minAvailable int32
		pods         []*pod_info.PodInfo
		expected     bool
	}{
		{
			name:         "satisfied with exact minimum",
			minAvailable: 2,
			pods: []*pod_info.PodInfo{
				{UID: "1", Status: pod_status.Running},
				{UID: "2", Status: pod_status.Running},
			},
			expected: true,
		},
		{
			name:         "not satisfied with insufficient active pods",
			minAvailable: 2,
			pods: []*pod_info.PodInfo{
				{UID: "1", Status: pod_status.Running},
				{UID: "2", Status: pod_status.Failed},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sgi := NewSubGroupInfo("test", tt.minAvailable)
			for _, pod := range tt.pods {
				sgi.AssignTask(pod)
			}
			if got := sgi.IsGangSatisfied(); got != tt.expected {
				t.Errorf("IsGangSatisfied() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestGetNumAliveTasks(t *testing.T) {
	sgi := NewSubGroupInfo("test", 2)
	pods := []*pod_info.PodInfo{
		{UID: "1", Status: pod_status.Running},
		{UID: "2", Status: pod_status.Pending},
		{UID: "3", Status: pod_status.Failed},
		{UID: "4", Status: pod_status.Succeeded},
	}

	for _, pod := range pods {
		sgi.AssignTask(pod)
	}

	expected := 2 // Running and Pending are alive statuses
	if got := sgi.GetNumAliveTasks(); got != expected {
		t.Errorf("GetNumAliveTasks() = %v, want %v", got, expected)
	}
}

func TestGetNumActiveUsedTasks(t *testing.T) {
	sgi := NewSubGroupInfo("test", 2)
	pods := []*pod_info.PodInfo{
		{UID: "1", Status: pod_status.Running},
		{UID: "2", Status: pod_status.Pipelined},
		{UID: "3", Status: pod_status.Failed},
		{UID: "4", Status: pod_status.Succeeded},
	}

	for _, pod := range pods {
		sgi.AssignTask(pod)
	}

	expected := 2 // Running and Pipelined are active allocated statuses
	if got := sgi.GetNumActiveAllocatedTasks(); got != expected {
		t.Errorf("GetNumAliveTasks() = %v, want %v", got, expected)
	}
}

func TestGetNumGatedTasks(t *testing.T) {
	sgi := NewSubGroupInfo("test", 2)
	pods := []*pod_info.PodInfo{
		{UID: "1", Status: pod_status.Gated},
		{UID: "2", Status: pod_status.Running},
		{UID: "3", Status: pod_status.Gated},
		{UID: "4", Status: pod_status.Pending},
	}

	for _, pod := range pods {
		sgi.AssignTask(pod)
	}

	expected := 2 // Two Gated pods
	if got := sgi.GetNumGatedTasks(); got != expected {
		t.Errorf("GetNumGatedTasks() = %v, want %v", got, expected)
	}
}

func TestGetNumPendingTasks(t *testing.T) {
	sgi := NewSubGroupInfo("test", 2)
	pods := []*pod_info.PodInfo{
		{UID: "1", Status: pod_status.Pending},
		{UID: "2", Status: pod_status.Running},
		{UID: "3", Status: pod_status.Pending},
		{UID: "4", Status: pod_status.Gated},
		{UID: "5", Status: pod_status.Pending},
	}

	for _, pod := range pods {
		sgi.AssignTask(pod)
	}

	expected := 3 // Three Pending pods
	if got := sgi.GetNumPendingTasks(); got != expected {
		t.Errorf("GetNumPendingTasks() = %v, want %v", got, expected)
	}
}

func TestGetNumActiveAllocatedTasks(t *testing.T) {
	tests := []struct {
		name     string
		pods     []*pod_info.PodInfo
		expected int
	}{
		{
			name:     "no pods",
			pods:     nil,
			expected: 0,
		},
		{
			name: "all pods not active/allocated",
			pods: []*pod_info.PodInfo{
				{UID: "1", Status: pod_status.Pending},
				{UID: "2", Status: pod_status.Gated},
			},
			expected: 0,
		},
		{
			name: "one active allocated",
			pods: []*pod_info.PodInfo{
				{UID: "1", Status: pod_status.Allocated},
				{UID: "2", Status: pod_status.Pending},
			},
			expected: 1,
		},
		{
			name: "multiple active allocated",
			pods: []*pod_info.PodInfo{
				{UID: "1", Status: pod_status.Allocated},
				{UID: "2", Status: pod_status.Running},
				{UID: "3", Status: pod_status.Gated},
				{UID: "4", Status: pod_status.Allocated},
			},
			expected: 3, // Allocated and Running are assumed active allocated
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sgi := NewSubGroupInfo("test", 1)
			for _, pod := range tt.pods {
				sgi.AssignTask(pod)
			}
			got := sgi.GetNumActiveAllocatedTasks()
			if got != tt.expected {
				t.Errorf("GetNumActiveAllocatedTasks() = %d, want %d", got, tt.expected)
			}
		})
	}
}
