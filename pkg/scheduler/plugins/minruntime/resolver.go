package minruntime

import (
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/common_info"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/queue_info"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// getPreemptMinRuntime resolves min-runtime for preemptions
// Starting from the leaf-queue, walk the tree until the first defined preempt-min-runtime is set and use that
func (mr *minruntimePlugin) getPreemptMinRuntime(
	queue *queue_info.QueueInfo,
) metav1.Duration {
	if queue == nil {
		return *mr.defaultPreemptMinRuntime
	}

	if minRuntime, ok := mr.preemptMinRuntimeCache[queue.UID]; ok {
		return minRuntime
	}

	// Start with the provided leaf queue
	currentQueue := queue

	// Walk up the tree until we find a set preempt-min-runtime or reach the root
	for currentQueue != nil {
		if currentQueue.PreemptMinRuntime != nil {
			mr.preemptMinRuntimeCache[queue.UID] = *currentQueue.PreemptMinRuntime
			return *currentQueue.PreemptMinRuntime
		}

		// Move to parent queue if it exists
		if currentQueue.ParentQueue != "" {
			parentQueue, found := mr.queues[currentQueue.ParentQueue]
			if found {
				currentQueue = parentQueue
				continue
			}
		}

		// Break if no more parent queues
		break
	}

	// If no preempt-min-runtime is set in the queue tree, use default
	mr.preemptMinRuntimeCache[queue.UID] = *mr.defaultPreemptMinRuntime
	return *mr.defaultPreemptMinRuntime
}

// getReclaimMinRuntime resolves min-runtime for reclaims
// Depending on the resolveMethod, it will use either:
// 1. queue: Starting from the leaf-queue, walk the tree (similar to preempt)
// 2. lca: Use the lowest common ancestor approach
func (mr *minruntimePlugin) getReclaimMinRuntime(
	resolveMethod string,
	preemptorQueue *queue_info.QueueInfo,
	preempteeQueue *queue_info.QueueInfo,
) metav1.Duration {
	if preemptorQueue == nil || preempteeQueue == nil {
		return *mr.defaultReclaimMinRuntime
	}

	if minRuntime, ok := mr.reclaimMinRuntimeCache[preemptorQueue.UID][preempteeQueue.UID]; ok {
		return minRuntime
	}

	// If method is LCA, use the LCA-based algorithm
	if resolveMethod == resolveMethodLCA {
		return mr.getReclaimMinRuntimeLCA(preemptorQueue, preempteeQueue)
	}

	// Otherwise, use queue-based approach (similar to preempt)
	// Start with the provided leaf queue
	currentQueue := preempteeQueue

	// Walk up the tree until we find a set reclaim-min-runtime or reach the root
	for currentQueue != nil {
		if currentQueue.ReclaimMinRuntime != nil {
			return *currentQueue.ReclaimMinRuntime
		}

		// Move to parent queue if it exists
		if currentQueue.ParentQueue != "" {
			parentQueue, found := mr.queues[currentQueue.ParentQueue]
			if found {
				currentQueue = parentQueue
				continue
			}
		}

		// Break if no more parent queues
		break
	}

	// If no reclaim-min-runtime is set in the queue tree, use default
	return *mr.defaultReclaimMinRuntime
}

// getReclaimMinRuntimeLCA resolves min-runtime for reclaims using the LCA method
// 1. Resolve the lowest common ancestor (LCA) between the leaf-queues of preemptor and preemptee
// 2. Walk 1 step down to the child of the LCA that is an ancestor to the preemptee's leaf queue (or is the leaf queue)
// 3. Use the reclaim-min-runtime from this queue, if it is set. Otherwise move back up towards root of tree
//
// Note: Top-level queues (with empty parent) are treated as siblings with an implicit shadow parent
func (mr *minruntimePlugin) getReclaimMinRuntimeLCA(
	preemptorQueue *queue_info.QueueInfo,
	preempteeQueue *queue_info.QueueInfo,
) metav1.Duration {
	if preemptorQueue == nil || preempteeQueue == nil {
		return *mr.defaultReclaimMinRuntime
	}

	// Get hierarchy paths for both queues
	preemptorPath := mr.getQueueHierarchyPath(preemptorQueue)
	preempteePath := mr.getQueueHierarchyPath(preempteeQueue)

	// Handle case where both queues are from different top-level hierarchies
	// Consider them as siblings with a shadow parent
	if len(preemptorPath) > 0 && len(preempteePath) > 0 {
		topLevelPreemptor := preemptorPath[0]
		topLevelPreemptee := preempteePath[0]

		// If both are top-level queues (empty parent) and different
		if topLevelPreemptor.ParentQueue == "" && topLevelPreemptee.ParentQueue == "" &&
			topLevelPreemptor.UID != topLevelPreemptee.UID {

			// For different top-level queues:
			// The LCA is the implicit shadow parent at level -1 (doesn't exist in path)
			// So we'd want to look at the top-level queue (first in path) that's the ancestor
			// of the preemptee, which is preempteePath[0]

			// Check if the top-level preemptee queue has reclaim-min-runtime set
			if topLevelPreemptee.ReclaimMinRuntime != nil {
				mr.cacheReclaimMinRuntime(preemptorQueue, preempteeQueue, *topLevelPreemptee.ReclaimMinRuntime)
				return *topLevelPreemptee.ReclaimMinRuntime
			}

			// No value in hierarchy, use default
			mr.cacheReclaimMinRuntime(preemptorQueue, preempteeQueue, *mr.defaultReclaimMinRuntime)
			return *mr.defaultReclaimMinRuntime
		}
	}

	// Find the lowest common ancestor (LCA)
	lcaIndex := 0
	minLength := min(len(preemptorPath), len(preempteePath))

	for i := 0; i < minLength; i++ {
		if preemptorPath[i].UID != preempteePath[i].UID {
			break
		}
		lcaIndex = i
	}

	// If we found a common ancestor
	if lcaIndex < len(preemptorPath) && lcaIndex < len(preempteePath) &&
		preemptorPath[lcaIndex].UID == preempteePath[lcaIndex].UID {

		lca := preemptorPath[lcaIndex]

		// If there's a path down from LCA to preemptee
		if lcaIndex+1 < len(preempteePath) {
			// Walk 1 step down to the child of the LCA that is an ancestor to the preemptee's leaf queue
			childQueue := preempteePath[lcaIndex+1]

			// Check if this queue has reclaim-min-runtime set
			if childQueue.ReclaimMinRuntime != nil {
				mr.cacheReclaimMinRuntime(preemptorQueue, preempteeQueue, *childQueue.ReclaimMinRuntime)
				return *childQueue.ReclaimMinRuntime
			}

			// Otherwise move back up towards root and select first available override
			for i := lcaIndex; i >= 0; i-- {
				if preempteePath[i].ReclaimMinRuntime != nil {
					mr.cacheReclaimMinRuntime(preemptorQueue, preempteeQueue, *preempteePath[i].ReclaimMinRuntime)
					return *preempteePath[i].ReclaimMinRuntime
				}
			}
		} else {
			// If LCA is the preemptee queue itself, check its reclaim-min-runtime
			if lca.ReclaimMinRuntime != nil {
				mr.cacheReclaimMinRuntime(preemptorQueue, preempteeQueue, *lca.ReclaimMinRuntime)
				return *lca.ReclaimMinRuntime
			}

			// Otherwise move up towards root
			for i := lcaIndex - 1; i >= 0; i-- {
				if preempteePath[i].ReclaimMinRuntime != nil {
					mr.cacheReclaimMinRuntime(preemptorQueue, preempteeQueue, *preempteePath[i].ReclaimMinRuntime)
					return *preempteePath[i].ReclaimMinRuntime
				}
			}
		}
	}

	// If no reclaim-min-runtime is found in the hierarchy, use default
	mr.cacheReclaimMinRuntime(preemptorQueue, preempteeQueue, *mr.defaultReclaimMinRuntime)
	return *mr.defaultReclaimMinRuntime
}

// getQueueHierarchyPath returns the queue hierarchy from top-level queue to the specified queue
// Note: For top-level queues (empty parent), the path will only contain the queue itself
func (mr *minruntimePlugin) getQueueHierarchyPath(
	queue *queue_info.QueueInfo,
) []*queue_info.QueueInfo {
	var hierarchyPath []*queue_info.QueueInfo

	// Start with the queue itself
	currentQueue := queue

	// Build the path from queue to parent
	for currentQueue != nil {
		// Add current queue to the path (at beginning to maintain parent->child order)
		hierarchyPath = append([]*queue_info.QueueInfo{currentQueue}, hierarchyPath...)

		// If queue has a parent, add it to the path
		if currentQueue.ParentQueue != "" {
			parentQueue, found := mr.queues[currentQueue.ParentQueue]
			if found {
				currentQueue = parentQueue
				continue
			}
		}

		// Break if no more parents (reached top-level)
		break
	}

	return hierarchyPath
}

func (mr *minruntimePlugin) cacheReclaimMinRuntime(preemptorQueue *queue_info.QueueInfo, preempteeQueue *queue_info.QueueInfo, minRuntime metav1.Duration) {
	if mr.reclaimMinRuntimeCache == nil {
		return
	}

	if mr.reclaimMinRuntimeCache[preemptorQueue.UID] == nil {
		mr.reclaimMinRuntimeCache[preemptorQueue.UID] = make(map[common_info.QueueID]metav1.Duration)
	}
	mr.reclaimMinRuntimeCache[preemptorQueue.UID][preempteeQueue.UID] = minRuntime
}
