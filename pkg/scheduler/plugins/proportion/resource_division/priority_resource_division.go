// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package resource_division

import (
	"math"

	commonconstants "github.com/NVIDIA/KAI-scheduler/pkg/common/constants"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/common_info"
	rs "github.com/NVIDIA/KAI-scheduler/pkg/scheduler/plugins/proportion/resource_share"
)

// SetResourcesSharePriority distributes resources between queues according to the
// three-phase algorithm described in docs/developer/designs/priority-based-fair-share.md.
//
//  1. Queues are grouped by priority (higher priority buckets processed first).
//  2. Within a bucket we run:
//     a) Phase-1: allocate Deserved quota (subject to remaining capacity).
//     b) Phase-2: allocate over-quota resources proportionally to OverQuotaWeight.
//     c) Phase-3: allocate any still-remaining capacity equally between the queues
//     that still have unmet demand (treating all weights as 1).
func SetResourcesSharePriority(totalResource rs.ResourceQuantities, queues map[common_info.QueueID]*rs.QueueAttributes) {
	for _, resource := range rs.AllResources {
		remaining := totalResource[resource]
		if remaining <= 0 {
			continue
		}

		// Group queues by priority (higher first)
		queuesByPriority, priorities := getQueuesByPriority(queues)

		// Process each priority level
		for _, priority := range priorities {
			remaining = processPriorityBucket(remaining, queuesByPriority[priority], resource, totalResource[resource])
			if remaining <= 0 {
				break
			}
		}
	}

	reportDivisionResult(queues)
}

// processPriorityBucket processes a single priority bucket through all division phases
func processPriorityBucket(remaining float64, bucket map[common_info.QueueID]*rs.QueueAttributes, resource rs.ResourceName, totalResource float64) float64 {
	if remaining <= 0 {
		return remaining
	}

	// Phase 1: Distribute Deserved quota
	remaining = divideDeservedQuota(remaining, bucket, resource, totalResource)
	if remaining <= 0 {
		return remaining
	}

	// Phase 2: Distribute over quota between queues
	remaining = divideOverQuotaResource(remaining, bucket, resource)
	if remaining <= 0 {
		return remaining
	}

	// Phase 3: Weight-0 compensation - Distribute remaining quota to queues with 0 OverQuotaWeight
	remaining = divideWithCompensation(remaining, bucket, resource)

	return remaining
}

func divideDeservedQuota(remaining float64, queues map[common_info.QueueID]*rs.QueueAttributes, resource rs.ResourceName, totalResource float64) float64 {
	for _, q := range queues {
		if remaining <= 0 {
			break
		}
		rsShare := q.ResourceShare(resource)
		deserved := rsShare.Deserved
		if deserved == commonconstants.UnlimitedResourceQuantity {
			deserved = totalResource
		}
		toGive := math.Min(deserved, rsShare.GetRequestableShare())
		toGive = math.Min(toGive, remaining)
		if toGive > 0 {
			q.AddResourceShare(resource, toGive)
			remaining -= toGive
		}
	}
	return remaining
}

// divideWithCompensation divides remaining resources among queues with zero weight by temporarily setting their weights to 1
func divideWithCompensation(remaining float64, queues map[common_info.QueueID]*rs.QueueAttributes, resource rs.ResourceName) float64 {
	originalOverQuotaWeight := make(map[common_info.QueueID]float64)
	for _, q := range queues {
		originalOverQuotaWeight[q.UID] = q.ResourceShare(resource).OverQuotaWeight
		q.ResourceShare(resource).OverQuotaWeight = 1
	}

	remaining = divideOverQuotaResource(remaining, queues, resource)

	for _, q := range queues {
		q.ResourceShare(resource).OverQuotaWeight = originalOverQuotaWeight[q.UID]
	}

	return remaining
}
