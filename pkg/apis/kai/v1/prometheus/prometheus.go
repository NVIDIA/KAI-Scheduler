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

	// TSDB defines the configuration for Prometheus TSDB storage
	// +kubebuilder:validation:Optional
	TSDB *TSDB `json:"tsdb,omitempty"`

	// Status defines the observed state of the Prometheus TSDB
	// +kubebuilder:validation:Optional
	Status *TSDBStatus `json:"status,omitempty"`
}

type TSDB struct {
	// Connection defines the connection configuration for TSDB
	// +kubebuilder:validation:Optional
	Connection *Connection `json:"connection,omitempty"`

	// RetentionPeriod defines how long to retain data (e.g., "2w", "1d", "30d")
	// +kubebuilder:validation:Optional
	RetentionPeriod *string `json:"retentionPeriod,omitempty"`

	// SampleFrequency defines the frequency of sampling (e.g., "1m", "30s", "5m")
	// +kubebuilder:validation:Optional
	SampleFrequency *string `json:"sampleFrequency,omitempty"`
}

type Connection struct {
	// URL defines the connection URL for TSDB
	// +kubebuilder:validation:Optional
	URL *string `json:"url,omitempty"`

	// AuthSecretName defines the name of the secret containing authentication credentials
	// +kubebuilder:validation:Optional
	AuthSecretName *string `json:"authSecretName,omitempty"`
}

type TSDBStatus struct {
	// State defines the current state of the TSDB
	// +kubebuilder:validation:Optional
	State *string `json:"state,omitempty"`

	// Reason defines the reason for the current state
	// +kubebuilder:validation:Optional
	Reason *string `json:"reason,omitempty"`
}

func (p *Prometheus) SetDefaultsWhereNeeded() {
	if p == nil {
		return
	}
	if p.Enabled == nil {
		p.Enabled = ptr.To(false)
	}

	if p.TSDB == nil {
		p.TSDB = &TSDB{}
	}
	p.TSDB.SetDefaultsWhereNeeded()
}

func (t *TSDB) SetDefaultsWhereNeeded() {
	if t == nil {
		return
	}
	if t.RetentionPeriod == nil {
		t.RetentionPeriod = ptr.To("2w")
	}

	if t.SampleFrequency == nil {
		t.SampleFrequency = ptr.To("1m")
	}

	if t.Connection == nil {
		t.Connection = &Connection{}
	}
}

// CalculateStorageSize estimates the required storage size based on TSDB parameters according to design
func (t *TSDB) CalculateStorageSize(ctx context.Context, client client.Reader) (string, error) {
	logger := log.FromContext(ctx)
	defaultStorageSize := "30Gi"
	// Get number of NodePools (SchedulingShards)
	nodePools, err := t.getNodePoolCount(ctx, client)
	if err != nil {
		logger.Error(err, "Failed to get NodePool count")
		return defaultStorageSize, err // Fallback to default
	}

	// Get number of Queues
	numQueues, err := t.getQueueCount(ctx, client)
	if err != nil {
		logger.Error(err, "Failed to get Queue count")
		return defaultStorageSize, err // Fallback to default
	}

	// Parse retention period to minutes
	retentionMinutes, err := t.parseDurationToMinutes(t.RetentionPeriod)
	if err != nil {
		logger.Error(err, "Failed to parse retention period")
		return defaultStorageSize, err // Fallback to default
	}

	// Parse sample frequency to minutes
	sampleFrequencyMinutes, err := t.parseDurationToMinutes(t.SampleFrequency)
	if err != nil {
		logger.Error(err, "Failed to parse sample frequency")
		return defaultStorageSize, err // Fallback to default
	}

	// Calculate storage size using the formula
	sampleSize := 2.0           // [bytes]
	recordedResourcesLen := 5.0 //
	storageSizeGi := ((sampleSize * recordedResourcesLen * float64(nodePools) * float64(numQueues) * float64(retentionMinutes)) / float64(sampleFrequencyMinutes)) / (1024 * 1024 * 1024)

	logger.Info("Calculated storage size",
		"nodePools", nodePools,
		"numQueues", numQueues,
		"retentionMinutes", retentionMinutes,
		"sampleFrequencyMinutes", sampleFrequencyMinutes,
		"storageSizeGi", storageSizeGi)

	// Convert to Gi string, ensuring minimum of 1Gi
	if storageSizeGi < 1.0 {
		storageSizeGi = 1.0
	}
	return fmt.Sprintf("%.0fGi", storageSizeGi), nil
}

// getNodePoolCount returns the number of NodePools (SchedulingShards) in the cluster
func (t *TSDB) getNodePoolCount(ctx context.Context, client client.Reader) (int, error) {
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
func (t *TSDB) getQueueCount(ctx context.Context, client client.Reader) (int, error) {
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
func (t *TSDB) parseDurationToMinutes(duration *string) (int, error) {
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
		return t.parseCustomDuration(durationStr)
	}

	minutes := int(durationValue.Minutes())
	if minutes == 0 {
		minutes = 1 // Ensure minimum 1 minute
	}
	return minutes, nil
}

// parseCustomDuration parses custom duration formats like "2w", "1d", "30m"
func (t *TSDB) parseCustomDuration(durationStr string) (int, error) {
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
