// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package usagedb

import (
	"context"
	"fmt"
	"time"

	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/common_info"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/queue_info"
	"github.com/prometheus/client_golang/api"
	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
)

var _ Interface = &PrometheusClient{}

type PrometheusClient struct {
	client     v1.API
	promClient api.Client
}

func NewPrometheusClient(address string) (*PrometheusClient, error) {
	cfg := api.Config{
		Address: address,
	}

	client, err := api.NewClient(cfg)
	if err != nil {
		return nil, fmt.Errorf("error creating prometheus client: %v", err)
	}

	v1api := v1.NewAPI(client)

	return &PrometheusClient{
		client:     v1api,
		promClient: client,
	}, nil
}

func (p *PrometheusClient) GetResourceUsage() (*queue_info.ClusterUsage, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	usage := queue_info.NewClusterUsage()

	// Get cluster GPU capacity over time
	gpuCapacityResult, warnings, err := p.client.Query(ctx, "sum_over_time(count(DCGM_FI_DEV_GPU_UTIL)[1h:1m])", time.Now())
	if err != nil {
		return nil, fmt.Errorf("error querying cluster GPU capacity: %v", err)
	}
	if len(warnings) > 0 {
		// Log warnings but continue
		for _, w := range warnings {
			fmt.Printf("Warning querying cluster GPU capacity: %s\n", w)
		}
	}

	if gpuCapacityResult.Type() == model.ValVector {
		vector := gpuCapacityResult.(model.Vector)
		if len(vector) > 0 {
			usage.Cluster.GPU = float64(vector[0].Value)
		}
	}

	// Get queue GPU usage over time
	result, warnings, err := p.client.Query(ctx, "sum_over_time(kai_queue_allocated_gpus[1h:1m])", time.Now())
	if err != nil {
		return nil, fmt.Errorf("error querying queue GPU usage: %v", err)
	}
	if len(warnings) > 0 {
		// Log warnings but continue
		for _, w := range warnings {
			fmt.Printf("Warning querying queue GPU usage: %s\n", w)
		}
	}

	if result.Type() == model.ValVector {
		vector := result.(model.Vector)
		for _, sample := range vector {
			queueName := string(sample.Metric["queue_name"])
			value := float64(sample.Value)

			if _, exists := usage.Queues[common_info.QueueID(queueName)]; !exists {
				usage.Queues[common_info.QueueID(queueName)] = &queue_info.QueueUsage{}
			}

			usage.Queues[common_info.QueueID(queueName)].GPU = value
			usage.Cluster.GPU += value
		}
	}

	return usage, nil
}
