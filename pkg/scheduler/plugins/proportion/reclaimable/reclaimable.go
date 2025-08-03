// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package reclaimable

import (
	"math"

	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/common_info"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/resource_info"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/log"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/plugins/proportion/reclaimable/strategies"
	rs "github.com/NVIDIA/KAI-scheduler/pkg/scheduler/plugins/proportion/resource_share"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/plugins/proportion/utils"
)

type Reclaimable struct {
	isInferencePreemptible bool
}

func New(isInferencePreemptible bool) *Reclaimable {
	return &Reclaimable{isInferencePreemptible}
}

func (r *Reclaimable) CanReclaimResources(
	queues map[common_info.QueueID]*rs.QueueAttributes,
	reclaimer *ReclaimerInfo,
) bool {
	reclaimerQueue := queues[reclaimer.Queue]
	requestedResources := utils.QuantifyResource(reclaimer.RequiredResources)

	allocatedResources := reclaimerQueue.GetAllocatedShare()
	allocatedResources.Add(requestedResources)
	if !allocatedResources.LessEqual(reclaimerQueue.GetFairShare()) {
		return false
	}

	if reclaimer.IsPreemptable {
		return true
	}

	allocatedNonPreemptible := reclaimerQueue.GetAllocatedNonPreemptible()
	allocatedNonPreemptible.Add(requestedResources)
	if !allocatedNonPreemptible.LessEqual(reclaimerQueue.GetDeservedShare()) {
		return false
	}

	return true
}

func (r *Reclaimable) Reclaimable(
	queues map[common_info.QueueID]*rs.QueueAttributes,
	reclaimer *ReclaimerInfo,
	reclaimeeResourcesByQueue map[common_info.QueueID][]*resource_info.Resource,
) bool {
	reclaimable, reclaimedQueuesRemainingResources :=
		r.reclaimResourcesFromReclaimees(queues, reclaimer, reclaimeeResourcesByQueue)
	if !reclaimable {
		return false
	}
	return r.reclaimingQueuesRemainWithinBoundaries(queues, reclaimer, reclaimedQueuesRemainingResources)
}

func (r *Reclaimable) reclaimResourcesFromReclaimees(
	queues map[common_info.QueueID]*rs.QueueAttributes,
	reclaimer *ReclaimerInfo,
	reclaimeesResourcesByQueue map[common_info.QueueID][]*resource_info.Resource,
) (
	bool, map[common_info.QueueID]rs.ResourceQuantities,
) {
	remainingResourcesMap := map[common_info.QueueID]rs.ResourceQuantities{}
	for reclaimeeQueueID, reclaimeeQueueReclaimedResources := range reclaimeesResourcesByQueue {
		reclaimerQueue, reclaimeeQueue := r.getLeveledQueues(queues, reclaimer.Queue, reclaimeeQueueID)

		if _, found := remainingResourcesMap[reclaimeeQueue.UID]; !found {
			remainingResourcesMap[reclaimeeQueue.UID] = queues[reclaimeeQueue.UID].GetAllocatedShare()
		}
		remainingResources := remainingResourcesMap[reclaimeeQueue.UID]

		for _, reclaimeeResources := range reclaimeeQueueReclaimedResources {
			if !strategies.FitsReclaimStrategy(reclaimer.RequiredResources, reclaimerQueue, reclaimeeQueue,
				remainingResources) {
				log.InfraLogger.V(7).Infof("queue <%s>，shouldn't be reclaimed, for %v resources"+
					" remaining reosurces: <%v>, deserved: <%v>, fairShare: <%v>",
					reclaimeeQueue.Name, resource_info.StringResourceArray(reclaimeeQueueReclaimedResources),
					remainingResources.String(), reclaimeeQueue.GetDeservedShare().String(),
					reclaimeeQueue.GetFairShare().String())
				return false, nil
			}

			r.subtractReclaimedResources(queues, remainingResourcesMap, reclaimeeQueueID, reclaimeeResources)
		}
	}

	return true, remainingResourcesMap
}

func (r *Reclaimable) subtractReclaimedResources(
	queues map[common_info.QueueID]*rs.QueueAttributes,
	remainingResourcesMap map[common_info.QueueID]rs.ResourceQuantities,
	reclaimeeQueueID common_info.QueueID,
	reclaimedResources *resource_info.Resource,
) {
	for queue, ok := queues[reclaimeeQueueID]; ok; queue, ok = queues[queue.ParentQueue] {
		if _, found := remainingResourcesMap[queue.UID]; !found {
			remainingResourcesMap[queue.UID] = queues[queue.UID].GetAllocatedShare()
		}

		remainingResources := remainingResourcesMap[queue.UID]
		activeAllocatedQuota := utils.QuantifyResource(reclaimedResources)
		remainingResources.Sub(activeAllocatedQuota)
	}
}

func (r *Reclaimable) reclaimingQueuesRemainWithinBoundaries(
	queues map[common_info.QueueID]*rs.QueueAttributes,
	reclaimer *ReclaimerInfo,
	remainingResourcesMap map[common_info.QueueID]rs.ResourceQuantities,
) bool {

	requestedQuota := utils.QuantifyResource(reclaimer.RequiredResources)

	topQueuesCount := len(r.getTopQueues(queues))
	for reclaimingQueue, found := queues[reclaimer.Queue]; found; reclaimingQueue, found = queues[reclaimingQueue.ParentQueue] {
		remainingResources, foundRemaining := remainingResourcesMap[reclaimingQueue.UID]
		if !foundRemaining {
			remainingResources = reclaimingQueue.GetAllocatedShare()
		}
		remainingResources.Add(requestedQuota)

		if !reclaimingQueue.IsTopQueue() || topQueuesCount > 1 {
			if !remainingResources.LessEqual(reclaimingQueue.GetFairShare()) {
				log.InfraLogger.V(5).Infof("Failed to reclaim resources for job: <%s/%s> in queue <%s>. "+
					"Queue will be over fair share if job will be running. "+
					"Queue fair share: %v, queue allocated resources with task: <%v>",
					reclaimer.Namespace, reclaimer.Name, reclaimingQueue.Name, reclaimingQueue.GetFairShare().String(),
					remainingResources.String())
				return false
			}
		}

		if reclaimer.IsPreemptable {
			continue
		}

		allocatedNonPreemptible := reclaimingQueue.GetAllocatedNonPreemptible()
		allocatedNonPreemptible.Add(requestedQuota)
		if !allocatedNonPreemptible.LessEqual(reclaimingQueue.GetDeservedShare()) {
			log.InfraLogger.V(5).Infof("Failed to reclaim resources for: <%s/%s> in queue <%s>. "+
				"Queue will have nonpreemtible jobs over quota and reclaimer job is an interactive job. "+
				"Queue quota: %v, queue allocated nonpreemtible resources with task: %v",
				reclaimer.Namespace, reclaimer.Name, reclaimingQueue.Name, reclaimingQueue.GetDeservedShare().String(),
				allocatedNonPreemptible.String())
			return false
		}
	}

	return true
}

func (r *Reclaimable) getTopQueues(queues map[common_info.QueueID]*rs.QueueAttributes) map[common_info.QueueID]bool {
	topQueues := make(map[common_info.QueueID]bool)
	for queueID, queue := range queues {
		if queue.IsTopQueue() {
			topQueues[queueID] = true
		}
	}
	return topQueues
}

func (r *Reclaimable) getLeveledQueues(
	queues map[common_info.QueueID]*rs.QueueAttributes,
	reclaimerQueueID common_info.QueueID,
	reclaimeeQueueID common_info.QueueID,
) (*rs.QueueAttributes, *rs.QueueAttributes) {
	reclaimers := r.getHierarchyPath(queues, reclaimerQueueID)
	reclaimees := r.getHierarchyPath(queues, reclaimeeQueueID)

	minLength := int(math.Min(float64(len(reclaimers)), float64(len(reclaimees))))
	var reclaimerQueue, reclaimeeQueue *rs.QueueAttributes
	for i := 0; i < minLength; i++ {
		reclaimerQueue = reclaimers[i]
		reclaimeeQueue = reclaimees[i]
		if reclaimerQueue.UID != reclaimeeQueue.UID {
			break
		}
	}
	return reclaimerQueue, reclaimeeQueue
}

func (r *Reclaimable) getHierarchyPath(
	queues map[common_info.QueueID]*rs.QueueAttributes, queueId common_info.QueueID) []*rs.QueueAttributes {
	var hierarchyPath []*rs.QueueAttributes
	queue, found := queues[queueId]
	for found {
		hierarchyPath = append([]*rs.QueueAttributes{queue}, hierarchyPath...)
		queue, found = queues[queue.ParentQueue]
	}
	return hierarchyPath
}
