// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package prometheus

import (
	"context"
	"fmt"
	"time"

	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/common_info"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/queue_info"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/cache/usagedb/api"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/log"
	promapi "github.com/prometheus/client_golang/api"
	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
)

var _ api.Interface = &PrometheusClient{}

type PrometheusClient struct {
	client      v1.API
	promClient  promapi.Client
	usageParams *api.UsageParams
}

func NewPrometheusClient(address string, params *api.UsageParams) (api.Interface, error) {
	cfg := promapi.Config{
		Address: address,
	}

	client, err := promapi.NewClient(cfg)
	if err != nil {
		return nil, fmt.Errorf("error creating prometheus client: %v", err)
	}

	v1api := v1.NewAPI(client)

	if params.WindowType != nil && *params.WindowType == api.TumblingWindow {
		log.InfraLogger.V(3).Warnf("Tumbling window is not supported for prometheus client, using sliding window instead")
		windowType := api.SlidingWindow
		params.WindowType = &windowType
	}

	return &PrometheusClient{
		client:      v1api,
		promClient:  client,
		usageParams: params,
	}, nil
}

func (p *PrometheusClient) GetResourceUsage() (*queue_info.ClusterUsage, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	usage := queue_info.NewClusterUsage()

	// get gpu capacity and usage
	gpuUsage, err := p.queryResourceUsage(ctx, "gpu")
	if err != nil {
		return nil, fmt.Errorf("error querying gpu capacity and usage: %v", err)
	}
	for queueID, queueGPUUsage := range gpuUsage {
		if _, exists := usage.Queues[queueID]; !exists {
			usage.Queues[queueID] = &queue_info.QueueUsage{}
		}
		usage.Queues[queueID].GPU = queueGPUUsage
	}

	cpuUsage, err := p.queryResourceUsage(ctx, "cpu")
	if err != nil {
		return nil, fmt.Errorf("error querying cpu capacity and usage: %v", err)
	}
	for queueID, queueCPUUsage := range cpuUsage {
		if _, exists := usage.Queues[queueID]; !exists {
			usage.Queues[queueID] = &queue_info.QueueUsage{}
		}
		usage.Queues[queueID].CPU = queueCPUUsage
	}

	memoryUsage, err := p.queryResourceUsage(ctx, "memory")
	if err != nil {
		return nil, fmt.Errorf("error querying memory capacity and usage: %v", err)
	}
	for queueID, queueMemoryUsage := range memoryUsage {
		if _, exists := usage.Queues[queueID]; !exists {
			usage.Queues[queueID] = &queue_info.QueueUsage{}
		}
		usage.Queues[queueID].Memory = queueMemoryUsage
	}

	return usage, nil
}

func (p *PrometheusClient) queryResourceUsage(ctx context.Context, resource string) (map[common_info.QueueID]float64, error) {
	expressionMap := map[string]string{
		"gpu":    "kai_queue_allocated_gpus",
		"cpu":    "kai_queue_allocated_cpu_cores",
		"memory": "kai_queue_allocated_memory_bytes",
	}

	queueUsage := make(map[common_info.QueueID]float64)

	usageQuery := fmt.Sprintf("sum_over_time(%s[%s:%s])",
		expressionMap[resource],
		p.usageParams.WindowSize.String(),
		"1m", // ToDo: make resolution configurable
	)

	usageResult, warnings, err := p.client.Query(ctx, usageQuery, time.Now())
	if err != nil {
		return nil, fmt.Errorf("error querying cluster %s usage: %v", resource, err)
	}

	// Log warnings if exist
	for _, w := range warnings {
		log.InfraLogger.V(3).Warnf("Warning querying cluster %s usage: %s", resource, w)
	}

	if usageResult.Type() != model.ValVector {
		return nil, fmt.Errorf("unexpected query result: got %s, expected vector", usageResult.Type())
	}

	usageVector := usageResult.(model.Vector)
	if len(usageVector) == 0 {
		return nil, fmt.Errorf("no data returned for cluster %s usage", resource)
	}

	for _, usageSample := range usageVector {
		queueName := string(usageSample.Metric["queue_name"])
		value := float64(usageSample.Value)

		queueUsage[common_info.QueueID(queueName)] = value
	}

	return queueUsage, nil
}
