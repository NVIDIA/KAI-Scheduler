// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package accumulated_scenario_filters

import (
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/common_info"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/pod_info"
)

// greedyMatchRequirements checks whether each resource requirement (of numerical type) can be satisfied by one of the
// capacity holders using greedy virtual allocation. Both requirements and holders must be sorted
// descending. Returns true if all non-zero requirements can be matched.
func greedyMatchRequirements[K comparable](
	requirements []float64,
	holders []K,
	capacity func(K) float64,
) bool {
	virtuallyAllocated := make(map[K]float64, len(holders))
	for _, required := range requirements {
		if required == 0 {
			continue
		}
		matched := false
		for _, holder := range holders {
			totalCapacity := capacity(holder)
			// Early termination: holders are sorted descending by capacity.
			// If the best total capacity is below required, no holder can satisfy it.
			if totalCapacity < required {
				break
			}
			available := totalCapacity - virtuallyAllocated[holder]
			if available >= required {
				virtuallyAllocated[holder] += required
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}
	return true
}

// iterateNewVictims calls fn for each victim task not yet in processedCache.
// Tasks with no assigned node are skipped. Each new task is added to processedCache before
// fn is called, preventing double-counting when the same victim appears across multiple calls.
// Returns the number of cache hits (tasks that were already processed).
func iterateNewVictims(
	victimTasks []*pod_info.PodInfo,
	processedCache map[common_info.PodID]bool,
	fn func(*pod_info.PodInfo),
) int {
	numCacheHits := 0
	for _, task := range victimTasks {
		if task.NodeName == "" {
			continue
		}
		if processedCache[task.UID] {
			numCacheHits++
			continue
		}
		processedCache[task.UID] = true
		fn(task)
	}
	return numCacheHits
}
