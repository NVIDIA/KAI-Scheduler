// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package accumulated_scenario_filters

import (
	"reflect"
	"testing"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/actions/common/solvers/scenario"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/common_info"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/pod_info"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/podgroup_info"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/resource_info"
)

func TestAccumulatedIdleGpus_updateRequiredResources_countsMigPendingPods(t *testing.T) {
	ig := &AccumulatedIdleGpus{
		pendingTasksInState: map[common_info.PodID]bool{},
	}

	migPending := pod_info.NewTaskInfo(&v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			UID:       "uid-mig-pending",
			Name:      "pending-mig",
			Namespace: "n1",
		},
		Spec: v1.PodSpec{
			Containers: []v1.Container{
				{
					Resources: v1.ResourceRequirements{
						Requests: v1.ResourceList{
							v1.ResourceName("nvidia.com/mig-1g.5gb"): resource.MustParse("1"),
						},
					},
				},
			},
		},
	})

	scen := scenario.NewByNodeScenario(
		nil,
		nil,
		podgroup_info.NewPodGroupInfo("pg1", migPending),
		[]*pod_info.PodInfo{},
		[]*podgroup_info.PodGroupInfo{},
	)

	if err := ig.updateRequiredResources(scen, true); err != nil {
		t.Fatalf("updateRequiredResources returned error: %v", err)
	}

	want := []float64{1}
	if !reflect.DeepEqual(ig.requiredGpusSorted, want) {
		t.Fatalf("requiredGpusSorted=%v, want %v", ig.requiredGpusSorted, want)
	}
}

func TestAccumulatedIdleGpus_updateWithVictim_countsMigFreedGpus(t *testing.T) {
	ig := &AccumulatedIdleGpus{
		nodesNameToIdleGpus:     map[string]float64{"n1": 1.0, "n2": 2.0},
		maxFreeGpuNodesSorted:   []string{"n2"},
		recordedVictimsInCache:  map[common_info.PodID]bool{},
		potentialVictimsInCache: map[common_info.PodID]bool{},
	}

	victim := &pod_info.PodInfo{
		NodeName: "n1",
		UID:      "uid-mig-victim",
		AcceptedResource: &resource_info.ResourceRequirements{
			GpuResourceRequirement: *resource_info.NewGpuResourceRequirementWithMig(map[v1.ResourceName]int64{
				v1.ResourceName("nvidia.com/mig-2g.10gb"): 1,
			}),
		},
	}

	relevantCacheData := map[common_info.PodID]bool{}
	minIdleGpusRelevant := ig.updateWithVictim(victim, "n2", relevantCacheData)

	if minIdleGpusRelevant != "n1" {
		t.Fatalf("minIdleGpusRelevant=%q, want %q", minIdleGpusRelevant, "n1")
	}
	if got := ig.nodesNameToIdleGpus["n1"]; got != 3.0 {
		t.Fatalf("nodesNameToIdleGpus[n1]=%v, want %v", got, 3.0)
	}
	if !reflect.DeepEqual(ig.maxFreeGpuNodesSorted, []string{"n1"}) {
		t.Fatalf("maxFreeGpuNodesSorted=%v, want %v", ig.maxFreeGpuNodesSorted, []string{"n1"})
	}
	if !reflect.DeepEqual(relevantCacheData, map[common_info.PodID]bool{"uid-mig-victim": true}) {
		t.Fatalf("relevantCacheData=%v, want %v", relevantCacheData, map[common_info.PodID]bool{"uid-mig-victim": true})
	}
}
