// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package queue_info

import "github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/common_info"

type QueueQuota struct {
	GPU    ResourceQuota `json:"gpu,omitempty"`
	CPU    ResourceQuota `json:"cpu,omitempty"`
	Memory ResourceQuota `json:"memory,omitempty"`
}

type QueueUsage QueueQuota
type ClusterUsage struct {
	Cluster QueueUsage                          `json:"cluster"`
	Queues  map[common_info.QueueID]*QueueUsage `json:"queues"`
}

func NewClusterUsage() *ClusterUsage {
	return &ClusterUsage{
		Cluster: QueueUsage{},
		Queues:  make(map[common_info.QueueID]*QueueUsage),
	}
}

type ResourceQuota struct {
	// +optional
	Quota float64 `json:"deserved"`
	// +optional
	OverQuotaWeight float64 `json:"overQuotaWeight"`
	// +optional
	Limit float64 `json:"limit"`
}
