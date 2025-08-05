// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package usagedb

import (
	"fmt"
	"sync"
	"time"

	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/common_info"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/queue_info"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/log"
)

var defaultFetchInterval = 1 * time.Minute

type Interface interface {
	GetResourceUsage() (map[common_info.QueueID]*queue_info.QueueUsage, error)
}

type UsageLister struct {
	client             Interface
	lastUsageData      map[common_info.QueueID]*queue_info.QueueUsage
	lastUsageDataMutex sync.RWMutex
	lastUsageDataTime  *time.Time
	stopCh             <-chan struct{}
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

	return &UsageLister{
		client:          client,
		lastUsageData:   make(map[common_info.QueueID]*queue_info.QueueUsage),
		fetchInterval:   *fetchInterval,
		stalenessPeriod: *stalenessPeriod,
	}
}

// GetResourceUsage returns the last known resource usage data.
// If the data is stale, an error is returned, but the most recent data is still returned.
func (l *UsageLister) GetResourceUsage() (map[common_info.QueueID]*queue_info.QueueUsage, error) {
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
	l.stopCh = stopCh
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
			case <-l.stopCh:
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
			hasData := len(l.lastUsageData) > 0
			l.lastUsageDataMutex.RUnlock()

			if hasData {
				return true
			}
		}
	}
}

func (l *UsageLister) fetchAndUpdateUsage() error {
	// TODO: Add metrics
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
