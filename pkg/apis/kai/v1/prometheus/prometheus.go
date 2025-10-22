// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

// +kubebuilder:object:generate:=true
package prometheus

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/NVIDIA/KAI-scheduler/pkg/apis/kai/v1/common"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

	// ServiceMonitor defines ServiceMonitor configuration for KAI services
	// +kubebuilder:validation:Optional
	ServiceMonitor *ServiceMonitor `json:"serviceMonitor,omitempty"`

	// ExternalPrometheusUrl defines the URL of an external Prometheus instance to use
	// When set, KAI will not deploy its own Prometheus but will configure ServiceMonitors
	// for the external instance and validate connectivity
	// +kubebuilder:validation:Optional
	ExternalPrometheusUrl *string `json:"externalPrometheusUrl,omitempty"`
}

func (p *Prometheus) SetDefaultsWhereNeeded() {
	if p == nil {
		return
	}
	p.Enabled = common.SetDefault(p.Enabled, ptr.To(false))
	p.RetentionPeriod = common.SetDefault(p.RetentionPeriod, ptr.To("2w"))
	p.SampleInterval = common.SetDefault(p.SampleInterval, ptr.To("1m"))
	p.StorageClassName = common.SetDefault(p.StorageClassName, ptr.To("standard"))
	p.ExternalPrometheusUrl = common.SetDefault(p.ExternalPrometheusUrl, nil)

	p.ServiceMonitor = common.SetDefault(p.ServiceMonitor, &ServiceMonitor{})
	p.ServiceMonitor.SetDefaultsWhereNeeded()
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

// ServiceMonitor defines ServiceMonitor configuration for KAI services
type ServiceMonitor struct {
	// Enabled defines whether ServiceMonitor resources should be created
	// +kubebuilder:validation:Optional
	Enabled *bool `json:"enabled,omitempty"`

	// Interval defines the scrape interval for the ServiceMonitor (e.g., "30s", "1m")
	// +kubebuilder:validation:Optional
	Interval *string `json:"interval,omitempty"`

	// ScrapeTimeout defines the scrape timeout for the ServiceMonitor (e.g., "10s", "30s")
	// +kubebuilder:validation:Optional
	ScrapeTimeout *string `json:"scrapeTimeout,omitempty"`

	// BearerTokenFile defines the path to the bearer token file for authentication
	// +kubebuilder:validation:Optional
	BearerTokenFile *string `json:"bearerTokenFile,omitempty"`
}

func (s *ServiceMonitor) SetDefaultsWhereNeeded() {
	if s == nil {
		return
	}
	s.Enabled = common.SetDefault(s.Enabled, ptr.To(true))
	s.Interval = common.SetDefault(s.Interval, ptr.To("30s"))
	s.ScrapeTimeout = common.SetDefault(s.ScrapeTimeout, ptr.To("10s"))
	s.BearerTokenFile = common.SetDefault(s.BearerTokenFile, ptr.To("/var/run/secrets/kubernetes.io/serviceaccount/token"))
}

func (p *Prometheus) ValidateExternalPrometheusConnection(ctx context.Context, externalPrometheusUrl string) error {
	// Check if external Prometheus URL is configured
	if p.ExternalPrometheusUrl == nil || *p.ExternalPrometheusUrl == "" {
		return nil
	}

	// Validate the connection once
	ok, err := pingExternalPrometheus(ctx, *p.ExternalPrometheusUrl)
	if err != nil || !ok {
		return fmt.Errorf("failed to ping external Prometheus: %w", err)
	}
	return nil
}

// StartPrometheusMonitoring starts a background goroutine to monitor external Prometheus connectivity
// and update the status periodically. The goroutine will stop when ctx is cancelled or when
// ExternalPrometheusUrl is set to nil or empty string.
func (p *Prometheus) StartPrometheusMonitoring(ctx context.Context, statusUpdater func(ctx context.Context, condition metav1.Condition) error) {
	if p.ExternalPrometheusUrl == nil || *p.ExternalPrometheusUrl == "" {
		return
	}

	go func() {
		ticker := time.NewTicker(30 * time.Second) // Check every 30 seconds
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				// Check if external Prometheus URL is still configured
				if p.ExternalPrometheusUrl == nil || *p.ExternalPrometheusUrl == "" {
					return
				}

				// Test connectivity
				ok, err := pingExternalPrometheus(ctx, *p.ExternalPrometheusUrl)

				var condition metav1.Condition
				if err != nil || !ok {
					condition = metav1.Condition{
						Type:               "PrometheusConnectivity",
						Status:             metav1.ConditionFalse,
						Reason:             "prometheus_connection_failed",
						Message:            fmt.Sprintf("Failed to ping external Prometheus: %v", err),
						LastTransitionTime: metav1.Now(),
					}
				} else {
					condition = metav1.Condition{
						Type:               "PrometheusConnectivity",
						Status:             metav1.ConditionTrue,
						Reason:             "prometheus_connected",
						Message:            "External Prometheus connectivity verified",
						LastTransitionTime: metav1.Now(),
					}
				}

				// Update status
				if updateErr := statusUpdater(ctx, condition); updateErr != nil {
					logger := log.FromContext(ctx)
					logger.Error(updateErr, "Failed to update Prometheus connectivity status")
				}

			case <-ctx.Done():
				return
			}
		}
	}()
}

// validateExternalPrometheusConnection validates connectivity to an external Prometheus instance
func pingExternalPrometheus(ctx context.Context, prometheusURL string) (bool, error) {
	logger := log.FromContext(ctx)

	// Ensure the URL has a scheme
	if !strings.Contains(prometheusURL, "://") {
		prometheusURL = "http://" + prometheusURL
	}

	// Parse the URL to ensure it's valid
	_, err := url.Parse(prometheusURL)
	if err != nil {
		return false, fmt.Errorf("invalid Prometheus URL: %w", err)
	}

	// Create HTTP client with timeout
	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	// Try to connect to the Prometheus /api/v1/status/config endpoint
	statusURL := prometheusURL + "/api/v1/status/config"
	logger.Info("Validating external Prometheus connection", "url", statusURL)

	req, err := http.NewRequestWithContext(ctx, "GET", statusURL, nil)
	if err != nil {
		return false, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return false, fmt.Errorf("failed to connect to external Prometheus: %w", err)
	}
	defer resp.Body.Close()

	// Check if we got a successful response
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return false, fmt.Errorf("external Prometheus returned status code %d", resp.StatusCode)
	}
	return true, nil
}
