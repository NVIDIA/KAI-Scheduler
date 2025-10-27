// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package fake

import (
	"math"
	"sync"

	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/common_info"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/queue_info"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/cache/usagedb/api"
	"github.com/go-gota/gota/dataframe"
	"github.com/go-gota/gota/series"
	v1 "k8s.io/api/core/v1"
)

type FakeUsageDBClient struct {
	resourceUsage      *queue_info.ClusterUsage
	resourceUsageMutex sync.RWMutex

	usageParams *api.UsageParams
	// each entry is an allocation per second
	allocationHistory      AllocationHistory
	clusterCapacityHistory ClusterCapacityHistory
}

var _ api.Interface = &FakeUsageDBClient{}
var ClientWithHistory *FakeUsageDBClient

func NewFakeWithHistoryClient(_ string, usageParams *api.UsageParams) (api.Interface, error) {
	if ClientWithHistory == nil {
		ClientWithHistory = &FakeUsageDBClient{
			resourceUsage: queue_info.NewClusterUsage(),
			usageParams:   usageParams,
		}
	}

	return ClientWithHistory, nil
}

func (f *FakeUsageDBClient) ResetClient() {
	ClientWithHistory = nil
}

func (f *FakeUsageDBClient) GetResourceUsage() (*queue_info.ClusterUsage, error) {
	f.resourceUsageMutex.RLock()
	defer f.resourceUsageMutex.RUnlock()

	usage := queue_info.NewClusterUsage()

	var windowStart, windowEnd int
	size := f.usageParams.WindowSize.Seconds()
	if len(f.allocationHistory) <= int(size) {
		windowStart = 0
		windowEnd = len(f.allocationHistory)
	} else {
		windowStart = len(f.allocationHistory) - int(size)
		windowEnd = len(f.allocationHistory)
	}

	totalDecayFactor := 0.0
	var decayFactors []float64
	for i := range windowEnd - windowStart {
		decayFactors = append(decayFactors, math.Pow(0.5, float64(size-float64(i))))
		totalDecayFactor += decayFactors[i]
	}
	for i, decayFactor := range decayFactors {
		decayFactors[i] = decayFactor / totalDecayFactor
	}

	for i, queueAllocations := range f.allocationHistory[windowStart:windowEnd] {
		timeDecayFactor := math.Pow(0.5, float64(size-float64(i)))
		for queueID, allocation := range queueAllocations {
			if _, exists := usage.Queues[queueID]; !exists {
				usage.Queues[queueID] = queue_info.QueueUsage{}
			}
			for resource, allocation := range allocation {
				if _, exists := usage.Queues[queueID][resource]; !exists {
					usage.Queues[queueID][resource] = 0
				}
				usage.Queues[queueID][resource] += ((allocation * timeDecayFactor) / f.clusterCapacityHistory[windowStart:windowEnd][i][resource])
			}
		}
	}

	return usage, nil
}

func (f *FakeUsageDBClient) AppendQueuedAllocation(queueAllocations map[common_info.QueueID]queue_info.QueueUsage, totalInCluster map[v1.ResourceName]float64) {
	f.resourceUsageMutex.Lock()
	defer f.resourceUsageMutex.Unlock()

	f.allocationHistory = append(f.allocationHistory, queueAllocations)
	f.clusterCapacityHistory = append(f.clusterCapacityHistory, totalInCluster)
}

func (f *FakeUsageDBClient) GetAllocationHistory() AllocationHistory {
	return f.allocationHistory
}

type AllocationHistory []map[common_info.QueueID]queue_info.QueueUsage
type ClusterCapacityHistory []map[v1.ResourceName]float64

func (a AllocationHistory) ToDataFrame() dataframe.DataFrame {
	var times []int
	var queueIDs []string
	var resources []string
	var allocations []float64

	for timeIndex, queueAllocations := range a {
		for queueID, queueUsage := range queueAllocations {
			for resourceName, allocation := range queueUsage {
				times = append(times, timeIndex)
				queueIDs = append(queueIDs, string(queueID))
				resources = append(resources, string(resourceName))
				allocations = append(allocations, allocation)
			}
		}
	}

	df := dataframe.New(
		series.New(times, series.Int, "Time"),
		series.New(queueIDs, series.String, "QueueID"),
		series.New(resources, series.String, "Resource"),
		series.New(allocations, series.Float, "Allocation"),
	)

	return df
}
