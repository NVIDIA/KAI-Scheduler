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

	parentQueue := jo.getNextParentQueue()
	if parentQueue == nil {
		return nil
	}

	leafQueue := jo.getNextLeafQueue(parentQueue)
	if leafQueue == nil {
		return nil
	}

	job := jo.leafQueues[leafQueue.UID].items.Pop().(*podgroup_info.PodGroupInfo)

	if jo.options.VictimQueue {
		jo.poppedJobsByQueue[leafQueue.UID] = append(jo.poppedJobsByQueue[leafQueue.UID], job)
	}

	jo.handlePopFromLeafQueue(leafQueue, parentQueue)
	jo.handlePopFromParentQueue(parentQueue)

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

func (jo *JobsOrderByQueues) handlePopFromParentQueue(parentQueue *queue_info.QueueInfo) {
	meta := jo.parentQueues[parentQueue.UID]
	if meta.items.Len() == 0 {
		jo.activeParentQueues.Pop()
		delete(jo.parentQueues, parentQueue.UID)
		return
	}
	meta.needsReorder = true
}

func (jo *JobsOrderByQueues) handlePopFromLeafQueue(leafQueue, parentQueue *queue_info.QueueInfo) {
	meta := jo.leafQueues[leafQueue.UID]
	if meta.items.Len() == 0 {
		jo.parentQueues[parentQueue.UID].items.Pop()
		delete(jo.leafQueues, leafQueue.UID)
		return
	}
	meta.needsReorder = true
}

func (jo *JobsOrderByQueues) getNextLeafQueue(parentQueue *queue_info.QueueInfo) *queue_info.QueueInfo {
	parentMeta := jo.parentQueues[parentQueue.UID]
	leafQueue := parentMeta.items.Peek().(*queue_info.QueueInfo)

	leafMeta := jo.leafQueues[leafQueue.UID]
	if leafMeta.needsReorder {
		parentMeta.items.Fix(0)
		leafMeta.needsReorder = false
		return jo.getNextLeafQueue(parentQueue)
	}

	if leafMeta.items.Len() == 0 {
		log.InfraLogger.V(7).Warnf("Queue: <%v> is active, yet no jobs in queue", leafQueue.Name)
		return nil
	}

	log.InfraLogger.V(7).Infof("Get queue: %v", leafQueue.Name)
	return leafQueue
}

func (jo *JobsOrderByQueues) getNextParentQueue() *queue_info.QueueInfo {
	parentQueue := jo.activeParentQueues.Peek().(*queue_info.QueueInfo)
	meta := jo.parentQueues[parentQueue.UID]

	if meta.needsReorder {
		jo.activeParentQueues.Fix(0)
		meta.needsReorder = false
		return jo.getNextParentQueue()
	}

	if meta.items.Empty() {
		log.InfraLogger.V(7).Warnf("Parent queue: <%v> is active, yet no child queues", parentQueue.Name)
		return nil
	}

	log.InfraLogger.V(7).Infof("Get parent queue: %v", parentQueue.Name)
	return parentQueue
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
			jo.buildLeafQueueOrderFn(jo.options.VictimQueue),
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

		if _, found := jo.ssn.Queues[queue.ParentQueue]; !found {
			log.InfraLogger.V(7).Warnf("Queue's parent doesn't exist. Queue: <%v>, Parent: <%v>",
				queue.Name, queue.ParentQueue)
			continue
		}

		parentMeta, found := jo.parentQueues[queue.ParentQueue]
		if !found {
			log.InfraLogger.V(7).Infof("Adding parent queue <%s>", queue.ParentQueue)
			jo.parentQueues[queue.ParentQueue] = &queueMetadata{
				items: scheduler_util.NewPriorityQueue(
					jo.buildLeafQueueOrderFn(reverseOrder),
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

// buildLeafQueueOrderFn creates a comparison function for ordering leaf queues within a parent queue
func (jo *JobsOrderByQueues) buildLeafQueueOrderFn(reverseOrder bool) func(interface{}, interface{}) bool {
	return func(lQ, rQ interface{}) bool {
		lQueue := lQ.(*queue_info.QueueInfo)
		rQueue := rQ.(*queue_info.QueueInfo)

		lMeta, lFound := jo.leafQueues[lQueue.UID]
		rMeta, rFound := jo.leafQueues[rQueue.UID]

		if !lFound || lMeta.items.Len() == 0 {
			log.InfraLogger.V(7).Infof("Queue: %v has no pending jobs", lQueue.Name)
			return !reverseOrder
		}

		if !rFound || rMeta.items.Len() == 0 {
			log.InfraLogger.V(7).Infof("Queue: %v has no pending jobs", rQueue.Name)
			return reverseOrder
		}

		var lPending, rPending *podgroup_info.PodGroupInfo
		var lVictims, rVictims []*podgroup_info.PodGroupInfo

		if jo.options.VictimQueue {
			lVictims = jo.getVictimsForQueue(lQueue.UID, lMeta)
			rVictims = jo.getVictimsForQueue(rQueue.UID, rMeta)
		} else {
			lPending = lMeta.items.Pop().(*podgroup_info.PodGroupInfo)
			lMeta.items.Push(lPending)
			rPending = rMeta.items.Pop().(*podgroup_info.PodGroupInfo)
			rMeta.items.Push(rPending)
		}

		result := jo.ssn.QueueOrderFn(lQueue, rQueue, lPending, rPending, lVictims, rVictims)
		if reverseOrder {
			return !result
		}
		return result
	}
}

// buildParentQueueOrderFn creates a comparison function for ordering parent queues
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

		lBestLeaf := lMeta.items.Pop().(*queue_info.QueueInfo)
		rBestLeaf := rMeta.items.Pop().(*queue_info.QueueInfo)
		defer func() {
			lMeta.items.Push(lBestLeaf)
			rMeta.items.Push(rBestLeaf)
		}()

		lLeafMeta := jo.leafQueues[lBestLeaf.UID]
		rLeafMeta := jo.leafQueues[rBestLeaf.UID]

		var lPending, rPending *podgroup_info.PodGroupInfo
		var lVictims, rVictims []*podgroup_info.PodGroupInfo

		if jo.options.VictimQueue {
			lVictims = jo.getVictimsForQueue(lBestLeaf.UID, lLeafMeta)
			rVictims = jo.getVictimsForQueue(rBestLeaf.UID, rLeafMeta)
		} else {
			lPending = lLeafMeta.items.Pop().(*podgroup_info.PodGroupInfo)
			lLeafMeta.items.Push(lPending)
			rPending = rLeafMeta.items.Pop().(*podgroup_info.PodGroupInfo)
			rLeafMeta.items.Push(rPending)
		}

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
