// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package fake

import (
	"fmt"
	"maps"
	"math"
	"slices"
	"strings"
	"sync"

	"github.com/NVIDIA/KAI-scheduler/pkg/common/constants"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/common_info"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/queue_info"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/cache/usagedb/api"
	v1 "k8s.io/api/core/v1"
)

type FakeWithHistoryClient struct {
	resourceUsage      *queue_info.ClusterUsage
	resourceUsageMutex sync.RWMutex
	resourceUsageErr   error

	usageParams *api.UsageParams
	// each entry is an allocation per second
	allocationHistory      AllocationHistory
	clusterCapacityHistory ClusterCapacityHistory
}

var _ api.Interface = &FakeWithHistoryClient{}
var ClientWithHistory *FakeWithHistoryClient

func NewFakeWithHistoryClient(_ string, usageParams *api.UsageParams) (api.Interface, error) {
	if ClientWithHistory == nil {
		ClientWithHistory = &FakeWithHistoryClient{
			resourceUsage: queue_info.NewClusterUsage(),
			usageParams:   usageParams,
		}
	}

	return ClientWithHistory, nil
}

func (f *FakeWithHistoryClient) ResetClient() {
	ClientWithHistory = nil
}

func (f *FakeWithHistoryClient) GetResourceUsage() (*queue_info.ClusterUsage, error) {
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

func (f *FakeWithHistoryClient) AppendQueuedAllocation(queueAllocations map[common_info.QueueID]queue_info.QueueUsage, totalInCluster map[v1.ResourceName]float64) {
	f.resourceUsageMutex.Lock()
	defer f.resourceUsageMutex.Unlock()

	f.allocationHistory = append(f.allocationHistory, queueAllocations)
	f.clusterCapacityHistory = append(f.clusterCapacityHistory, totalInCluster)
}

func (f *FakeWithHistoryClient) GetAllocationHistory() AllocationHistory {
	return f.allocationHistory
}

type AllocationHistory []map[common_info.QueueID]queue_info.QueueUsage
type ClusterCapacityHistory []map[v1.ResourceName]float64

func (a AllocationHistory) ToTsv() string {
	allQueues := make(map[string]bool)
	for _, allocation := range a {
		for queueID := range allocation {
			allQueues[string(queueID)] = true
		}
	}
	sortedQueues := slices.Collect(maps.Keys(allQueues))
	slices.Sort(sortedQueues)
	csv := "t\t"
	csv += strings.Join(sortedQueues, "\t")
	csv += "\n"

	for t, allocation := range a {
		csv += fmt.Sprintf("%d\t", t)
		for _, queueID := range sortedQueues {
			allocation, exists := allocation[common_info.QueueID(queueID)]
			if !exists {
				allocation = queue_info.QueueUsage{}
			}
			csv += fmt.Sprintf("%f\t", allocation[constants.GpuResource])
		}
		csv += "\n"
	}
	return csv
}
