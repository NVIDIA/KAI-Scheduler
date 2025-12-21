// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"sigs.k8s.io/controller-runtime/pkg/metrics"
)

const (
	podGroupNameLabel      = "pod_group_name"
	podGroupNamespaceLabel = "pod_group_namespace"
	podGroupUUIDLabel      = "pod_group_uuid"
	nodepoolLabel          = "nodepool"
	evictorActionLabel     = "evictor_action"
)

var (
	initiated = false

	podGroupEvictionsCount      *prometheus.CounterVec
	podGroupNotEvictedPodsCount *prometheus.CounterVec
)

// InitMetrics initializes the metrics for the pod group controller.
func InitMetrics(namespace string) {
	if initiated {
		return
	}
	initiated = true

	podGroupLabels := []string{
		podGroupNameLabel,
		podGroupNamespaceLabel,
		podGroupUUIDLabel,
		nodepoolLabel,
		evictorActionLabel,
	}

	podGroupEvictionsCount = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "pod_group_evictions_counter",
			Help:      "Number of pods evictions for pod-group",
		}, podGroupLabels,
	)

	podGroupNotEvictedPodsCount = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "pod_group_non_evicted_pods_counter",
			Help:      "Number of pods non-evicted for pod-group",
		}, podGroupLabels,
	)

	metrics.Registry.MustRegister(podGroupEvictionsCount, podGroupNotEvictedPodsCount)
}

// IncPodGroupEvictions increments the pod group evictions counter.
func IncPodGroupEvictions(podGroupName, podGroupNamespace, podGroupUUID, nodepool, evictorAction string) {
	if podGroupEvictionsCount == nil {
		return
	}
	podGroupEvictionsCount.WithLabelValues(
		podGroupName,
		podGroupNamespace,
		podGroupUUID,
		nodepool,
		evictorAction,
	).Inc()
}

// IncPodGroupNotEvictedPods increments the pod group non-evicted pods counter.
func IncPodGroupNotEvictedPods(podGroupName, podGroupNamespace, podGroupUUID, nodepool, evictorAction string) {
	if podGroupNotEvictedPodsCount == nil {
		return
	}
	podGroupNotEvictedPodsCount.WithLabelValues(
		podGroupName,
		podGroupNamespace,
		podGroupUUID,
		nodepool,
		evictorAction,
	).Inc()
}

// GetPodGroupEvictionsCount returns the pod group evictions counter metric.
func GetPodGroupEvictionsCount() *prometheus.CounterVec {
	return podGroupEvictionsCount
}

// GetPodGroupNotEvictedPodsCount returns the pod group non-evicted pods counter metric.
func GetPodGroupNotEvictedPodsCount() *prometheus.CounterVec {
	return podGroupNotEvictedPodsCount
}

