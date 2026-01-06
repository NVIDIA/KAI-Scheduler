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

// queueNode represents a node in the queue hierarchy tree.
// Each node contains either child queue nodes (for non-leaf queues) or jobs (for leaf queues).
// This structure supports n-level queue hierarchies.
type queueNode struct {
	queue        *queue_info.QueueInfo
	children     *scheduler_util.PriorityQueue // Contains *queueNode (for non-leaf) or *PodGroupInfo (for leaf)
	needsReorder bool
	parent       *queueNode // nil for root-level nodes
	isLeaf       bool       // true if children contains jobs, false if it contains queue nodes
}

// JobsOrderByQueues manages job ordering across an n-level queue hierarchy.
// The hierarchy is represented as a tree of queueNode objects.
type JobsOrderByQueues struct {
	ssn     *framework.Session
	options JobsOrderInitOptions

	rootNodes  *scheduler_util.PriorityQueue      // Top-level queue nodes (nodes with no parent)
	queueNodes map[common_info.QueueID]*queueNode // All queue nodes by ID for quick lookup

	poppedJobsByQueue map[common_info.QueueID][]*podgroup_info.PodGroupInfo
}

func NewJobsOrderByQueues(ssn *framework.Session, options JobsOrderInitOptions) JobsOrderByQueues {
	return JobsOrderByQueues{
		ssn:               ssn,
		options:           options,
		queueNodes:        map[common_info.QueueID]*queueNode{},
		poppedJobsByQueue: map[common_info.QueueID][]*podgroup_info.PodGroupInfo{},
	}
}

func (jo *JobsOrderByQueues) IsEmpty() bool {
	return jo.rootNodes == nil || jo.rootNodes.Empty()
}

func (jo *JobsOrderByQueues) Len() int {
	count := 0
	for _, node := range jo.queueNodes {
		if node.isLeaf {
			count += node.children.Len()
		}
	}
	return count
}

func (jo *JobsOrderByQueues) PopNextJob() *podgroup_info.PodGroupInfo {
	if jo.IsEmpty() {
		log.InfraLogger.V(7).Infof("No active queues")
		return nil
	}

	// Traverse down the tree to find the best leaf node
	leafNode := jo.traverseToLeaf(jo.rootNodes)
	if leafNode == nil {
		return nil
	}

	// Pop the job from the leaf
	job := leafNode.children.Pop().(*podgroup_info.PodGroupInfo)

	if jo.options.VictimQueue {
		jo.poppedJobsByQueue[leafNode.queue.UID] = append(jo.poppedJobsByQueue[leafNode.queue.UID], job)
	}

	// Handle cleanup and bubble up needsReorder
	jo.handlePopFromNode(leafNode)

	log.InfraLogger.V(7).Infof("Popped job: %v", job.Name)
	return job
}

func (jo *JobsOrderByQueues) PushJob(job *podgroup_info.PodGroupInfo) {
	leafQueueInfo := jo.ssn.Queues[job.Queue]
	parentQueueInfo := jo.ssn.Queues[leafQueueInfo.ParentQueue]

	// Ensure parent node exists
	parentNode := jo.queueNodes[parentQueueInfo.UID]
	if parentNode == nil {
		parentNode = jo.createNonLeafNode(parentQueueInfo, nil)
		jo.queueNodes[parentQueueInfo.UID] = parentNode
		jo.rootNodes.Push(parentNode)
	}

	// Ensure leaf node exists
	leafNode := jo.queueNodes[job.Queue]
	if leafNode == nil {
		leafNode = jo.createLeafNode(leafQueueInfo, parentNode)
		jo.queueNodes[job.Queue] = leafNode
		parentNode.children.Push(leafNode)
	}

	// Push job and mark ancestors for reordering
	leafNode.children.Push(job)
	jo.markAncestorsForReorder(leafNode)

	log.InfraLogger.V(7).Infof("Pushed job: %v for queue %v, parent queue %v",
		job.Name, leafQueueInfo.Name, parentQueueInfo.Name)
}

// traverseToLeaf recursively traverses from a priority queue of nodes down to the best leaf node.
func (jo *JobsOrderByQueues) traverseToLeaf(pq *scheduler_util.PriorityQueue) *queueNode {
	node := jo.getNextNode(pq)
	if node == nil {
		return nil
	}

	if node.isLeaf {
		return node
	}

	// Recurse into children
	return jo.traverseToLeaf(node.children)
}

// getNextNode retrieves the next node from a priority queue, handling reordering as needed.
func (jo *JobsOrderByQueues) getNextNode(pq *scheduler_util.PriorityQueue) *queueNode {
	if pq.Empty() {
		return nil
	}

	node := pq.Peek().(*queueNode)

	if node.needsReorder {
		pq.Fix(0)
		node.needsReorder = false
		return jo.getNextNode(pq)
	}

	if node.children.Empty() {
		log.InfraLogger.V(7).Warnf("Queue node <%v> is active but has no children", node.queue.Name)
		return nil
	}

	log.InfraLogger.V(7).Infof("Selected queue: %v (isLeaf=%v)", node.queue.Name, node.isLeaf)
	return node
}

// handlePopFromNode handles cleanup after popping a job from a leaf node.
// If the node becomes empty, it's removed from its parent. Otherwise, ancestors are marked for reorder.
func (jo *JobsOrderByQueues) handlePopFromNode(node *queueNode) {
	if node.children.Len() == 0 {
		// Remove this node from its parent
		jo.removeNodeFromParent(node)
		delete(jo.queueNodes, node.queue.UID)

		// If parent is now empty, recursively clean up
		if node.parent != nil && node.parent.children.Len() == 0 {
			jo.handlePopFromNode(node.parent)
		} else if node.parent != nil {
			jo.markAncestorsForReorder(node.parent)
		}
		return
	}

	jo.markAncestorsForReorder(node)
}

// removeNodeFromParent removes a node from its parent's children priority queue.
func (jo *JobsOrderByQueues) removeNodeFromParent(node *queueNode) {
	if node.parent != nil {
		node.parent.children.Pop()
	} else {
		jo.rootNodes.Pop()
	}
}

// markAncestorsForReorder marks all ancestors of a node (including itself) as needing reorder.
func (jo *JobsOrderByQueues) markAncestorsForReorder(node *queueNode) {
	for current := node; current != nil; current = current.parent {
		current.needsReorder = true
	}
}

// createLeafNode creates a new leaf node that will contain jobs.
func (jo *JobsOrderByQueues) createLeafNode(queue *queue_info.QueueInfo, parent *queueNode) *queueNode {
	reverseOrder := jo.options.VictimQueue
	return &queueNode{
		queue: queue,
		children: scheduler_util.NewPriorityQueue(func(l, r interface{}) bool {
			if reverseOrder {
				return !jo.ssn.JobOrderFn(l, r)
			}
			return jo.ssn.JobOrderFn(l, r)
		}, jo.options.MaxJobsQueueDepth),
		parent: parent,
		isLeaf: true,
	}
}

// createNonLeafNode creates a new non-leaf node that will contain child queue nodes.
func (jo *JobsOrderByQueues) createNonLeafNode(queue *queue_info.QueueInfo, parent *queueNode) *queueNode {
	return &queueNode{
		queue: queue,
		children: scheduler_util.NewPriorityQueue(
			jo.buildNodeOrderFn(jo.options.VictimQueue),
			scheduler_util.QueueCapacityInfinite,
		),
		parent: parent,
		isLeaf: false,
	}
}

// addJobToQueue adds a job to its leaf queue, creating the queue node if needed.
func (jo *JobsOrderByQueues) addJobToQueue(job *podgroup_info.PodGroupInfo, reverseOrder bool) {
	if _, found := jo.queueNodes[job.Queue]; !found {
		leafQueue := jo.ssn.Queues[job.Queue]
		jo.queueNodes[job.Queue] = &queueNode{
			queue: leafQueue,
			children: scheduler_util.NewPriorityQueue(func(l, r interface{}) bool {
				if reverseOrder {
					return !jo.ssn.JobOrderFn(l, r)
				}
				return jo.ssn.JobOrderFn(l, r)
			}, jo.options.MaxJobsQueueDepth),
			isLeaf: true,
		}
	}
	jo.queueNodes[job.Queue].children.Push(job)
}

// buildActiveQueues builds the queue hierarchy from leaf nodes up to root nodes.
func (jo *JobsOrderByQueues) buildActiveQueues(reverseOrder bool) {
	// First pass: create parent nodes for all leaf nodes that have jobs
	parentNodes := map[common_info.QueueID]*queueNode{}

	for _, queue := range jo.ssn.Queues {
		leafNode, found := jo.queueNodes[queue.UID]
		if !found || !leafNode.isLeaf || leafNode.children.Len() == 0 {
			log.InfraLogger.V(7).Infof("Skipping queue <%s> because no jobs in it", queue.Name)
			continue
		}

		parentQueueInfo, parentExists := jo.ssn.Queues[queue.ParentQueue]
		if !parentExists {
			log.InfraLogger.V(7).Warnf("Queue's parent doesn't exist. Queue: <%v>, Parent: <%v>",
				queue.Name, queue.ParentQueue)
			continue
		}

		parentNode, parentNodeExists := parentNodes[queue.ParentQueue]
		if !parentNodeExists {
			log.InfraLogger.V(7).Infof("Adding parent queue <%s>", queue.ParentQueue)
			parentNode = &queueNode{
				queue: parentQueueInfo,
				children: scheduler_util.NewPriorityQueue(
					jo.buildNodeOrderFn(reverseOrder),
					scheduler_util.QueueCapacityInfinite,
				),
				isLeaf: false,
			}
			parentNodes[queue.ParentQueue] = parentNode
			jo.queueNodes[queue.ParentQueue] = parentNode
		}

		// Link leaf to parent
		leafNode.parent = parentNode
		parentNode.children.Push(leafNode)
		log.InfraLogger.V(7).Infof("Added leaf queue to parent: parent=<%v>, leaf=<%v>, jobs=<%v>, reverseOrder=<%v>",
			queue.ParentQueue, queue.Name, leafNode.children.Len(), reverseOrder)
	}

	// Build root nodes priority queue
	log.InfraLogger.V(7).Infof("Building root nodes priority queue, reverseOrder=<%v>", reverseOrder)
	jo.rootNodes = scheduler_util.NewPriorityQueue(
		jo.buildNodeOrderFn(reverseOrder),
		scheduler_util.QueueCapacityInfinite)

	for parentQueueID, parentNode := range parentNodes {
		log.InfraLogger.V(7).Infof("Active root queue <%s>", parentQueueID)
		jo.rootNodes.Push(parentNode)
	}
}

// buildNodeOrderFn creates a comparison function for ordering queue nodes.
// It compares nodes based on their best descendant job (recursively for non-leaf nodes).
func (jo *JobsOrderByQueues) buildNodeOrderFn(reverseOrder bool) func(interface{}, interface{}) bool {
	return func(l, r interface{}) bool {
		lNode := l.(*queueNode)
		rNode := r.(*queueNode)

		if lNode.children.Empty() {
			log.InfraLogger.V(7).Infof("Queue node %v has no children", lNode.queue.Name)
			return !reverseOrder
		}

		if rNode.children.Empty() {
			log.InfraLogger.V(7).Infof("Queue node %v has no children", rNode.queue.Name)
			return reverseOrder
		}

		// Get the best job from each subtree for comparison
		lPending, lVictims := jo.getBestJobFromNode(lNode)
		rPending, rVictims := jo.getBestJobFromNode(rNode)

		result := jo.ssn.QueueOrderFn(lNode.queue, rNode.queue, lPending, rPending, lVictims, rVictims)
		if reverseOrder {
			return !result
		}
		return result
	}
}

// getBestJobFromNode recursively finds the best job in a node's subtree.
// Returns (pending, victims) where exactly one is populated based on VictimQueue option.
func (jo *JobsOrderByQueues) getBestJobFromNode(node *queueNode) (*podgroup_info.PodGroupInfo, []*podgroup_info.PodGroupInfo) {
	if node.isLeaf {
		return jo.extractJobsForComparison(node.queue.UID, node)
	}

	// For non-leaf nodes, get the best child and recurse
	bestChild := node.children.Pop().(*queueNode)
	defer node.children.Push(bestChild)

	return jo.getBestJobFromNode(bestChild)
}

// extractJobsForComparison extracts the pending job or victims from a leaf node for comparison.
func (jo *JobsOrderByQueues) extractJobsForComparison(
	queueID common_info.QueueID,
	node *queueNode,
) (*podgroup_info.PodGroupInfo, []*podgroup_info.PodGroupInfo) {
	if jo.options.VictimQueue {
		return nil, jo.getVictimsForQueue(queueID, node)
	}

	pending := node.children.Pop().(*podgroup_info.PodGroupInfo)
	node.children.Push(pending)
	return pending, nil
}

// getVictimsForQueue returns all popped jobs plus the next job in queue for victim ordering.
func (jo *JobsOrderByQueues) getVictimsForQueue(queueID common_info.QueueID, node *queueNode) []*podgroup_info.PodGroupInfo {
	var victims []*podgroup_info.PodGroupInfo
	if poppedJobs := jo.poppedJobsByQueue[queueID]; len(poppedJobs) > 0 {
		victims = append(victims, poppedJobs...)
	}
	nextJob := node.children.Pop().(*podgroup_info.PodGroupInfo)
	node.children.Push(nextJob)
	return append(victims, nextJob)
}
