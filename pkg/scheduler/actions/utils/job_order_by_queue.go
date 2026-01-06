// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package utils

import (
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/common_info"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/podgroup_info"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/queue_info"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/framework"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/log"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/scheduler_util"
)

// queueMetadata holds the priority queue and state for a queue at any level in the hierarchy.
// For leaf queues, items contains jobs. For parent queues, items contains child queues.
type queueMetadata struct {
	items         *scheduler_util.PriorityQueue
	needsReorder  bool
	parentQueueID common_info.QueueID // only set for leaf queues
}

// JobsOrderByQueues manages job ordering across a two-level queue hierarchy.
// Parent queues contain leaf queues, and leaf queues contain jobs.
type JobsOrderByQueues struct {
	ssn     *framework.Session
	options JobsOrderInitOptions

	activeParentQueues *scheduler_util.PriorityQueue
	leafQueues         map[common_info.QueueID]*queueMetadata // leaf queue ID -> jobs
	parentQueues       map[common_info.QueueID]*queueMetadata // parent queue ID -> child queues

	poppedJobsByQueue map[common_info.QueueID][]*podgroup_info.PodGroupInfo
}

func NewJobsOrderByQueues(ssn *framework.Session, options JobsOrderInitOptions) JobsOrderByQueues {
	return JobsOrderByQueues{
		ssn:               ssn,
		options:           options,
		leafQueues:        map[common_info.QueueID]*queueMetadata{},
		parentQueues:      map[common_info.QueueID]*queueMetadata{},
		poppedJobsByQueue: map[common_info.QueueID][]*podgroup_info.PodGroupInfo{},
	}
}

func (jo *JobsOrderByQueues) IsEmpty() bool {
	return jo.activeParentQueues == nil || jo.activeParentQueues.Empty()
}

func (jo *JobsOrderByQueues) Len() int {
	count := 0
	for _, meta := range jo.leafQueues {
		count += meta.items.Len()
	}
	return count
}

func (jo *JobsOrderByQueues) PopNextJob() *podgroup_info.PodGroupInfo {
	if jo.IsEmpty() {
		log.InfraLogger.V(7).Infof("No active parent queues")
		return nil
	}

	parentQueue := jo.getNextFromQueue(
		jo.activeParentQueues,
		jo.parentQueues,
		"parent queue",
	)
	if parentQueue == nil {
		return nil
	}

	leafQueue := jo.getNextFromQueue(
		jo.parentQueues[parentQueue.UID].items,
		jo.leafQueues,
		"leaf queue",
	)
	if leafQueue == nil {
		return nil
	}

	job := jo.leafQueues[leafQueue.UID].items.Pop().(*podgroup_info.PodGroupInfo)

	if jo.options.VictimQueue {
		jo.poppedJobsByQueue[leafQueue.UID] = append(jo.poppedJobsByQueue[leafQueue.UID], job)
	}

	jo.handlePopFromQueue(leafQueue.UID, jo.leafQueues, jo.parentQueues[parentQueue.UID].items)
	jo.handlePopFromQueue(parentQueue.UID, jo.parentQueues, jo.activeParentQueues)

	log.InfraLogger.V(7).Infof("Popped job: %v", job.Name)
	return job
}

func (jo *JobsOrderByQueues) PushJob(job *podgroup_info.PodGroupInfo) {
	leafQueue := jo.ssn.Queues[job.Queue]
	parentQueue := jo.ssn.Queues[leafQueue.ParentQueue]

	if _, found := jo.parentQueues[parentQueue.UID]; !found {
		jo.initParentQueue(parentQueue)
		jo.activeParentQueues.Push(parentQueue)
	}

	if _, found := jo.leafQueues[job.Queue]; !found {
		jo.initLeafQueue(job)
		jo.parentQueues[parentQueue.UID].items.Push(leafQueue)
	}

	jo.leafQueues[job.Queue].items.Push(job)
	jo.leafQueues[leafQueue.UID].needsReorder = true
	jo.parentQueues[parentQueue.UID].needsReorder = true

	log.InfraLogger.V(7).Infof("Pushed job: %v for queue %v, parent queue %v",
		job.Name, leafQueue.Name, parentQueue.Name)
}

// getNextFromQueue retrieves the next queue from a priority queue, handling reordering as needed.
// It peeks at the top item, checks if its metadata needs reordering, fixes if needed, and returns.
func (jo *JobsOrderByQueues) getNextFromQueue(
	priorityQueue *scheduler_util.PriorityQueue,
	metadataMap map[common_info.QueueID]*queueMetadata,
	queueType string,
) *queue_info.QueueInfo {
	queue := priorityQueue.Peek().(*queue_info.QueueInfo)
	meta := metadataMap[queue.UID]

	if meta.needsReorder {
		priorityQueue.Fix(0)
		meta.needsReorder = false
		return jo.getNextFromQueue(priorityQueue, metadataMap, queueType)
	}

	if meta.items.Empty() {
		log.InfraLogger.V(7).Warnf("%s: <%v> is active, yet has no children", queueType, queue.Name)
		return nil
	}

	log.InfraLogger.V(7).Infof("Get %s: %v", queueType, queue.Name)
	return queue
}

// handlePopFromQueue handles cleanup after popping an item from a queue.
// If the queue is now empty, it removes it from its parent priority queue and deletes its metadata.
// Otherwise, it marks the queue for reordering.
func (jo *JobsOrderByQueues) handlePopFromQueue(
	queueID common_info.QueueID,
	metadataMap map[common_info.QueueID]*queueMetadata,
	parentPriorityQueue *scheduler_util.PriorityQueue,
) {
	meta := metadataMap[queueID]
	if meta.items.Len() == 0 {
		parentPriorityQueue.Pop()
		delete(metadataMap, queueID)
		return
	}
	meta.needsReorder = true
}

// addJobToQueue adds a job to its leaf queue, creating the queue metadata if needed
func (jo *JobsOrderByQueues) addJobToQueue(job *podgroup_info.PodGroupInfo, reverseOrder bool) {
	if _, found := jo.leafQueues[job.Queue]; !found {
		jo.initLeafQueueWithOrder(job, reverseOrder)
	}
	jo.leafQueues[job.Queue].items.Push(job)
}

func (jo *JobsOrderByQueues) initParentQueue(parentQueue *queue_info.QueueInfo) {
	jo.parentQueues[parentQueue.UID] = &queueMetadata{
		items: scheduler_util.NewPriorityQueue(
			jo.buildQueueOrderFn(jo.leafQueues, jo.options.VictimQueue),
			scheduler_util.QueueCapacityInfinite,
		),
	}
}

func (jo *JobsOrderByQueues) initLeafQueue(job *podgroup_info.PodGroupInfo) {
	jo.initLeafQueueWithOrder(job, jo.options.VictimQueue)
}

func (jo *JobsOrderByQueues) initLeafQueueWithOrder(job *podgroup_info.PodGroupInfo, reverseOrder bool) {
	leafQueue := jo.ssn.Queues[job.Queue]
	jo.leafQueues[job.Queue] = &queueMetadata{
		items: scheduler_util.NewPriorityQueue(func(l, r interface{}) bool {
			if reverseOrder {
				return !jo.ssn.JobOrderFn(l, r)
			}
			return jo.ssn.JobOrderFn(l, r)
		}, jo.options.MaxJobsQueueDepth),
		parentQueueID: leafQueue.ParentQueue,
	}
}

func (jo *JobsOrderByQueues) buildActiveQueues(reverseOrder bool) {
	jo.parentQueues = map[common_info.QueueID]*queueMetadata{}

	for _, queue := range jo.ssn.Queues {
		leafMeta, found := jo.leafQueues[queue.UID]
		if !found || leafMeta.items.Len() == 0 {
			log.InfraLogger.V(7).Infof("Skipping queue <%s> because no jobs in it", queue.Name)
			continue
		}

		if _, parentExists := jo.ssn.Queues[queue.ParentQueue]; !parentExists {
			log.InfraLogger.V(7).Warnf("Queue's parent doesn't exist. Queue: <%v>, Parent: <%v>",
				queue.Name, queue.ParentQueue)
			continue
		}

		parentMeta, parentMetaExists := jo.parentQueues[queue.ParentQueue]
		if !parentMetaExists {
			log.InfraLogger.V(7).Infof("Adding parent queue <%s>", queue.ParentQueue)
			jo.parentQueues[queue.ParentQueue] = &queueMetadata{
				items: scheduler_util.NewPriorityQueue(
					jo.buildQueueOrderFn(jo.leafQueues, reverseOrder),
					scheduler_util.QueueCapacityInfinite,
				),
			}
			parentMeta = jo.parentQueues[queue.ParentQueue]
		}

		parentMeta.items.Push(queue)
		log.InfraLogger.V(7).Infof("Added leaf queue to parent: parent=<%v>, leaf=<%v>, jobs=<%v>, reverseOrder=<%v>",
			queue.ParentQueue, queue.Name, leafMeta.items.Len(), reverseOrder)
	}

	log.InfraLogger.V(7).Infof("Building parent queues priority queue, reverseOrder=<%v>", reverseOrder)
	jo.activeParentQueues = scheduler_util.NewPriorityQueue(
		jo.buildParentQueueOrderFn(reverseOrder),
		scheduler_util.QueueCapacityInfinite)

	for parentQueueID := range jo.parentQueues {
		log.InfraLogger.V(7).Infof("Active parent queue <%s>", parentQueueID)
		jo.activeParentQueues.Push(jo.ssn.Queues[parentQueueID])
	}
}

// extractJobsForComparison extracts the pending job or victims from a leaf queue for comparison.
// Returns (pending, victims) where exactly one is populated based on VictimQueue option.
func (jo *JobsOrderByQueues) extractJobsForComparison(
	queueID common_info.QueueID,
	meta *queueMetadata,
) (*podgroup_info.PodGroupInfo, []*podgroup_info.PodGroupInfo) {
	if jo.options.VictimQueue {
		return nil, jo.getVictimsForQueue(queueID, meta)
	}

	pending := meta.items.Pop().(*podgroup_info.PodGroupInfo)
	meta.items.Push(pending)
	return pending, nil
}

// buildQueueOrderFn creates a comparison function for ordering queues based on their best job.
// Used for ordering leaf queues within a parent queue.
func (jo *JobsOrderByQueues) buildQueueOrderFn(
	leafQueuesMap map[common_info.QueueID]*queueMetadata,
	reverseOrder bool,
) func(interface{}, interface{}) bool {
	return func(lQ, rQ interface{}) bool {
		lQueue := lQ.(*queue_info.QueueInfo)
		rQueue := rQ.(*queue_info.QueueInfo)

		lMeta, lFound := leafQueuesMap[lQueue.UID]
		rMeta, rFound := leafQueuesMap[rQueue.UID]

		if !lFound || lMeta.items.Len() == 0 {
			log.InfraLogger.V(7).Infof("Queue: %v has no pending jobs", lQueue.Name)
			return !reverseOrder
		}

		if !rFound || rMeta.items.Len() == 0 {
			log.InfraLogger.V(7).Infof("Queue: %v has no pending jobs", rQueue.Name)
			return reverseOrder
		}

		lPending, lVictims := jo.extractJobsForComparison(lQueue.UID, lMeta)
		rPending, rVictims := jo.extractJobsForComparison(rQueue.UID, rMeta)

		result := jo.ssn.QueueOrderFn(lQueue, rQueue, lPending, rPending, lVictims, rVictims)
		if reverseOrder {
			return !result
		}
		return result
	}
}

// buildParentQueueOrderFn creates a comparison function for ordering parent queues.
// It compares parent queues based on their best leaf queue's best job.
func (jo *JobsOrderByQueues) buildParentQueueOrderFn(reverseOrder bool) func(interface{}, interface{}) bool {
	return func(l, r interface{}) bool {
		lParent := l.(*queue_info.QueueInfo)
		rParent := r.(*queue_info.QueueInfo)

		lMeta := jo.parentQueues[lParent.UID]
		rMeta := jo.parentQueues[rParent.UID]

		if lMeta.items.Empty() {
			return !reverseOrder
		}
		if rMeta.items.Empty() {
			return reverseOrder
		}

		// Get best leaf queue from each parent
		lBestLeaf := lMeta.items.Pop().(*queue_info.QueueInfo)
		rBestLeaf := rMeta.items.Pop().(*queue_info.QueueInfo)
		defer func() {
			lMeta.items.Push(lBestLeaf)
			rMeta.items.Push(rBestLeaf)
		}()

		// Extract jobs from the best leaf queues for comparison
		lPending, lVictims := jo.extractJobsForComparison(lBestLeaf.UID, jo.leafQueues[lBestLeaf.UID])
		rPending, rVictims := jo.extractJobsForComparison(rBestLeaf.UID, jo.leafQueues[rBestLeaf.UID])

		result := jo.ssn.QueueOrderFn(lParent, rParent, lPending, rPending, lVictims, rVictims)
		if reverseOrder {
			return !result
		}
		return result
	}
}

// getVictimsForQueue returns all popped jobs plus the next job in queue for victim ordering
func (jo *JobsOrderByQueues) getVictimsForQueue(queueID common_info.QueueID, meta *queueMetadata) []*podgroup_info.PodGroupInfo {
	var victims []*podgroup_info.PodGroupInfo
	if poppedJobs := jo.poppedJobsByQueue[queueID]; len(poppedJobs) > 0 {
		victims = append(victims, poppedJobs...)
	}
	nextJob := meta.items.Pop().(*podgroup_info.PodGroupInfo)
	meta.items.Push(nextJob)
	return append(victims, nextJob)
}
