// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package usagedb

import (
	"testing"
	"time"

	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/queue_info"
	usagedbapi "github.com/NVIDIA/KAI-scheduler/pkg/scheduler/cache/usagedb/api"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/cache/usagedb/fake"
	"github.com/stretchr/testify/assert"
)

func TestNewUsageLister(t *testing.T) {
	tests := []struct {
		name            string
		fetchInterval   *time.Duration
		stalenessPeriod *time.Duration
		wantInterval    time.Duration
		wantStaleness   time.Duration
	}{
		{
			name:          "default values",
			wantInterval:  defaultFetchInterval,
			wantStaleness: 5 * defaultFetchInterval,
		},
		{
			name: "custom fetch interval",
			fetchInterval: func() *time.Duration {
				d := 30 * time.Second
				return &d
			}(),
			wantInterval:  30 * time.Second,
			wantStaleness: 5 * defaultFetchInterval,
		},
		{
			name: "custom staleness period",
			stalenessPeriod: func() *time.Duration {
				d := 10 * time.Minute
				return &d
			}(),
			wantInterval:  defaultFetchInterval,
			wantStaleness: 10 * time.Minute,
		},
		{
			name: "staleness less than fetch interval",
			fetchInterval: func() *time.Duration {
				d := 2 * time.Minute
				return &d
			}(),
			stalenessPeriod: func() *time.Duration {
				d := 1 * time.Minute
				return &d
			}(),
			wantInterval:  2 * time.Minute,
			wantStaleness: 2 * time.Minute, // Should be adjusted to match fetch interval
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lister := NewUsageLister(&fake.FakeClient{}, tt.fetchInterval, tt.stalenessPeriod, nil)
			assert.Equal(t, tt.wantInterval, lister.fetchInterval)
			assert.Equal(t, tt.wantStaleness, lister.stalenessPeriod)
			assert.NotNil(t, lister.lastUsageData)
			assert.Nil(t, lister.lastUsageDataTime)
		})
	}
}

func TestGetResourceUsage(t *testing.T) {
	tests := []struct {
		name        string
		setupLister func(*UsageLister)
		wantUsage   *queue_info.ClusterUsage
		wantErr     bool
	}{
		{
			name: "no data available",
			setupLister: func(l *UsageLister) {
				// Do nothing - simulate fresh lister
			},
			wantErr: true,
		},
		{
			name: "fresh data available",
			setupLister: func(l *UsageLister) {
				usage := queue_info.NewClusterUsage()
				usage.ClusterCapacity.GPU = 10
				usage.Queues["queue1"] = &queue_info.QueueUsage{GPU: 5}
				now := time.Now()
				l.lastUsageData = usage
				l.lastUsageDataTime = &now
			},
			wantUsage: func() *queue_info.ClusterUsage {
				usage := queue_info.NewClusterUsage()
				usage.ClusterCapacity.GPU = 10
				usage.Queues["queue1"] = &queue_info.QueueUsage{GPU: 5}
				return usage
			}(),
		},
		{
			name: "stale data",
			setupLister: func(l *UsageLister) {
				usage := queue_info.NewClusterUsage()
				usage.ClusterCapacity.GPU = 10
				staleTime := time.Now().Add(-10 * time.Minute)
				l.lastUsageData = usage
				l.lastUsageDataTime = &staleTime
			},
			wantUsage: func() *queue_info.ClusterUsage {
				usage := queue_info.NewClusterUsage()
				usage.ClusterCapacity.GPU = 10
				return usage
			}(),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lister := NewUsageLister(&fake.FakeClient{}, nil, nil, nil)
			if tt.setupLister != nil {
				tt.setupLister(lister)
			}

			got, err := lister.GetResourceUsage()
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			if tt.wantUsage != nil {
				assert.Equal(t, tt.wantUsage, got)
			}
		})
	}
}

func TestGetClient(t *testing.T) {
	tests := []struct {
		name   string
		config *usagedbapi.UsageDBConfig

		wantError bool
		wantNil   bool
	}{
		{
			name:    "nil config",
			config:  nil,
			wantNil: true,
		},
		{
			name: "fake client",
			config: &usagedbapi.UsageDBConfig{
				ClientType:       "fake",
				ConnectionString: "fake-connection",
			},
		},
		{
			name: "unknown client type",
			config: &usagedbapi.UsageDBConfig{
				ClientType:       "unknown",
				ConnectionString: "test-connection",
			},
			wantError: true,
		},
		{
			name: "empty client type",
			config: &usagedbapi.UsageDBConfig{
				ClientType:       "",
				ConnectionString: "test-connection",
			},
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := GetClient(tt.config)

			if tt.wantError {
				assert.Error(t, err)
				assert.Nil(t, client)
				if tt.config != nil {
					if tt.config.ClientType == "" {
						assert.Contains(t, err.Error(), "client type cannot be empty")
					} else {
						assert.Contains(t, err.Error(), "unknown client type")
						assert.Contains(t, err.Error(), tt.config.ClientType)
					}
				}
				return
			}

			if tt.wantNil {
				assert.NoError(t, err)
				assert.Nil(t, client)
				return
			}

			assert.NoError(t, err)
			assert.NotNil(t, client)
		})
	}
}
