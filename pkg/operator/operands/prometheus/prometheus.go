// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package prometheus

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/NVIDIA/KAI-scheduler/pkg/operator/operands/common"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	kaiv1 "github.com/NVIDIA/KAI-scheduler/pkg/apis/kai/v1"
	kaiprometheus "github.com/NVIDIA/KAI-scheduler/pkg/apis/kai/v1/prometheus"
)

type Prometheus struct {
	namespace        string
	lastDesiredState []client.Object
	client           client.Client
}
type promethuesResourceForKAIConfig func(ctx context.Context, runtimeClient client.Reader, kaiConfig *kaiv1.Config) ([]client.Object, error)

func (p *Prometheus) DesiredState(
	ctx context.Context, runtimeClient client.Reader, kaiConfig *kaiv1.Config,
) ([]client.Object, error) {
	p.namespace = kaiConfig.Spec.Namespace
	p.client = runtimeClient.(client.Client)

	var objects []client.Object
	for _, resourceFunc := range []promethuesResourceForKAIConfig{
		prometheusForKAIConfig,
		prometheusServiceAccountForKAIConfig,
		serviceMonitorsForKAIConfig,
	} {
		obj, err := resourceFunc(ctx, runtimeClient, kaiConfig)
		if err != nil {
			return nil, err
		}
		objects = append(objects, obj...)
	}

	p.lastDesiredState = objects
	return objects, nil

}

func (b *Prometheus) IsDeployed(ctx context.Context, readerClient client.Reader) (bool, error) {
	// If there are no objects to check, consider it deployed
	if len(b.lastDesiredState) == 0 {
		return true, nil
	}
	return common.AllObjectsExists(ctx, readerClient, b.lastDesiredState)
}

func (b *Prometheus) IsAvailable(ctx context.Context, readerClient client.Reader) (bool, error) {
	// If there are no objects to check, consider it available
	if len(b.lastDesiredState) == 0 {
		return true, nil
	}

	prometheus := &monitoringv1.Prometheus{}
	err := readerClient.Get(ctx, client.ObjectKey{
		Name:      mainResourceName,
		Namespace: b.namespace,
	}, prometheus)
	if err != nil {
		return false, err
	}

	// Check if there are any conditions and if the first one is Available
	if len(prometheus.Status.Conditions) > 0 {
		for _, condition := range prometheus.Status.Conditions {
			if condition.Type == monitoringv1.ConditionType("Available") {
				return condition.Status == monitoringv1.ConditionTrue, nil
			}
		}
	}
	return false, nil
}

func (b *Prometheus) Name() string {
	return "KAI-prometheus"
}

// StartMonitoring starts a background goroutine to monitor external Prometheus connectivity
// and update the status periodically. The goroutine will stop when ctx is cancelled or when
// ExternalPrometheusUrl is set to nil or empty string.
func StartMonitoring(ctx context.Context, prometheusConfig *kaiprometheus.Prometheus, statusUpdater func(ctx context.Context, condition metav1.Condition) error) {
	if prometheusConfig == nil || prometheusConfig.ExternalPrometheusUrl == nil || *prometheusConfig.ExternalPrometheusUrl == "" {
		return
	}

	go func() {
		ticker := time.NewTicker(time.Duration(*prometheusConfig.ExternalPrometheusPingConfig.PingsInterval) * time.Second) // Check every 30 seconds
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				// Check if external Prometheus URL is still configured
				if prometheusConfig.ExternalPrometheusUrl == nil || *prometheusConfig.ExternalPrometheusUrl == "" {
					return
				}

				// Test connectivity
				err := pingExternalPrometheus(ctx, *prometheusConfig.ExternalPrometheusUrl, *prometheusConfig.ExternalPrometheusPingConfig.PingsTimeout, *prometheusConfig.ExternalPrometheusPingConfig.PingsMaxRetries)

				var condition metav1.Condition
				if err != nil {
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

// pingExternalPrometheus validates connectivity to an external Prometheus instance
func pingExternalPrometheus(ctx context.Context, prometheusURL string, timeout int, maxRetries int) error {
	logger := log.FromContext(ctx)

	// Ensure the URL has a scheme
	if !strings.Contains(prometheusURL, "://") {
		prometheusURL = "http://" + prometheusURL
	}

	// Parse the URL to ensure it's valid
	_, err := url.Parse(prometheusURL)
	if err != nil {
		return fmt.Errorf("invalid Prometheus URL: %w, prometheusURL: %s", err, prometheusURL)
	}

	// Create HTTP client with timeout
	client := &http.Client{
		Timeout: time.Duration(timeout) * time.Second,
	}

	// Try to connect to the Prometheus /api/v1/status/config endpoint
	statusURL := prometheusURL + "/api/v1/status/config"
	logger.Info("Validating external Prometheus connection", "url", statusURL)

	var lastErr error
	for attempt := 1; attempt <= maxRetries; attempt++ {
		req, err := http.NewRequestWithContext(ctx, "GET", statusURL, nil)
		if err != nil {
			return fmt.Errorf("failed to create request: %w, statusURL: %s", err, statusURL)
		}

		resp, err := client.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("failed to connect to external Prometheus: %w, statusURL: %s", err, statusURL)
			if attempt < maxRetries {
				backoff := time.Duration(attempt) * time.Second
				logger.Info("Retrying Prometheus connection", "attempt", attempt, "maxRetries", maxRetries, "nextRetryInSeconds", backoff.Seconds())
				time.Sleep(backoff)
			}
			continue
		}

		// Check if we got a successful response
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			resp.Body.Close()
			lastErr = fmt.Errorf("external Prometheus returned status code %d, statusURL: %s", resp.StatusCode, statusURL)
			if attempt < maxRetries {
				backoff := time.Duration(attempt) * time.Second
				logger.Info("Retrying Prometheus connection", "attempt", attempt, "maxRetries", maxRetries, "statusCode", resp.StatusCode, "nextRetryInSeconds", backoff.Seconds())
				time.Sleep(backoff)
			}
			continue
		}

		resp.Body.Close()
		return nil
	}

	return fmt.Errorf("failed to connect to external Prometheus after %d attempts: %w", maxRetries, lastErr)
}

// ValidateExternalPrometheusConnection validates connectivity to an external Prometheus instance
func ValidateExternalPrometheusConnection(ctx context.Context, prometheusConfig *kaiprometheus.Prometheus) error {
	// Check if external Prometheus URL is configured
	if prometheusConfig == nil || prometheusConfig.ExternalPrometheusUrl == nil || *prometheusConfig.ExternalPrometheusUrl == "" {
		return nil
	}

	// Validate the connection once
	err := pingExternalPrometheus(ctx, *prometheusConfig.ExternalPrometheusUrl, *prometheusConfig.ExternalPrometheusPingConfig.PingsTimeout, *prometheusConfig.ExternalPrometheusPingConfig.PingsMaxRetries)
	if err != nil {
		return fmt.Errorf("failed to ping external Prometheus: %w", err)
	}
	return nil
}
