// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package usagedb

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/common_info"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/queue_info"
	"github.com/prometheus/client_golang/api"
	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewPrometheusClient(t *testing.T) {
	tests := []struct {
		name          string
		address       string
		expectError   bool
		errorContains string
	}{
		{
			name:          "valid address",
			address:       "http://localhost:9090",
			expectError:   false,
			errorContains: "",
		},
		{
			name:          "invalid address",
			address:       "://invalid:9090",
			expectError:   true,
			errorContains: "error creating prometheus client",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := NewPrometheusClient(tt.address)
			if tt.expectError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorContains)
				assert.Nil(t, client)
			} else {
				require.NoError(t, err)
				assert.NotNil(t, client)
				assert.NotNil(t, client.client)
				assert.NotNil(t, client.promClient)
			}
		})
	}
}

func TestGetResourceUsageSuccess(t *testing.T) {
	tests := []struct {
		name           string
		gpuCapResp     string
		queueUsageResp string
		expectError    bool
		expectedUsage  *queue_info.ClusterUsage
	}{
		{
			name: "successful query with data",
			gpuCapResp: `{
				"status": "success",
				"data": {
					"resultType": "vector",
					"result": [
						{
							"metric": {},
							"value": [1234567890, "8"]
						}
					]
				}
			}`,
			queueUsageResp: `{
				"status": "success",
				"data": {
					"resultType": "vector",
					"result": [
						{
							"metric": {"queue_name": "queue1"},
							"value": [1234567890, "4"]
						},
						{
							"metric": {"queue_name": "queue2"},
							"value": [1234567890, "2"]
						}
					]
				}
			}`,
			expectError: false,
			expectedUsage: func() *queue_info.ClusterUsage {
				usage := queue_info.NewClusterUsage()
				usage.Cluster.GPU = 14 // 8 from capacity + 6 from queue usage
				usage.Queues[common_info.QueueID("queue1")] = &queue_info.QueueUsage{GPU: 4}
				usage.Queues[common_info.QueueID("queue2")] = &queue_info.QueueUsage{GPU: 2}
				return usage
			}(),
		},
		{
			name: "empty response",
			gpuCapResp: `{
				"status": "success",
				"data": {
					"resultType": "vector",
					"result": []
				}
			}`,
			queueUsageResp: `{
				"status": "success",
				"data": {
					"resultType": "vector",
					"result": []
				}
			}`,
			expectError: false,
			expectedUsage: func() *queue_info.ClusterUsage {
				return queue_info.NewClusterUsage()
			}(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a test server that returns predefined responses
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				query := r.URL.Query().Get("query")
				switch query {
				case "sum_over_time(count(DCGM_FI_DEV_GPU_UTIL)[1h:1m])":
					fmt.Fprint(w, tt.gpuCapResp)
				case "sum_over_time(kai_queue_allocated_gpus[1h:1m])":
					fmt.Fprint(w, tt.queueUsageResp)
				default:
					t.Errorf("unexpected query: %s", query)
				}
			}))
			defer server.Close()

			// Create a client pointing to the test server
			client, err := api.NewClient(api.Config{Address: server.URL})
			require.NoError(t, err)

			promClient := &PrometheusClient{
				client:     v1.NewAPI(client),
				promClient: client,
			}

			// Test GetResourceUsage
			usage, err := promClient.GetResourceUsage()
			if tt.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expectedUsage, usage)
			}
		})
	}
}

func TestGetResourceUsageErrorCases(t *testing.T) {
	tests := []struct {
		name           string
		serverResponse func(w http.ResponseWriter, r *http.Request)
		expectError    bool
		errorContains  string
	}{
		{
			name: "server error response",
			serverResponse: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
				fmt.Fprint(w, "internal server error")
			},
			expectError:   true,
			errorContains: "error querying cluster GPU capacity",
		},
		{
			name: "invalid JSON response",
			serverResponse: func(w http.ResponseWriter, r *http.Request) {
				fmt.Fprint(w, "invalid json")
			},
			expectError:   true,
			errorContains: "error querying cluster GPU capacity",
		},
		{
			name: "timeout test",
			serverResponse: func(w http.ResponseWriter, r *http.Request) {
				time.Sleep(2 * time.Second)
				fmt.Fprint(w, "{}")
			},
			expectError:   true,
			errorContains: "context deadline exceeded",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(tt.serverResponse))
			defer server.Close()

			client, err := api.NewClient(api.Config{Address: server.URL})
			require.NoError(t, err)

			promClient := &PrometheusClient{
				client:     v1.NewAPI(client),
				promClient: client,
			}

			usage, err := promClient.GetResourceUsage()
			if tt.expectError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorContains)
				assert.Nil(t, usage)
			} else {
				require.NoError(t, err)
				assert.NotNil(t, usage)
			}
		})
	}
}
