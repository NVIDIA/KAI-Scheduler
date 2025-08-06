// Copyright 2023 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"os"
	"testing"

	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/common_info"
	rs "github.com/NVIDIA/KAI-scheduler/pkg/scheduler/plugins/proportion/resource_share"
	"knative.dev/pkg/ptr"
)

func TestSimulateSetResourcesShare(t *testing.T) {
	totalResource := rs.ResourceQuantities{
		"GPU": 100,
	}

	totalUsage := map[rs.ResourceName]float64{
		"GPU": 100,
	}

	k := float64(1000)

	getQueues := func() []rs.QueueOverrides {
		return []rs.QueueOverrides{
			{
				UID:      "queue1",
				Name:     "test-queue",
				Priority: ptr.Int32(0),
				ResourceShare: rs.QueueResourceShareOverrides{
					GPU: &rs.ResourceShareOverrides{
						Deserved:        ptr.Float64(0),
						Request:         ptr.Float64(100),
						OverQuotaWeight: ptr.Float64(3),
					},
				},
			},
			{
				UID:      "queue2",
				Name:     "test-queue2",
				Priority: ptr.Int32(0),
				ResourceShare: rs.QueueResourceShareOverrides{
					GPU: &rs.ResourceShareOverrides{
						Deserved:        ptr.Float64(0),
						Request:         ptr.Float64(100),
						OverQuotaWeight: ptr.Float64(1),
					},
				},
			},
		}
	}

	// Create CSV file
	f, err := os.Create("fairshare_simulation.csv")
	if err != nil {
		t.Fatalf("Failed to create CSV file: %v", err)
	}
	defer f.Close()

	// Write CSV header
	_, err = f.WriteString("iteration,queue1_gpu_fair_share,queue2_gpu_fair_share\n")
	if err != nil {
		t.Fatalf("Failed to write CSV header: %v", err)
	}

	// Run simulation and write results
	for i := range 120 {
		queues := getQueues()
		queues[0].ResourceShare.GPU.AbsoluteUsage = ptr.Float64(float64(i))

		result := SimulateSetResourcesShare(totalResource, totalUsage, k, queues)

		// Write results to CSV
		_, err := f.WriteString(fmt.Sprintf("%d,%f,%f\n",
			i,
			result[common_info.QueueID("queue1")].GPU.FairShare,
			result[common_info.QueueID("queue2")].GPU.FairShare))
		if err != nil {
			t.Fatalf("Failed to write to CSV at iteration %d: %v", i, err)
		}
	}
}
