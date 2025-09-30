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

type usageWindowQueryFunction func(ctx context.Context, decayedAllocationMetric string) (model.Value, promv1.Warnings, error)

var _ api.Interface = &PrometheusClient{}

type PrometheusClient struct {
	client      promv1.API
	promClient  promapi.Client
	usageParams *api.UsageParams

	// Extra params
	usageQueryTimeout            time.Duration
	queryResolution              time.Duration
	allocationMetricsMap         map[string]string
	capacityMetricsMap           map[string]string
	usageWindowQuery             usageWindowQueryFunction
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

	capacityMetricsMap := map[string]string{
		"nvidia.com/gpu": params.GetExtraStringParamOrDefault("gpuCapacityMetric", "sum(kube_node_status_capacity{resource=\"nvidia_com_gpu\"})"),
		"cpu":            params.GetExtraStringParamOrDefault("cpuCapacityMetric", "sum(kube_node_status_capacity{resource=\"cpu\"})"),
		"memory":         params.GetExtraStringParamOrDefault("memoryCapacityMetric", "sum(kube_node_status_capacity{resource=\"memory\"})"),
	}

	clientObj := &PrometheusClient{
		client:      v1api,
		promClient:  client,
		usageParams: params,

		usageQueryTimeout:    usageQueryTimeout,
		queryResolution:      queryResolution,
		allocationMetricsMap: allocationMetricsMap,
		capacityMetricsMap:   capacityMetricsMap,
	}

	if params.WindowType == nil {
		return nil, fmt.Errorf("window type is not set in usage params")
	}
	switch *params.WindowType {
	case api.TumblingWindow:
		clientObj.usageWindowQuery = clientObj.queryTumblingTimeWindow

		cronExpression, err := cronexpr.Parse(params.TumblingWindowCronString)
		if err != nil {
			return nil, fmt.Errorf("error parsing cron string '%s' for usage tumbling window: %v", params.TumblingWindowCronString, err)
		}
		clientObj.tumblingWindowCronExpression = cronExpression
	case api.SlidingWindow:
		clientObj.usageWindowQuery = clientObj.querySlidingTimeWindow
	}

	return clientObj, nil
}

func (p *PrometheusClient) GetResourceUsage() (*queue_info.ClusterUsage, error) {
	ctx, cancel := context.WithTimeout(context.Background(), p.usageQueryTimeout)
	defer cancel()

	capacity := map[v1.ResourceName]float64{}
	for _, resource := range []v1.ResourceName{commonconstants.GpuResource, v1.ResourceCPU, v1.ResourceMemory} {
		resourceCapacity, err := p.queryResourceCapacity(ctx, p.capacityMetricsMap[string(resource)], p.usageWindowQuery)
		if err != nil {
			return nil, fmt.Errorf("error querying %s and capacity: %v", resource, err)
		}
		capacity[resource] = resourceCapacity
	}

	usage := queue_info.NewClusterUsage()

	for _, resource := range []v1.ResourceName{commonconstants.GpuResource, v1.ResourceCPU, v1.ResourceMemory} {
		capacityForResource, found := capacity[resource]
		if !found {
			capacityForResource = 1
			log.InfraLogger.V(3).Warnf("Capacity for %s not found, setting to 1", resource)
		}

		resourceUsage, err := p.queryResourceUsage(ctx, p.allocationMetricsMap[string(resource)], p.usageWindowQuery)
		if err != nil {
			return nil, fmt.Errorf("error querying %s and usage: %v", resource, err)
		}
		for queueID, queueResourceUsage := range resourceUsage {
			if _, exists := usage.Queues[queueID]; !exists {
				usage.Queues[queueID] = queue_info.QueueUsage{}
			}
			usage.Queues[queueID][resource] = queueResourceUsage / capacityForResource
		}
	}

	return usage, nil
}

func (p *PrometheusClient) queryResourceCapacity(ctx context.Context, capacityMetric string, queryByWindow usageWindowQueryFunction) (float64, error) {
	decayedCapacityMetric := capacityMetric
	if p.usageParams.HalfLifePeriod != nil {
		decayedCapacityMetric = fmt.Sprintf("((%s) * (%s))", capacityMetric, getExponentialDecayQuery(p.usageParams.HalfLifePeriod))
	}

	capacityResult, warnings, err := queryByWindow(ctx, decayedCapacityMetric)
	if err != nil {
		return 0, fmt.Errorf("error querying cluster capacity metric %s: %v", decayedCapacityMetric, err)
	}

	// Log warnings if exist
	for _, w := range warnings {
		log.InfraLogger.V(3).Warnf("Warning querying cluster capacity metric %s: %s", decayedCapacityMetric, w)
	}

	if capacityResult.Type() != model.ValVector {
		return 0, fmt.Errorf("unexpected query result: got %s, expected vector", capacityResult.Type())
	}

	capacityVector := capacityResult.(model.Vector)
	if len(capacityVector) == 0 {
		return 0, fmt.Errorf("no data returned for cluster capacity metric %s", decayedCapacityMetric)
	}

	return float64(capacityVector[0].Value), nil
}

func (p *PrometheusClient) queryResourceUsage(
	ctx context.Context, allocationMetric string, queryByWindow usageWindowQueryFunction) (map[common_info.QueueID]float64, error) {
	queueUsage := make(map[common_info.QueueID]float64)

	decayedAllocationMetric := allocationMetric
	if p.usageParams.HalfLifePeriod != nil {
		decayedAllocationMetric = fmt.Sprintf("((%s) * (%s))", allocationMetric, getExponentialDecayQuery(p.usageParams.HalfLifePeriod))
	}

	usageResult, warnings, err := queryByWindow(ctx, decayedAllocationMetric)
	if err != nil {
		return nil, fmt.Errorf("error querying cluster usage metric %s: %v", decayedAllocationMetric, err)
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

func (p *PrometheusClient) querySlidingTimeWindow(ctx context.Context, decayedAllocationMetric string) (model.Value, promv1.Warnings, error) {
	usageQuery := fmt.Sprintf("sum_over_time((%s)[%s:%s])",
		decayedAllocationMetric,
		p.usageParams.WindowSize.String(),
		p.queryResolution.String(),
	)

	usageResult, warnings, err := p.client.Query(ctx, usageQuery, time.Now())
	return usageResult, warnings, err
}

func (p *PrometheusClient) queryTumblingTimeWindow(ctx context.Context, decayedAllocationMetric string) (model.Value, promv1.Warnings, error) {
	usageQuery := fmt.Sprintf("sum_over_time(%s)", decayedAllocationMetric)
	lastUsageReset := p.getLatestUsageResetTime()

	usageResult, warnings, err := p.client.QueryRange(ctx, usageQuery, promv1.Range{
		Start: lastUsageReset,
		End:   time.Now(),
		Step:  p.queryResolution,
	})
	return usageResult, warnings, err
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

func getExponentialDecayQuery(halfLifePeriod *time.Duration) string {
	if halfLifePeriod == nil {
		return ""
	}

	halfLifeSeconds := halfLifePeriod.Seconds()
	now := time.Now().Unix()

	return fmt.Sprintf("0.5^((%d - time()) / %f)", now, halfLifeSeconds)
}
