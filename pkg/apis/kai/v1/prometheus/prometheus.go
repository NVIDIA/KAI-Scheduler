// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

// +kubebuilder:object:generate:=true
package prometheus

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/NVIDIA/KAI-scheduler/pkg/apis/kai/v1/common"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

type Prometheus struct {
	// Enabled defines whether a Prometheus instance should be deployed
	// +kubebuilder:validation:Optional
	Enabled *bool `json:"enabled,omitempty"`

	// RetentionPeriod defines how long to retain data (e.g., "2w", "1d", "30d")
	// +kubebuilder:validation:Optional
	RetentionPeriod *string `json:"retentionPeriod,omitempty"`

	// SampleInterval defines the interval of sampling (e.g., "1m", "30s", "5m")
	// +kubebuilder:validation:Optional
	SampleInterval *string `json:"sampleInterval,omitempty"`

	// StorageSize defines the size of the storage (e.g., "20Gi", "30Gi")
	// +kubebuilder:validation:Optional
	StorageSize *string `json:"storageSize,omitempty"`

	// StorageClassName defines the name of the storageClass that will be used to store the TSDB data. defaults to "standard".
	// +kubebuilder:validation:Optional
	StorageClassName *string `json:"storageClassName,omitempty"`
}

func (p *Prometheus) SetDefaultsWhereNeeded() {
	if p == nil {
		return
	}
	p.Enabled = common.SetDefault(p.Enabled, ptr.To(false))
	p.RetentionPeriod = common.SetDefault(p.RetentionPeriod, ptr.To("2w"))
	p.SampleInterval = common.SetDefault(p.SampleInterval, ptr.To("1m"))
	p.StorageClassName = common.SetDefault(p.StorageClassName, ptr.To("standard"))

}

// CalculateStorageSize estimates the required storage size based on TSDB parameters according to design
func (p *Prometheus) CalculateStorageSize(ctx context.Context, client client.Reader) (string, error) {

	if p.StorageSize != nil {
		return *p.StorageSize, nil
	}

	logger := log.FromContext(ctx)
	defaultStorageSize := "30Gi"
	// Get number of NodePools (SchedulingShards)
	nodePools, err := p.getNodePoolCount(ctx, client)
	if err != nil {
		logger.Error(err, "Failed to get NodePool count")
		return defaultStorageSize, err // Fallback to default
	}

	// Get number of Queues
	numQueues, err := p.getQueueCount(ctx, client)
	if err != nil {
		logger.Error(err, "Failed to get Queue count")
		return defaultStorageSize, err // Fallback to default
	}

	// Parse retention period to minutes
	retentionMinutes, err := p.parseDurationToMinutes(p.RetentionPeriod)
	if err != nil {
		logger.Error(err, "Failed to parse retention period")
		return defaultStorageSize, err // Fallback to default
	}

	// Parse sample frequency to minutes
	sampleIntervalMinutes, err := p.parseDurationToMinutes(p.SampleInterval)
	if err != nil {
		logger.Error(err, "Failed to parse sample frequency")
		return defaultStorageSize, err // Fallback to default
	}

	// Calculate storage size using the formula
	sampleSize := 2.0           // [bytes]
	recordedResourcesLen := 5.0 // overspec the storage size for future growth
	storageSizeGi := ((sampleSize * recordedResourcesLen * float64(nodePools) * float64(numQueues) * float64(retentionMinutes)) / float64(sampleIntervalMinutes)) / (1024 * 1024 * 1024)

	// Convert to Gi string, ensuring minimum of 1Gi
	if storageSizeGi < 1.0 {
		storageSizeGi = 1.0
	}

	logger.Info("Calculated storage size",
		"nodePools", nodePools,
		"numQueues", numQueues,
		"retentionMinutes", retentionMinutes,
		"sampleIntervalMinutes", sampleIntervalMinutes,
		"storageSizeGi", storageSizeGi)

	return fmt.Sprintf("%.0fGi", storageSizeGi), nil
}

// getNodePoolCount returns the number of NodePools (SchedulingShards) in the cluster
func (p *Prometheus) getNodePoolCount(ctx context.Context, client client.Reader) (int, error) {
	// Use unstructured objects to avoid import cycles
	logger := log.FromContext(ctx)
	shardList := &unstructured.UnstructuredList{}
	shardList.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "kai.scheduler",
		Version: "v1",
		Kind:    "SchedulingShardList",
	})
	err := client.List(ctx, shardList)
	if err != nil {
		return 0, fmt.Errorf("failed to list SchedulingShards: %v", err)
	}

	if len(shardList.Items) == 0 {
		logger.Info("No SchedulingShards found, using default nodePool count of 1")
		return 1, nil
	}

	return len(shardList.Items), nil
}

// getQueueCount returns the number of Queues in the cluster
func (p *Prometheus) getQueueCount(ctx context.Context, client client.Reader) (int, error) {
	logger := log.FromContext(ctx)

	// Get all Queue CRs from Group scheduling.run.ai
	queueList := &unstructured.UnstructuredList{}
	queueList.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "scheduling.run.ai",
		Version: "v2",
		Kind:    "Queue",
	})

	err := client.List(ctx, queueList)
	if err != nil {
		logger.Info("Failed to list Queues, using default queue count of 1", "error", err)
		return 1, nil // Return 1 as default if we can't list queues
	}

	if len(queueList.Items) == 0 {
		logger.Info("No Queues found, using default queue count of 1")
		return 1, nil
	}

	return len(queueList.Items), nil
}

// parseDurationToMinutes parses duration strings like "2w", "1d", "30m", "1h" to minutes
func (p *Prometheus) parseDurationToMinutes(duration *string) (int, error) {
	if duration == nil {
		return 0, fmt.Errorf("duration is nil")
	}

	durationStr := strings.TrimSpace(*duration)
	if durationStr == "" {
		return 0, fmt.Errorf("duration is empty")
	}

	// Parse the duration string
	durationValue, err := time.ParseDuration(durationStr)
	if err != nil {
		// Try to parse custom formats like "2w", "1d"
		return p.parseCustomDuration(durationStr)
	}

	minutes := int(durationValue.Minutes())
	if minutes == 0 {
		minutes = 1 // Ensure minimum 1 minute
	}
	return minutes, nil
}

// parseCustomDuration parses custom duration formats like "2w", "1d", "30m"
func (p *Prometheus) parseCustomDuration(durationStr string) (int, error) {
	if len(durationStr) < 2 {
		return 0, fmt.Errorf("invalid duration format: %s", durationStr)
	}

	// Extract number and unit
	numberStr := durationStr[:len(durationStr)-1]
	unit := durationStr[len(durationStr)-1:]

	number, err := strconv.Atoi(numberStr)
	if err != nil {
		return 0, fmt.Errorf("invalid number in duration: %s", numberStr)
	}

	// Convert to minutes based on unit
	switch unit {
	case "s": // seconds
		return number / 60, nil // Convert seconds to minutes (round down)
	case "m": // minutes
		return number, nil
	case "h": // hours
		return number * 60, nil
	case "d": // days
		return number * 24 * 60, nil
	case "w": // weeks
		return number * 7 * 24 * 60, nil
	case "M": // months (approximate as 30 days)
		return number * 30 * 24 * 60, nil
	case "y": // years (approximate as 365 days)
		return number * 365 * 24 * 60, nil
	default:
		return 0, fmt.Errorf("unsupported duration unit: %s", unit)
	}
}
