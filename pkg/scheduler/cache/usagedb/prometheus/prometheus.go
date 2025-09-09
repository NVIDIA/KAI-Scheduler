// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package prometheus

import (
	"context"
	"fmt"
	"time"

	commonconstants "github.com/NVIDIA/KAI-scheduler/pkg/common/constants"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/common_info"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/queue_info"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/cache/usagedb/api"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/log"
	"github.com/aptible/supercronic/cronexpr"
	promapi "github.com/prometheus/client_golang/api"
	promv1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
	v1 "k8s.io/api/core/v1"
)

const (
	queueNameLabel = "queue_name"
)

type queryUsageFunction func(ctx context.Context, allocationMetric string) (map[common_info.QueueID]float64, error)

var _ api.Interface = &PrometheusClient{}

type PrometheusClient struct {
	client      promv1.API
	promClient  promapi.Client
	usageParams *api.UsageParams

	// Extra params
	usageQueryTimeout            time.Duration
	queryResolution              time.Duration
	allocationMetricsMap         map[string]string
	queryUsageFunction           queryUsageFunction
	tumblingWindowCronExpression *cronexpr.Expression
}

func NewPrometheusClient(address string, params *api.UsageParams) (api.Interface, error) {
	cfg := promapi.Config{
		Address: address,
	}

	client, err := promapi.NewClient(cfg)
	if err != nil {
		return nil, fmt.Errorf("error creating prometheus client: %v", err)
	}

	v1api := promv1.NewAPI(client)

	usageQueryTimeout := params.GetExtraDurationParamOrDefault("usageQueryTimeout", 10*time.Second)
	queryResolution := params.GetExtraDurationParamOrDefault("queryResolution", 1*time.Minute)

	allocationMetricsMap := map[string]string{
		"nvidia.com/gpu": params.GetExtraStringParamOrDefault("gpuAllocationMetric", "kai_queue_allocated_gpus"),
		"cpu":            params.GetExtraStringParamOrDefault("cpuAllocationMetric", "kai_queue_allocated_cpu_cores"),
		"memory":         params.GetExtraStringParamOrDefault("memoryAllocationMetric", "kai_queue_allocated_memory_bytes"),
	}

	clientObj := &PrometheusClient{
		client:      v1api,
		promClient:  client,
		usageParams: params,

		usageQueryTimeout:    usageQueryTimeout,
		queryResolution:      queryResolution,
		allocationMetricsMap: allocationMetricsMap,
	}

	if params.WindowType == nil {
		return nil, fmt.Errorf("window type is not set in usage params")
	}
	switch *params.WindowType {
	case api.TumblingWindow:
		clientObj.queryUsageFunction = clientObj.queryTumblingWindowUsage

		cronExpression, err := cronexpr.Parse(params.CronString)
		if err != nil {
			return nil, fmt.Errorf("error parsing cron string '%s' for usage tumbling window: %v", params.CronString, err)
		}
		clientObj.tumblingWindowCronExpression = cronExpression
	case api.SlidingWindow:
		clientObj.queryUsageFunction = clientObj.querySlidingWindowUsage
	}

	return clientObj, nil
}

func (p *PrometheusClient) GetResourceUsage() (*queue_info.ClusterUsage, error) {
	ctx, cancel := context.WithTimeout(context.Background(), p.usageQueryTimeout)
	defer cancel()

	usage := queue_info.NewClusterUsage()

	for _, resource := range []v1.ResourceName{commonconstants.GpuResource, v1.ResourceCPU, v1.ResourceMemory} {
		resourceUsage, err := p.queryUsageFunction(ctx, p.allocationMetricsMap[string(resource)])
		if err != nil {
			return nil, fmt.Errorf("error querying %s and usage: %v", resource, err)
		}
		for queueID, queueResourceUsage := range resourceUsage {
			if _, exists := usage.Queues[queueID]; !exists {
				usage.Queues[queueID] = queue_info.QueueUsage{}
			}
			usage.Queues[queueID][resource] = queueResourceUsage
		}
	}

	return usage, nil
}

func (p *PrometheusClient) querySlidingWindowUsage(ctx context.Context, allocationMetric string) (map[common_info.QueueID]float64, error) {
	queueUsage := make(map[common_info.QueueID]float64)

	decayedAllocationMetric := allocationMetric
	if p.usageParams.HalfLifePeriod != nil {
		decayedAllocationMetric = fmt.Sprintf("((%s) * (%s))", allocationMetric, getExponentialDecayQuery(p.usageParams.HalfLifePeriod))
	}

	usageQuery := fmt.Sprintf("sum_over_time((%s)[%s:%s])",
		decayedAllocationMetric,
		p.usageParams.WindowSize.String(),
		p.queryResolution.String(),
	)

	usageResult, warnings, err := p.client.Query(ctx, usageQuery, time.Now())
	if err != nil {
		return nil, fmt.Errorf("error running query %s: %v", usageQuery, err)
	}

	// Log warnings if exist
	for _, w := range warnings {
		log.InfraLogger.V(3).Warnf("Warning querying cluster usage metric %s: %s", decayedAllocationMetric, w)
	}

	if usageResult.Type() != model.ValVector {
		return nil, fmt.Errorf("unexpected query result: got %s, expected vector", usageResult.Type())
	}

	usageVector := usageResult.(model.Vector)
	if len(usageVector) == 0 {
		return nil, fmt.Errorf("no data returned for cluster usage metric %s", decayedAllocationMetric)
	}

	for _, usageSample := range usageVector {
		queueName := string(usageSample.Metric[queueNameLabel])
		value := float64(usageSample.Value)

		queueUsage[common_info.QueueID(queueName)] = value
	}

	return queueUsage, nil
}

func getExponentialDecayQuery(halfLifePeriod *time.Duration) string {
	if halfLifePeriod == nil {
		return ""
	}

	halfLifeSeconds := halfLifePeriod.Seconds()
	now := time.Now().Unix()

	return fmt.Sprintf("0.5^((%d - time()) / %f)", now, halfLifeSeconds)
}

func (p *PrometheusClient) queryTumblingWindowUsage(ctx context.Context, allocationMetric string) (map[common_info.QueueID]float64, error) {
	queuesUsage := make(map[common_info.QueueID]float64)

	usageQuery := fmt.Sprintf("(%s)[%s:%s]",
		allocationMetric,
		p.usageParams.WindowSize.String(),
		p.queryResolution.String(),
	)

	usageResult, warnings, err := p.client.Query(ctx, usageQuery, time.Now())
	if err != nil {
		return nil, fmt.Errorf("error running query %s: %v", usageQuery, err)
	}

	// Log warnings if exist
	for _, w := range warnings {
		log.InfraLogger.V(3).Warnf("Warning querying cluster usage metric %s: %s", allocationMetric, w)
	}

	if usageResult.Type() != model.ValMatrix {
		return nil, fmt.Errorf("unexpected query result: got %s, expected matrix", usageResult.Type())
	}

	usageMatrix := usageResult.(model.Matrix)
	if len(usageMatrix) == 0 {
		return nil, fmt.Errorf("no data returned for cluster usage metric %s", allocationMetric)
	}

	lastUsageReset := p.getLatestUsageResetTime()

	for _, usageSample := range usageMatrix {
		queueID := common_info.QueueID(usageSample.Metric[queueNameLabel])

		for _, usagePoint := range usageSample.Values {
			if usagePoint.Timestamp.Time().Before(lastUsageReset) {
				continue // Skip data before the last tumbling reset
			}
			queuesUsage[queueID] += float64(usagePoint.Value)
		}
	}

	return queuesUsage, nil
}

func (p *PrometheusClient) getLatestUsageResetTime() time.Time {
	maxWindowStartingPoint := time.Now().Add(-*p.usageParams.WindowSize)
	lastUsageReset := maxWindowStartingPoint
	nextInWindowReset := maxWindowStartingPoint

	for nextInWindowReset.Before(time.Now()) {
		lastUsageReset = nextInWindowReset
		nextInWindowReset = p.tumblingWindowCronExpression.Next(nextInWindowReset)
	}
	return lastUsageReset
}
