// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package podgroup_info

import (
	"testing"

	v1 "k8s.io/api/core/v1"

	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/common_info"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/pod_info"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/pod_status"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/resource_info"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/scheduler_util"
)

func simpleTask(name string, subGroupName string, status pod_status.PodStatus) *pod_info.PodInfo {
	pod := common_info.BuildPod("node", name, "", v1.PodPending,
		common_info.BuildResourceList("1", "1G"),
		nil, nil, nil,
	)
	info := pod_info.NewTaskInfo(pod)
	info.Status = status
	info.SubGroupName = subGroupName
	return info
}

func alwaysLess(_, _ interface{}) bool { return false }

func Test_HasTasksToAllocate(t *testing.T) {
	pg := NewPodGroupInfo("pg1")
	if HasTasksToAllocate(pg, true) {
		t.Error("expected false with zero tasks")
	}
	// Add one pending that ShouldAllocate
	task := simpleTask("p1", "", pod_status.Pending)
	pg.AddTaskInfo(task)
	if !HasTasksToAllocate(pg, true) {
		t.Error("expected true with allocatable task")
	}
	// Now set the status so ShouldAllocate returns false
	task.Status = pod_status.Succeeded
	if HasTasksToAllocate(pg, true) {
		t.Error("expected false with non-allocatable status")
	}
}

func Test_GetTasksToAllocate(t *testing.T) {
	pg := NewPodGroupInfo("pg")
	pg.MinAvailable = 1
	task := simpleTask("pA", "", pod_status.Pending)
	pg.AddTaskInfo(task)
	result := GetTasksToAllocate(pg, alwaysLess, alwaysLess, true)
	if len(result) != 1 {
		t.Fatalf("expected 1 allocatable, got %d", len(result))
	}
}

func Test_GetTaskToAllocateWithSubGroups(t *testing.T) {
	pg := NewPodGroupInfo("pg")
	pg.MinAvailable = 2
	pg.SubGroups["sub"] = NewSubGroupInfo("sub", 2)

	pg.AddTaskInfo(simpleTask("pA", "sub", pod_status.Pending))
	pg.AddTaskInfo(simpleTask("pB", "sub", pod_status.Pending))

	got := GetTasksToAllocate(pg, alwaysLess, alwaysLess, true)
	if len(got) != 2 {
		t.Errorf("expected 2 tasks to allocate from main+subgroup, got %d", len(got))
	}
}

func Test_GetTasksToAllocateRequestedGPUs(t *testing.T) {
	pg := NewPodGroupInfo("test-podgroup")
	pg.MinAvailable = 1
	task := simpleTask("p1", "", pod_status.Pending)
	// manually set up a fake ResReq that returns 2 for GPUs and 1000 for GpuMemory
	task.ResReq = resource_info.NewResourceRequirements(2, 1000, 2000)
	pg.AddTaskInfo(task)
	gpus, _ := GetTasksToAllocateRequestedGPUs(pg, alwaysLess, alwaysLess, true)
	if gpus != 2 {
		t.Errorf("expected gpus=2, got %v", gpus)
	}
}

func Test_GetTasksToAllocateInitResource(t *testing.T) {
	pg := NewPodGroupInfo("ri")
	// Nil case
	res := GetTasksToAllocateInitResource(nil, alwaysLess, alwaysLess, true)
	if !res.IsEmpty() {
		t.Error("empty resource expected for nil pg")
	}

	pg.MinAvailable = 1
	task := simpleTask("p", "", pod_status.Pending)
	task.ResReq = resource_info.NewResourceRequirements(0, 5000, 0)
	pg.AddTaskInfo(task)
	resource := GetTasksToAllocateInitResource(pg, alwaysLess, alwaysLess, true)
	cpu := resource.BaseResource.Get(v1.ResourceCPU)
	if cpu != 5000 {
		t.Fatalf("want cpu=5, got %v", cpu)
	}
	// Memoization/second call should return r
	newResource := GetTasksToAllocateInitResource(pg, alwaysLess, alwaysLess, true)
	if newResource != resource {
		t.Error("cached resource pointer mismatch")
	}
}

func Test_getTasksFromQueue(t *testing.T) {
	q := scheduler_util.NewPriorityQueue(alwaysLess, 10)
	p1 := simpleTask("q1", "", pod_status.Pending)
	p2 := simpleTask("q2", "", pod_status.Pending)
	q.Push(p1)
	q.Push(p2)
	ts := getTasksFromQueue(q, 1)
	if len(ts) != 1 {
		t.Error("expected 1 task from queue")
	}
}

func Test_getTasksPriorityQueue(t *testing.T) {
	pg := NewPodGroupInfo("pq")
	pg.AddTaskInfo(simpleTask("t1", "", pod_status.Pending))
	pg.AddTaskInfo(simpleTask("t2", "", pod_status.Succeeded))
	q := getTasksPriorityQueue(pg, alwaysLess, true)
	if q.Len() != 1 {
		t.Error("should filter to only allocatable tasks")
	}
}

func Test_getTasksPriorityQueuePerSubGroup(t *testing.T) {
	pg := NewPodGroupInfo("test-pg")
	sg := NewSubGroupInfo("test-sub-group", 1)
	pg.SubGroups["test-sub-group"] = sg

	pg.AddTaskInfo(simpleTask("a", "test-sub-group", pod_status.Pending))
	m := getTasksPriorityQueuePerSubGroup(pg, alwaysLess, true)
	if len(m) != 1 {
		t.Error("expected 1 subgroup queue")
	}
	if m["test-sub-group"].Len() != 1 {
		t.Error("subgroup should contain one allocatable task")
	}
}

func Test_getNumOfTasksToAllocate(t *testing.T) {
	pg := NewPodGroupInfo("n")
	pg.MinAvailable = 2
	// None allocated, 2 pending
	pg.AddTaskInfo(simpleTask("p1", "", pod_status.Pending))
	pg.AddTaskInfo(simpleTask("p2", "", pod_status.Allocated))
	want := 1
	got := getNumOfTasksToAllocate(pg)
	if got != want {
		t.Errorf("got %d want %d", got, want)
	}
}

func Test_getNumOfTasksToAllocatePerSubGroup(t *testing.T) {
	pg := NewPodGroupInfo("pg")
	sg := NewSubGroupInfo("sg", 1)
	pg.SubGroups["sg"] = sg

	pg.AddTaskInfo(simpleTask("p1", "sg", pod_status.Pending))
	pg.AddTaskInfo(simpleTask("p2", "sg", pod_status.Allocated))
	m := getNumOfTasksToAllocatePerSubGroup(pg)
	if m["sg"] != 1 {
		t.Errorf("want 1, got %v", m["sg"])
	}
}

func Test_getMaxNumOfTasksToAllocate(t *testing.T) {
	type args struct {
		minAvailable     int32
		pods             []*v1.Pod
		overridingStatus []pod_status.PodStatus
	}
	tests := []struct {
		name string
		args args
		want int
	}{
		{
			name: "single pod pending",
			args: args{
				minAvailable: 1,
				pods: []*v1.Pod{
					common_info.BuildPod("n1", "p1", "", v1.PodPending,
						common_info.BuildResourceList("1000m", "1G"),
						nil, nil, nil),
				},
			},
			want: 1,
		},
		{
			name: "three pods pending",
			args: args{
				minAvailable: 3,
				pods: []*v1.Pod{
					common_info.BuildPod("n1", "p1", "", v1.PodPending,
						common_info.BuildResourceList("1000m", "1G"),
						nil, nil, nil),
					common_info.BuildPod("n1", "p2", "", v1.PodPending,
						common_info.BuildResourceList("1000m", "1G"),
						nil, nil, nil),
					common_info.BuildPod("n1", "p3", "", v1.PodPending,
						common_info.BuildResourceList("1000m", "1G"),
						nil, nil, nil),
				},
			},
			want: 3,
		},
		{
			name: "four pods, min available equal running, two pending",
			args: args{
				minAvailable: 2,
				pods: []*v1.Pod{
					common_info.BuildPod("n1", "p1", "", v1.PodRunning,
						common_info.BuildResourceList("1000m", "1G"),
						nil, nil, nil),
					common_info.BuildPod("n1", "p2", "", v1.PodRunning,
						common_info.BuildResourceList("1000m", "1G"),
						nil, nil, nil),
					common_info.BuildPod("n1", "p3", "", v1.PodPending,
						common_info.BuildResourceList("1000m", "1G"),
						nil, nil, nil),
					common_info.BuildPod("n1", "p4", "", v1.PodPending,
						common_info.BuildResourceList("1000m", "1G"),
						nil, nil, nil),
				},
			},
			want: 1,
		},
		{
			name: "pipline over dying pods",
			args: args{
				minAvailable: 2,
				pods: []*v1.Pod{
					common_info.BuildPod("n1", "p1", "", v1.PodRunning,
						common_info.BuildResourceList("1000m", "1G"),
						nil, nil, nil),
					common_info.BuildPod("n1", "p2", "", v1.PodRunning,
						common_info.BuildResourceList("1000m", "1G"),
						nil, nil, nil),
					common_info.BuildPod("n1", "p3", "", v1.PodPending,
						common_info.BuildResourceList("1000m", "1G"),
						nil, nil, nil),
					common_info.BuildPod("n1", "p4", "", v1.PodPending,
						common_info.BuildResourceList("1000m", "1G"),
						nil, nil, nil),
				},
				overridingStatus: []pod_status.PodStatus{pod_status.Releasing, pod_status.Releasing,
					pod_status.Pending, pod_status.Pending},
			},
			want: 2,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pg := NewPodGroupInfo("u1")
			pg.MinAvailable = tt.args.minAvailable
			for i, pod := range tt.args.pods {
				pi := pod_info.NewTaskInfo(pod)
				if tt.args.overridingStatus != nil {
					pi.Status = tt.args.overridingStatus[i]
				}
				pg.AddTaskInfo(pi)
			}

			if got := getNumOfTasksToAllocate(pg); got != tt.want {
				t.Errorf("getNumOfTasksToAllocate() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_getNumOfAllocatedTasks(t *testing.T) {
	type args struct {
		pods             []*v1.Pod
		overridingStatus []pod_status.PodStatus
	}
	tests := []struct {
		name string
		args args
		want int
	}{
		{
			name: "single pod pending",
			args: args{
				pods: []*v1.Pod{
					common_info.BuildPod("n1", "p1", "", v1.PodPending,
						common_info.BuildResourceList("1000m", "1G"),
						nil, nil, nil),
				},
			},
			want: 0,
		},
		{
			name: "single pod running",
			args: args{
				pods: []*v1.Pod{
					common_info.BuildPod("n1", "p1", "", v1.PodRunning,
						common_info.BuildResourceList("1000m", "1G"),
						nil, nil, nil),
				},
			},
			want: 1,
		},
		{
			name: "single pod releasing",
			args: args{
				pods: []*v1.Pod{
					common_info.BuildPod("n1", "p1", "", v1.PodFailed,
						common_info.BuildResourceList("1000m", "1G"),
						nil, nil, nil),
				},
				overridingStatus: []pod_status.PodStatus{pod_status.Releasing},
			},
			want: 0,
		},
		{
			name: "two pods running",
			args: args{
				pods: []*v1.Pod{
					common_info.BuildPod("n1", "p1", "", v1.PodRunning,
						common_info.BuildResourceList("1000m", "1G"),
						nil, nil, nil),
					common_info.BuildPod("n1", "p2", "", v1.PodRunning,
						common_info.BuildResourceList("1000m", "1G"),
						nil, nil, nil),
				},
			},
			want: 2,
		},
		{
			name: "one pending one running",
			args: args{
				pods: []*v1.Pod{
					common_info.BuildPod("n1", "p1", "", v1.PodPending,
						common_info.BuildResourceList("1000m", "1G"),
						nil, nil, nil),
					common_info.BuildPod("n1", "p2", "", v1.PodRunning,
						common_info.BuildResourceList("1000m", "1G"),
						nil, nil, nil),
				},
			},
			want: 1,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pg := NewPodGroupInfo("u1")
			for i, pod := range tt.args.pods {
				pi := pod_info.NewTaskInfo(pod)
				pg.AddTaskInfo(pi)

				if tt.args.overridingStatus != nil {
					pi.Status = tt.args.overridingStatus[i]
				}
			}

			if got := pg.GetActiveAllocatedTasksCount(); got != tt.want {
				t.Errorf("getNumOfAllocatedTasks() = %v, want %v", got, tt.want)
			}
		})
	}
}
