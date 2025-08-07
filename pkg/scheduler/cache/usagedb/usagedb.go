// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package usagedb

import (
	"fmt"
	"sync"
	"time"

	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/queue_info"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/log"
)

var defaultFetchInterval = 1 * time.Minute

type Interface interface {
	GetResourceUsage() (*queue_info.ClusterUsage, error)
}

type UsageLister struct {
	client             Interface
	lastUsageData      *queue_info.ClusterUsage
	lastUsageDataMutex sync.RWMutex
	lastUsageDataTime  *time.Time
	fetchInterval      time.Duration
	stalenessPeriod    time.Duration
}

func NewUsageLister(client Interface, fetchInterval, stalenessPeriod *time.Duration) *UsageLister {
	if fetchInterval == nil {
		log.InfraLogger.V(3).Infof("fetchInterval is not set, using default: %s", defaultFetchInterval)
		fetchInterval = &defaultFetchInterval
	}

	if stalenessPeriod == nil {
		period := 5 * defaultFetchInterval
		stalenessPeriod = &period
		log.InfraLogger.V(3).Infof("stalenessPeriod is not set, using default: %s", period)
	}

	if stalenessPeriod.Seconds() < fetchInterval.Seconds() {
		log.InfraLogger.V(2).Warnf("stalenessPeriod is less than fetchInterval, using stalenessPeriod: %s", stalenessPeriod)
		stalenessPeriod = fetchInterval
	}

	return &UsageLister{
		client:          client,
		lastUsageData:   queue_info.NewClusterUsage(),
		fetchInterval:   *fetchInterval,
		stalenessPeriod: *stalenessPeriod,
	}
}

// GetResourceUsage returns the last known resource usage data.
// If the data is stale, an error is returned, but the most recent data is still returned.
func (l *UsageLister) GetResourceUsage() (*queue_info.ClusterUsage, error) {
	l.lastUsageDataMutex.RLock()
	defer l.lastUsageDataMutex.RUnlock()

	if l.lastUsageDataTime == nil {
		return nil, fmt.Errorf("usage data is not available")
	}

	var err error
	if time.Since(*l.lastUsageDataTime) > l.stalenessPeriod {
		err = fmt.Errorf("usage data is stale, last update: %s, staleness period: %s, time since last update: %s", l.lastUsageDataTime, l.stalenessPeriod, time.Since(*l.lastUsageDataTime))
	}

	return l.lastUsageData, err
}

// Start begins periodic fetching of resource usage data in a background goroutine.
// The data is fetched every minute by default.
func (l *UsageLister) Start(stopCh <-chan struct{}) {
	go func() {
		ticker := time.NewTicker(l.fetchInterval)
		defer ticker.Stop()

		// Fetch immediately on start
		if err := l.fetchAndUpdateUsage(); err != nil {
			// Log error but continue - we'll retry on next tick
			// TODO: Add proper logging
		}

		for {
			select {
			case <-ticker.C:
				if err := l.fetchAndUpdateUsage(); err != nil {
					// Log error but continue - we'll retry on next tick
					// TODO: Add proper logging
				}
			case <-stopCh:
				return
			}
		}
	}()
}

func (l *UsageLister) WaitForCacheSync(stopCh <-chan struct{}) bool {
	// Check every 10ms for data or stop signal
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-stopCh:
			return false
		case <-ticker.C:
			l.lastUsageDataMutex.RLock()
			hasData := l.lastUsageData != nil
			l.lastUsageDataMutex.RUnlock()

			if hasData {
				return true
			}
		}
	}
}

func (l *UsageLister) fetchAndUpdateUsage() error {
	if l.client == nil {
		return fmt.Errorf("failed to fetch usage data: client is not set")
	}

	// TODO: Add metrics for fetch times
	usage, err := l.client.GetResourceUsage()
	if err != nil {
		return err
	}

	now := time.Now()

	l.lastUsageDataMutex.Lock()
	defer l.lastUsageDataMutex.Unlock()

	l.lastUsageData = usage
	l.lastUsageDataTime = &now
	return nil
}
