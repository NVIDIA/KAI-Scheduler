// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package cluster_info

import (
	"fmt"

	"github.com/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	enginev2 "github.com/NVIDIA/KAI-scheduler/pkg/apis/scheduling/v2"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/common_info"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/queue_info"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/log"
)

const (
	defaultQueueName = "default"
)

func (c *ClusterInfo) getDefaultParentQueue() *queue_info.QueueInfo {
	queue := &enginev2.Queue{
		ObjectMeta: metav1.ObjectMeta{
			Name:              defaultQueueName,
			CreationTimestamp: metav1.Now(),
		},
		Spec: enginev2.QueueSpec{
			Resources: &enginev2.QueueResources{
				GPU: enginev2.QueueResource{
					Quota:           -1,
					OverQuotaWeight: 1,
					Limit:           -1,
				},
				CPU: enginev2.QueueResource{
					Quota:           -1,
					OverQuotaWeight: 1,
					Limit:           -1,
				},
				Memory: enginev2.QueueResource{
					Quota:           -1,
					OverQuotaWeight: 1,
					Limit:           -1,
				},
			},
		},
	}
	return queue_info.NewQueueInfo(queue)
}

// snapshotQueues creates a snapshot of all queues in the cluster.
// A synthetic "default" root queue is always created to serve as the parent for:
//   - Orphan queues: queues without a parent (ParentQueue is empty) are adopted by the default queue
//   - Flat hierarchy: in ProjectLevelFairness mode, ALL queues are flattened under the default queue
//
// Note: This is different from cleanQueueOrphans which handles queues that reference a non-existent
// parent - those queues are deleted rather than adopted.
func (c *ClusterInfo) snapshotQueues() (map[common_info.QueueID]*queue_info.QueueInfo, error) {
	queues, err := c.dataLister.ListQueues()
	if err != nil {
		err = errors.WithStack(fmt.Errorf("error listing queues: %w", err))
		return nil, err
	}

	result := map[common_info.QueueID]*queue_info.QueueInfo{}
	defaultParentQueue := c.getDefaultParentQueue()
	result[defaultParentQueue.UID] = defaultParentQueue
	for _, queue := range queues {
		// Adopt orphan queues (no parent specified) under the default root queue.
		// In ProjectLevelFairness mode, flatten all queues under the default root.
		if len(queue.Spec.ParentQueue) == 0 || c.fairnessLevelType == ProjectLevelFairness {
			queue.Spec.ParentQueue = defaultQueueName
		}
		queueInfo := queue_info.NewQueueInfo(queue)
		result[queueInfo.UID] = queueInfo
	}

	return result, nil
}

func (c *ClusterInfo) snapshotQueueResourceUsage() (*queue_info.ClusterUsage, error) {
	if !c.collectUsageData {
		return nil, nil
	}

	return c.dataLister.ListResourceUsage()
}

// UpdateQueueHierarchy iterates over a map containing multiple levels of queue hierarchies, and updates queues with
// child queues where relevant
func UpdateQueueHierarchy(queues map[common_info.QueueID]*queue_info.QueueInfo) {
	updateQueueChildren(queues)
	cleanQueueOrphans(queues)
}

func updateQueueChildren(queues map[common_info.QueueID]*queue_info.QueueInfo) {
	for queueId, queue := range queues {
		if queue.ParentQueue != "" {
			if parent, found := queues[queue.ParentQueue]; found {
				parent.AddChildQueue(queueId)
			}
		}
	}
}

// cleanQueueOrphans removes queues that reference a non-existent parent queue.
// Note: This handles a different case than orphan adoption in snapshotQueues:
//   - snapshotQueues adopts queues with NO parent (empty ParentQueue) under the default root
//   - cleanQueueOrphans deletes queues that reference a MISSING parent (parent doesn't exist in the map)
func cleanQueueOrphans(queues map[common_info.QueueID]*queue_info.QueueInfo) {
	for queueId, queue := range queues {
		if queue.ParentQueue != "" {
			if _, found := queues[queue.ParentQueue]; !found {
				log.InfraLogger.V(2).Warnf("Found orphan queue %s with missing parent %s, deleting it and all children",
					queueId, queue.ParentQueue)
				deleteQueueAndChildren(queues, queueId)
			}
		}
	}
}

func deleteQueueAndChildren(queues map[common_info.QueueID]*queue_info.QueueInfo, queueID common_info.QueueID) {
	queue, found := queues[queueID]
	if !found {
		log.InfraLogger.V(2).Warnf("Could not find queue %s in queues map when deleting children", queueID)
		return
	}

	log.InfraLogger.V(6).Infof("Deleting queue %s and children %s", queueID, queue.ChildQueues)
	for _, child := range queue.ChildQueues {
		deleteQueueAndChildren(queues, child)
	}
	delete(queues, queueID)
}
