// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package resource_division

import (
	"math"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/stretchr/testify/assert"

	commonconstants "github.com/NVIDIA/KAI-scheduler/pkg/common/constants"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/common_info"
	rs "github.com/NVIDIA/KAI-scheduler/pkg/scheduler/plugins/proportion/resource_share"
)

func newQueue(id, name string, priority int, deserved, weight, request float64) *rs.QueueAttributes {
	qa := &rs.QueueAttributes{
		UID:               common_info.QueueID(types.UID(id)),
		Name:              name,
		Priority:          priority,
		CreationTimestamp: metav1.Now(),
	}
	qa.SetQuotaResources(rs.CpuResource, deserved, commonconstants.UnlimitedResourceQuantity, weight)
	qa.ResourceShare(rs.CpuResource).Request = request
	return qa
}

// Test scenarios from the design document.
func TestSetResourcesSharePriority_Scenarios(t *testing.T) {
	tests := []struct {
		name           string
		totalCPU       float64
		queues         []*rs.QueueAttributes
		expectedShares map[string]int // queueName -> cpu fair share
	}{
		{
			name:     "Example 1 – Capacity Exhausted at Top Priority",
			totalCPU: 100,
			queues: []*rs.QueueAttributes{
				newQueue("A", "A", 100, 20, 0, 70),
				newQueue("B", "B", 100, 30, 2, 40),
				newQueue("C", "C", 50, 30, 1, 50),
			},
			expectedShares: map[string]int{
				"A": 60,
				"B": 40,
				"C": 0,
			},
		},
		{
			name:     "Example 2 – Spare Capacity Reaches Lower Priority",
			totalCPU: 120,
			queues: []*rs.QueueAttributes{
				newQueue("A", "A", 100, 20, 0, 70),
				newQueue("B", "B", 100, 30, 2, 40),
				newQueue("C", "C", 50, 30, 1, 50),
			},
			expectedShares: map[string]int{
				"A": 70,
				"B": 40,
				"C": 10,
			},
		},
		{
			name:     "Example 3 – Capacity Spills to Next Priority",
			totalCPU: 200,
			queues: []*rs.QueueAttributes{
				newQueue("A", "A", 100, 20, 0, 70),
				newQueue("B", "B", 100, 30, 2, 50),
				newQueue("C", "C", 50, 30, 1, 90),
			},
			expectedShares: map[string]int{
				"A": 70,
				"B": 50,
				"C": 80,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			queuesMap := make(map[common_info.QueueID]*rs.QueueAttributes)
			for _, q := range tt.queues {
				queuesMap[q.UID] = q
			}
			total := rs.ResourceQuantities{
				rs.CpuResource:    tt.totalCPU,
				rs.MemoryResource: 0,
				rs.GpuResource:    0,
			}

			SetResourcesSharePriority(total, queuesMap)

			for _, q := range tt.queues {
				gotFloat := q.ResourceShare(rs.CpuResource).FairShare
				want := tt.expectedShares[q.Name]
				assert.Equal(t, want, int(math.Round(gotFloat)), "queue %s. want: %v, got: %v", q.Name, want, int(math.Round(gotFloat)))
			}
		})
	}
}
