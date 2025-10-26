// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package env_tests

import (
	"context"
	"fmt"
	"os"
	"strconv"

	"github.com/go-gota/gota/dataframe"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/NVIDIA/KAI-scheduler/pkg/env-tests/timeaware"
)

var _ = Describe("Time Aware Fairness", Ordered, func() {
	BeforeAll(func() {
		runtests, err := strconv.ParseBool(os.Getenv("RUN_TIME_AWARE_TESTS"))
		if err != nil {
			Skip(fmt.Sprintf("Failed to parse RUN_TIME_AWARE_TESTS environment variable: %v", err))
		}
		if !runtests {
			Skip("Skipping time aware fairness tests (RUN_TIME_AWARE_TESTS is not set to true)")
		}
	})

	It("Should run simulation", func(ctx context.Context) {
		allocationHistory, err, cleanupError := timeaware.RunSimulation(ctx, ctrlClient, cfg, timeaware.TimeAwareSimulation{
			Queues: []timeaware.TestQueue{
				{Name: "test-department", Parent: ""},
				{Name: "test-queue1", Parent: "test-department"},
				{Name: "test-queue2", Parent: "test-department"},
			},
			Jobs: map[string]timeaware.TestJobs{
				"test-queue1": {NumPods: 1, NumJobs: 100, GPUs: 4},
				"test-queue2": {NumPods: 1, NumJobs: 100, GPUs: 4},
			},
			Nodes: []timeaware.TestNodes{
				{GPUs: 4, Count: 1},
			},
		})
		Expect(err).NotTo(HaveOccurred(), "Failed to run simulation")
		Expect(cleanupError).NotTo(HaveOccurred(), "Failed to cleanup simulation")

		df := allocationHistory.ToDataFrame()

		// Sum allocations for each queue
		queueSums := df.GroupBy("QueueID").
			Aggregation([]dataframe.AggregationType{dataframe.Aggregation_SUM}, []string{"Allocation"})

		// Convert queueSums dataframe to map from queueID to sum
		queueSumMap := make(map[string]float64)
		for i := 0; i < queueSums.Nrow(); i++ {
			queueID := queueSums.Elem(i, 1).String()
			allocation := queueSums.Elem(i, 0).Float()
			queueSumMap[queueID] = allocation
		}

		// Assert that test-queue1 and test-queue2 allocations sum to approximately the department allocation
		// Small difference could happen due to queue controller non-atomic updates
		Expect(queueSumMap["test-queue1"]+queueSumMap["test-queue2"]).To(
			BeNumerically("~", queueSumMap["test-department"], queueSumMap["test-department"]*0.1),
			"Sum of queue1 and queue2 should equal department allocation")

		// Assert that test-queue1 and test-queue2 have approximately equal allocations (within 10%)
		Expect(queueSumMap["test-queue1"]).To(
			BeNumerically("~", queueSumMap["test-queue2"], queueSumMap["test-queue2"]*0.1),
			"Queue1 and Queue2 should have approximately equal allocations")
	})
})
