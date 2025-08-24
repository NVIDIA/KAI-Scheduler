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
	gpuCapacity, gpuUsage, err := p.queryResourceUsage(ctx, "gpu")
	if err != nil {
		return nil, fmt.Errorf("error querying gpu capacity and usage: %v", err)
	}
	usage.ClusterCapacity.GPU = gpuCapacity
	for queueID, queueGPUUsage := range gpuUsage {
		if _, exists := usage.Queues[queueID]; !exists {
			usage.Queues[queueID] = &queue_info.QueueUsage{}
		}
		usage.Queues[queueID].GPU = queueGPUUsage
	}

	cpuCapacity, cpuUsage, err := p.queryResourceUsage(ctx, "cpu")
	if err != nil {
		return nil, fmt.Errorf("error querying cpu capacity and usage: %v", err)
	}
	usage.ClusterCapacity.CPU = cpuCapacity
	for queueID, queueCPUUsage := range cpuUsage {
		if _, exists := usage.Queues[queueID]; !exists {
			usage.Queues[queueID] = &queue_info.QueueUsage{}
		}
		usage.Queues[queueID].CPU = queueCPUUsage
	}

	memoryCapacity, memoryUsage, err := p.queryResourceUsage(ctx, "memory")
	if err != nil {
		return nil, fmt.Errorf("error querying memory capacity and usage: %v", err)
	}
	usage.ClusterCapacity.Memory = memoryCapacity
	for queueID, queueMemoryUsage := range memoryUsage {
		if _, exists := usage.Queues[queueID]; !exists {
			usage.Queues[queueID] = &queue_info.QueueUsage{}
		}
		usage.Queues[queueID].Memory = queueMemoryUsage
	}

	return usage, nil
}

func (p *PrometheusClient) queryResourceUsage(ctx context.Context, resource string) (float64, map[common_info.QueueID]float64, error) {
	expressionMapUsage := map[string]string{
		"gpu":    "kai_queue_allocated_gpus",
		"cpu":    "kai_queue_allocated_cpu_cores",
		"memory": "kai_queue_allocated_memory_bytes",
	}
	expressionMapCapacity := map[string]string{
		"gpu":    "count(DCGM_FI_DEV_GPU_UTIL)",
		"cpu":    "sum(kube_node_status_capacity{resource=\"cpu\"})",
		"memory": "sum(kube_node_status_capacity{resource=\"memory\"})",
	}

	// Get queue GPU usage over time
	capacityQuery := fmt.Sprintf("sum_over_time(%s[%s:%s])",
		expressionMapCapacity[resource],
		p.usageParams.WindowSize.String(),
		"1m", // ToDo: make resolution configurable
	)

	result, warnings, err := p.client.Query(ctx, capacityQuery, time.Now())
	if err != nil {
		return 0, nil, fmt.Errorf("error querying cluster %s capacity: %v", resource, err)
	}

	// Log warnings if exist
	for _, w := range warnings {
		log.InfraLogger.V(3).Warnf("Warning querying cluster %s capacity: %s", resource, w)
	}

	var capacity float64
	if result.Type() != model.ValVector {
		return 0, nil, fmt.Errorf("unexpected query result: got %s, expected vector", result.Type())
	}
	vector := result.(model.Vector)
	if len(vector) == 0 {
		return 0, nil, fmt.Errorf("no data returned for cluster %s capacity", resource)
	}

	capacity = float64(vector[0].Value)

	queueUsage := make(map[common_info.QueueID]float64)

	usageQuery := fmt.Sprintf("sum_over_time(%s[%s:%s])",
		expressionMapUsage[resource],
		p.usageParams.WindowSize.String(),
		"1m", // ToDo: make resolution configurable
	)

	usageResult, warnings, err := p.client.Query(ctx, usageQuery, time.Now())
	if err != nil {
		return 0, nil, fmt.Errorf("error querying cluster %s usage: %v", resource, err)
	}

	// Log warnings if exist
	for _, w := range warnings {
		log.InfraLogger.V(3).Warnf("Warning querying cluster %s usage: %s", resource, w)
	}

	if usageResult.Type() != model.ValVector {
		return 0, nil, fmt.Errorf("unexpected query result: got %s, expected vector", usageResult.Type())
	}

	usageVector := usageResult.(model.Vector)
	if len(usageVector) == 0 {
		return 0, nil, fmt.Errorf("no data returned for cluster %s usage", resource)
	}

	for _, usageSample := range usageVector {
		queueName := string(usageSample.Metric["queue_name"])
		value := float64(usageSample.Value)

		queueUsage[common_info.QueueID(queueName)] = value
	}

	return capacity, queueUsage, nil
}
