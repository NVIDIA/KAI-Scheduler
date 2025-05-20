package minruntime

import (
	"time"

	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/common_info"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/queue_info"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/framework"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/log"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	defaultReclaimMinRuntimeConfig = "defaultReclaimMinRuntime"
	defaultPreemptMinRuntimeConfig = "defaultPreemptMinRuntime"
	actionTypeReclaim              = "reclaim"
	actionTypePreempt              = "preempt"
)

type minruntimePlugin struct {
	defaultReclaimMinRuntime *metav1.Duration
	defaultPreemptMinRuntime *metav1.Duration
}

func New(arguments map[string]string) framework.Plugin {
	plugin := &minruntimePlugin{}
	for key, value := range arguments {
		switch key {
		case defaultReclaimMinRuntimeConfig:
			duration, err := time.ParseDuration(value)
			if err != nil {
				log.InfraLogger.Errorf("Failed to parse %v: %v, using default value 0", key, err)
				duration = time.Duration(0 * time.Second)
			}
			plugin.defaultReclaimMinRuntime = &metav1.Duration{Duration: duration}
		case defaultPreemptMinRuntimeConfig:
			duration, err := time.ParseDuration(value)
			if err != nil {
				log.InfraLogger.Errorf("Failed to parse %v: %v, using default value 0", key, err)
				duration = time.Duration(0 * time.Second)
			}
			plugin.defaultPreemptMinRuntime = &metav1.Duration{Duration: duration}
		}
	}

	// Initialize with default values if not provided
	if plugin.defaultReclaimMinRuntime == nil {
		plugin.defaultReclaimMinRuntime = &metav1.Duration{Duration: time.Duration(0 * time.Second)}
	}
	if plugin.defaultPreemptMinRuntime == nil {
		plugin.defaultPreemptMinRuntime = &metav1.Duration{Duration: time.Duration(0 * time.Second)}
	}

	return plugin
}

func (mr *minruntimePlugin) Name() string {
	return "minruntime"
}

func (mr *minruntimePlugin) OnSessionOpen(ssn *framework.Session) {
	// TODO: add IsPreemptibleFn
}

func (mr *minruntimePlugin) OnSessionClose(ssn *framework.Session) {
}

// GetMinRuntime resolves the applicable min-runtime for reclaims and preemptions
// based on the provided action type and queues
func (mr *minruntimePlugin) GetMinRuntime(
	actionType string,
	preemptorQueue *queue_info.QueueInfo,
	preempteeQueue *queue_info.QueueInfo,
	queues map[common_info.QueueID]*queue_info.QueueInfo,
) *metav1.Duration {
	// Handle preemptions (preemptor and preemptee are within the same leaf-queue)
	if actionType == actionTypePreempt {
		return mr.getPreemptMinRuntime(preempteeQueue, queues)
	}

	// Handle reclaims (preemptor and preemptee are in different queues)
	if actionType == actionTypeReclaim {
		return mr.getReclaimMinRuntime(preemptorQueue, preempteeQueue, queues)
	}

	// If action type is unknown, return default reclaim min runtime
	return mr.defaultReclaimMinRuntime
}

// getPreemptMinRuntime resolves min-runtime for preemptions
// Starting from the leaf-queue, walk the tree until the first defined preempt-min-runtime is set and use that
func (mr *minruntimePlugin) getPreemptMinRuntime(
	queue *queue_info.QueueInfo,
	queues map[common_info.QueueID]*queue_info.QueueInfo,
) *metav1.Duration {
	// Start with the provided leaf queue
	currentQueue := queue

	// Walk up the tree until we find a set preempt-min-runtime or reach the root
	for currentQueue != nil {
		if currentQueue.PreemptMinRuntime != nil {
			return currentQueue.PreemptMinRuntime
		}

		// Move to parent queue if it exists
		if currentQueue.ParentQueue != "" {
			parentQueue, found := queues[currentQueue.ParentQueue]
			if found {
				currentQueue = parentQueue
				continue
			}
		}

		// Break if no more parent queues
		break
	}

	// If no preempt-min-runtime is set in the queue tree, use default
	return mr.defaultPreemptMinRuntime
}

// getReclaimMinRuntime resolves min-runtime for reclaims
// 1. Resolve the lowest common ancestor (LCA) between the leaf-queues of preemptor and preemptee
// 2. Walk 1 step down to the child of the LCA that is an ancestor to the preemptee's leaf queue (or is the leaf queue)
// 3. Use the reclaim-min-runtime from this queue, if it is set. Otherwise move back up towards root of tree
func (mr *minruntimePlugin) getReclaimMinRuntime(
	preemptorQueue *queue_info.QueueInfo,
	preempteeQueue *queue_info.QueueInfo,
	queues map[common_info.QueueID]*queue_info.QueueInfo,
) *metav1.Duration {
	// Get hierarchy paths for both queues
	preemptorPath := mr.getQueueHierarchyPath(preemptorQueue, queues)
	preempteePath := mr.getQueueHierarchyPath(preempteeQueue, queues)

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
		// If there's a path down from LCA to preemptee
		if lcaIndex+1 < len(preempteePath) {
			// Walk 1 step down to the child of the LCA that is an ancestor to the preemptee's leaf queue
			childQueue := preempteePath[lcaIndex+1]

			// Check if this queue has reclaim-min-runtime set
			if childQueue.ReclaimMinRuntime != nil {
				return childQueue.ReclaimMinRuntime
			}

			// Otherwise move back up towards root and select first available override
			for i := lcaIndex; i >= 0; i-- {
				if preempteePath[i].ReclaimMinRuntime != nil {
					return preempteePath[i].ReclaimMinRuntime
				}
			}
		} else {
			// If LCA is the preemptee queue itself, check its reclaim-min-runtime
			lca := preempteePath[lcaIndex]
			if lca.ReclaimMinRuntime != nil {
				return lca.ReclaimMinRuntime
			}

			// Otherwise move up towards root
			for i := lcaIndex - 1; i >= 0; i-- {
				if preempteePath[i].ReclaimMinRuntime != nil {
					return preempteePath[i].ReclaimMinRuntime
				}
			}
		}
	}

	// If no reclaim-min-runtime is found in the hierarchy, use default
	return mr.defaultReclaimMinRuntime
}

// getQueueHierarchyPath returns the queue hierarchy from root to the specified queue
func (mr *minruntimePlugin) getQueueHierarchyPath(
	queue *queue_info.QueueInfo,
	queues map[common_info.QueueID]*queue_info.QueueInfo,
) []*queue_info.QueueInfo {
	var hierarchyPath []*queue_info.QueueInfo

	// Start with the queue itself
	currentQueue := queue

	// Build the path from queue to root
	for currentQueue != nil {
		// Add current queue to the path (at beginning to maintain root->leaf order)
		hierarchyPath = append([]*queue_info.QueueInfo{currentQueue}, hierarchyPath...)

		// If queue has a parent, add it to the path
		if currentQueue.ParentQueue != "" {
			parentQueue, found := queues[currentQueue.ParentQueue]
			if found {
				currentQueue = parentQueue
				continue
			}
		}

		// Break if no more parents (reached root)
		break
	}

	return hierarchyPath
}
