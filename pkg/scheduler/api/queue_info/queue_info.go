// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package queue_info

import (
	"golang.org/x/exp/slices"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	enginev2 "github.com/NVIDIA/KAI-scheduler/pkg/apis/scheduling/v2"
	commonconstants "github.com/NVIDIA/KAI-scheduler/pkg/common/constants"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/common_info"
)

type QueueInfo struct {
	UID               common_info.QueueID   `json:"uid,omitempty"`
	Name              string                `json:"name,omitempty"`
	ParentQueue       common_info.QueueID   `json:"parentQueue,omitempty"`
	ChildQueues       []common_info.QueueID `json:"childQueues,omitempty"`
	Resources         QueueQuota            `json:"resources,omitempty"`
	Priority          int                   `json:"priority,omitempty"`
	CreationTimestamp metav1.Time           `json:"creationTimestamp,omitempty"`
}

func NewQueueInfo(queue *enginev2.Queue) *QueueInfo {
	queueName := queue.Name
	if queue.Spec.DisplayName != "" {
		queueName = queue.Spec.DisplayName
	}

	priority := commonconstants.DefaultQueuePriority
	if queue.Spec.Priority != nil {
		priority = *queue.Spec.Priority
	}

	return &QueueInfo{
		UID:               common_info.QueueID(queue.Name),
		Name:              queueName,
		ParentQueue:       common_info.QueueID(queue.Spec.ParentQueue),
		ChildQueues:       []common_info.QueueID{}, // ToDo: Calculate from queue status once we reflect it there
		Resources:         getQueueQuota(*queue),
		Priority:          priority,
		CreationTimestamp: queue.CreationTimestamp,
	}
}

func (q *QueueInfo) IsLeafQueue() bool {
	return len(q.ChildQueues) == 0
}

func (q *QueueInfo) AddChildQueue(queue common_info.QueueID) {
	if slices.Contains(q.ChildQueues, queue) {
		return
	}
	q.ChildQueues = append(q.ChildQueues, queue)
}

func getQueueQuota(queue enginev2.Queue) QueueQuota {
	if queue.Spec.Resources == nil {
		return QueueQuota{}
	}

	return QueueQuota{
		GPU:    ResourceQuota(queue.Spec.Resources.GPU),
		CPU:    ResourceQuota(queue.Spec.Resources.CPU),
		Memory: ResourceQuota(queue.Spec.Resources.Memory),
	}
}
