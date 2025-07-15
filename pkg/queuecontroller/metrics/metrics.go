// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package metrics

import (
	"sort"
	"strconv"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto" // auto-registry collectors in default registry

	v2 "github.com/NVIDIA/KAI-scheduler/pkg/apis/scheduling/v2"
)

const (
	milliCpuToCpuDivider       = 1000
	megabytesToBytesMultiplier = 1000000
	unlimitedQuota             = float64(-1)

	queueNameLabel = "queue_name"
)

var (
	queueInfo         *prometheus.GaugeVec
	queueDeservedGPUs *prometheus.GaugeVec
	queueQuotaCPU     *prometheus.GaugeVec
	queueQuotaMemory  *prometheus.GaugeVec

	additionalQueueLabelKeys       []string
	queueLabelToDefaultMetricValue map[string]string
)

// InitMetrics initializes the metrics for the queue controller.
// params:
//
//	namespace: the Prometheus namespace for the metrics
//	queueLabelToMetricLabelMap: a map of queue label keys to metric label keys
//	queueLabelToDefaultMetricValueMap: a map of queue label keys to default metric values
//
// For example, if a queue has a label "priority" with value "high",
// and you want to use it as a metric label "queue_priority",
// with a default value of "normal" if the label is not present,
// you would pass:
// queueLabelToMetricLabelMap        = map[string]string{"priority": "queue_priority"}
// queueLabelToDefaultMetricValueMap = map[string]string{"priority": "normal"}
func InitMetrics(namespace string, queueLabelToMetricLabelMap, queueLabelToDefaultMetricValueMap map[string]string) {
	// Sort the keys to ensure consistent order
	sortedQueueLabelKeys := make([]string, 0, len(queueLabelToMetricLabelMap))
	for key := range queueLabelToMetricLabelMap {
		sortedQueueLabelKeys = append(sortedQueueLabelKeys, key)
	}
	sort.Strings(sortedQueueLabelKeys)

	additionalMetricLabelKeys := make([]string, 0, len(queueLabelToMetricLabelMap))
	for _, queueLabelKey := range sortedQueueLabelKeys {
		metricLabelKey := queueLabelToMetricLabelMap[queueLabelKey]
		additionalQueueLabelKeys = append(additionalQueueLabelKeys, queueLabelKey)
		additionalMetricLabelKeys = append(additionalMetricLabelKeys, metricLabelKey)
	}

	queueLabelToDefaultMetricValue = queueLabelToDefaultMetricValueMap

	queueInfo = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "queue_info",
			Help:      "Queues info",
		}, append([]string{queueNameLabel, "gpu_guaranteed_quota", "cpu_quota", "memory_quota"}, additionalMetricLabelKeys...),
	)

	queueDeservedGPUs = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "queue_deserved_gpus",
			Help:      "Queue deserved GPUs",
		}, append([]string{queueNameLabel}, additionalMetricLabelKeys...),
	)

	queueQuotaCPU = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "queue_quota_cpu_cores",
			Help:      "Queue quota CPU",
		}, append([]string{queueNameLabel}, additionalMetricLabelKeys...),
	)

	queueQuotaMemory = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "queue_quota_memory_bytes",
			Help:      "Queue quota memory",
		}, append([]string{queueNameLabel}, additionalMetricLabelKeys...),
	)
}

func SetQueueMetrics(queue *v2.Queue) {
	if queue == nil {
		return
	}

	ResetQueueMetrics(queue.Name)

	additionalMetricLabelValues := getAdditionalMetricLabelValues(queue.Labels)

	queueName := queue.Name
	gpuQuota := getGpuQuota(queue.Spec.Resources)
	cpuQuota := getCpuQuota(queue.Spec.Resources)
	memoryQuota := getMemoryQuota(queue.Spec.Resources)

	queueInfoMetricValues := append([]string{queueName,
		strconv.FormatFloat(gpuQuota, 'f', -1, 64),
		strconv.FormatFloat(cpuQuota, 'f', -1, 64),
		strconv.FormatInt(int64(memoryQuota), 10)}, additionalMetricLabelValues...)
	queueQuotaMetricValues := append([]string{queueName}, additionalMetricLabelValues...)

	queueInfo.WithLabelValues(queueInfoMetricValues...).Set(1)
	queueDeservedGPUs.WithLabelValues(queueQuotaMetricValues...).Set(gpuQuota)
	queueQuotaCPU.WithLabelValues(queueQuotaMetricValues...).Set(cpuQuota)
	queueQuotaMemory.WithLabelValues(queueQuotaMetricValues...).Set(memoryQuota)
}

func ResetQueueMetrics(queueName string) {
	queueLabelIdentifier := prometheus.Labels{queueNameLabel: queueName}
	queueInfo.DeletePartialMatch(queueLabelIdentifier)
	queueDeservedGPUs.DeletePartialMatch(queueLabelIdentifier)
	queueQuotaCPU.DeletePartialMatch(queueLabelIdentifier)
	queueQuotaMemory.DeletePartialMatch(queueLabelIdentifier)
}

func getGpuQuota(queueSpecResources *v2.QueueResources) float64 {
	if queueSpecResources == nil {
		return float64(0)
	}
	return queueSpecResources.GPU.Quota
}

func getCpuQuota(queueSpecResources *v2.QueueResources) float64 {
	if queueSpecResources == nil {
		return float64(0)
	}
	cpuQuota := queueSpecResources.CPU.Quota
	if cpuQuota == unlimitedQuota {
		return unlimitedQuota
	}
	return queueSpecResources.CPU.Quota / milliCpuToCpuDivider
}

func getMemoryQuota(queueSpecResources *v2.QueueResources) float64 {
	if queueSpecResources == nil {
		return float64(0)
	}
	memoryQuota := queueSpecResources.Memory.Quota
	if memoryQuota == unlimitedQuota {
		return unlimitedQuota
	}
	return memoryQuota * megabytesToBytesMultiplier
}

func getAdditionalMetricLabelValues(queueLabels map[string]string) []string {
	labelValues := make([]string, len(additionalQueueLabelKeys))

	// we already added the additional metric labels to each metric using the original order,
	// so we can just iterate over the additionalQueueLabelKeys - they should have the same order.

	for i, queueLabelKey := range additionalQueueLabelKeys {
		if value, exists := queueLabels[queueLabelKey]; exists {
			labelValues[i] = value
		} else if defaultValue, defaultExists := queueLabelToDefaultMetricValue[queueLabelKey]; defaultExists {
			labelValues[i] = defaultValue
		} else {
			labelValues[i] = "" // Default to empty string if no value exists
		}
	}
	return labelValues

}
