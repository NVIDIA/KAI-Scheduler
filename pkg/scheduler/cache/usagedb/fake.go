// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package usagedb

import (
	"sync"

	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/queue_info"
)

var _ Interface = &FakeClient{}

type FakeClient struct {
	resourceUsage      *queue_info.ClusterUsage
	resourceUsageMutex sync.RWMutex
	resourceUsageErr   error
}

func (f *FakeClient) GetResourceUsage() (*queue_info.ClusterUsage, error) {
	f.resourceUsageMutex.RLock()
	defer f.resourceUsageMutex.RUnlock()

	return f.resourceUsage, f.resourceUsageErr
}

func (f *FakeClient) SetResourceUsage(resourceUsage *queue_info.ClusterUsage, err error) {
	f.resourceUsageMutex.Lock()
	defer f.resourceUsageMutex.Unlock()

	f.resourceUsage = resourceUsage
	f.resourceUsageErr = err
}
