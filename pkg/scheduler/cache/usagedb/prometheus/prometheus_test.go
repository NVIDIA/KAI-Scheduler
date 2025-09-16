// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package prometheus

import (
	"testing"

	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/cache/usagedb/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewPrometheusClient(t *testing.T) {
	tests := []struct {
		name          string
		address       string
		params        *api.UsageParams
		expectError   bool
		errorContains string
	}{
		{
			name:          "valid address",
			address:       "http://localhost:9090",
			params:        nil,
			expectError:   false,
			errorContains: "",
		},
		{
			name:          "invalid address",
			address:       "://invalid:9090",
			params:        nil,
			expectError:   true,
			errorContains: "error creating prometheus client",
		},
		{
			name:    "invalid cron string - for tumbling window",
			address: "http://localhost:9090",
			params: &api.UsageParams{
				WindowType:               &[]api.WindowType{api.TumblingWindow}[0],
				TumblingWindowCronString: "invalid",
			},
			expectError:   true,
			errorContains: "error parsing cron string 'invalid' for usage tumbling window",
		},
		{
			name:    "invalid cron string - for sliding window",
			address: "http://localhost:9090",
			params: &api.UsageParams{
				WindowType:               &[]api.WindowType{api.SlidingWindow}[0],
				TumblingWindowCronString: "invalid",
			},
			expectError:   false,
			errorContains: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.params == nil {
				tt.params = &api.UsageParams{}
				tt.params.SetDefaults()
			}
			client, err := NewPrometheusClient(tt.address, tt.params)
			if tt.expectError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorContains)
				assert.Nil(t, client)
			} else {
				require.NoError(t, err)
				assert.NotNil(t, client)
				promClient, ok := client.(*PrometheusClient)
				require.True(t, ok)
				assert.NotNil(t, promClient.client)
				assert.NotNil(t, promClient.promClient)
			}
		})
	}
}
