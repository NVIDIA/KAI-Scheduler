// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package reflectjoborder_test

import (
	"crypto/rand"
	"fmt"
	"testing"

	"go.uber.org/mock/gomock"

	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/actions/utils"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/pod_status"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/constants"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/framework"
	reflectjoborder "github.com/NVIDIA/KAI-scheduler/pkg/scheduler/plugins/reflectjoborder"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/test_utils"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/test_utils/jobs_fake"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/test_utils/nodes_fake"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/test_utils/tasks_fake"
)

func init() {
	test_utils.InitTestingInfrastructure()
}

// --- Baseline: BuildSession without the reflectjoborder plugin ---

// BenchmarkSessionOpen_SmallCluster benchmarks full session open (all default plugins, no reflectjoborder) with 50 jobs
func BenchmarkSessionOpen_SmallCluster(b *testing.B) {
	benchmarkSessionOpen(b, 50, 4, 2)
}

// BenchmarkSessionOpen_MediumCluster benchmarks full session open with 500 jobs
func BenchmarkSessionOpen_MediumCluster(b *testing.B) {
	benchmarkSessionOpen(b, 500, 4, 2)
}

// BenchmarkSessionOpen_LargeCluster benchmarks full session open with 5000 jobs
func BenchmarkSessionOpen_LargeCluster(b *testing.B) {
	benchmarkSessionOpen(b, 5000, 20, 5)
}

// BenchmarkSessionOpen_XLargeCluster benchmarks full session open with 50000 jobs
func BenchmarkSessionOpen_XLargeCluster(b *testing.B) {
	benchmarkSessionOpen(b, 50000, 100, 10)
}

// --- ReflectJobOrder OnSessionOpen ---

// BenchmarkReflectJobOrder_OnSessionOpen_SmallCluster benchmarks OnSessionOpen with 50 jobs, 4 queues
func BenchmarkReflectJobOrder_OnSessionOpen_SmallCluster(b *testing.B) {
	benchmarkOnSessionOpen(b, 50, 4, 2)
}

// BenchmarkReflectJobOrder_OnSessionOpen_MediumCluster benchmarks OnSessionOpen with 500 jobs, 4 queues
func BenchmarkReflectJobOrder_OnSessionOpen_MediumCluster(b *testing.B) {
	benchmarkOnSessionOpen(b, 500, 4, 2)
}

// BenchmarkReflectJobOrder_OnSessionOpen_LargeCluster benchmarks OnSessionOpen with 5000 jobs, 20 queues
func BenchmarkReflectJobOrder_OnSessionOpen_LargeCluster(b *testing.B) {
	benchmarkOnSessionOpen(b, 5000, 20, 5)
}

// BenchmarkReflectJobOrder_OnSessionOpen_XLargeCluster benchmarks OnSessionOpen with 50000 jobs, 100 queues
func BenchmarkReflectJobOrder_OnSessionOpen_XLargeCluster(b *testing.B) {
	benchmarkOnSessionOpen(b, 50000, 100, 10)
}

// --- Isolated JobsOrderByQueues tree build + drain ---

// BenchmarkReflectJobOrder_JobsOrderByQueues_SmallCluster benchmarks just the tree build+drain with 50 jobs
func BenchmarkReflectJobOrder_JobsOrderByQueues_SmallCluster(b *testing.B) {
	benchmarkJobsOrderByQueues(b, 50, 4, 2)
}

// BenchmarkReflectJobOrder_JobsOrderByQueues_MediumCluster benchmarks just the tree build+drain with 500 jobs
func BenchmarkReflectJobOrder_JobsOrderByQueues_MediumCluster(b *testing.B) {
	benchmarkJobsOrderByQueues(b, 500, 4, 2)
}

// BenchmarkReflectJobOrder_JobsOrderByQueues_LargeCluster benchmarks just the tree build+drain with 5000 jobs
func BenchmarkReflectJobOrder_JobsOrderByQueues_LargeCluster(b *testing.B) {
	benchmarkJobsOrderByQueues(b, 5000, 20, 5)
}

// BenchmarkReflectJobOrder_JobsOrderByQueues_XLargeCluster benchmarks just the tree build+drain with 50000 jobs
func BenchmarkReflectJobOrder_JobsOrderByQueues_XLargeCluster(b *testing.B) {
	benchmarkJobsOrderByQueues(b, 50000, 100, 10)
}

// --- ReflectJobOrder RepeatedOnSessionOpen (same plugin instance, simulates cross-session caching) ---

// BenchmarkReflectJobOrder_RepeatedOnSessionOpen_SmallCluster benchmarks repeated OnSessionOpen with 50 jobs
func BenchmarkReflectJobOrder_RepeatedOnSessionOpen_SmallCluster(b *testing.B) {
	benchmarkRepeatedOnSessionOpen(b, 50, 4, 2)
}

// BenchmarkReflectJobOrder_RepeatedOnSessionOpen_MediumCluster benchmarks repeated OnSessionOpen with 500 jobs
func BenchmarkReflectJobOrder_RepeatedOnSessionOpen_MediumCluster(b *testing.B) {
	benchmarkRepeatedOnSessionOpen(b, 500, 4, 2)
}

// BenchmarkReflectJobOrder_RepeatedOnSessionOpen_LargeCluster benchmarks repeated OnSessionOpen with 5000 jobs
func BenchmarkReflectJobOrder_RepeatedOnSessionOpen_LargeCluster(b *testing.B) {
	benchmarkRepeatedOnSessionOpen(b, 5000, 20, 5)
}

// BenchmarkReflectJobOrder_RepeatedOnSessionOpen_XLargeCluster benchmarks repeated OnSessionOpen with 50000 jobs
func BenchmarkReflectJobOrder_RepeatedOnSessionOpen_XLargeCluster(b *testing.B) {
	benchmarkRepeatedOnSessionOpen(b, 50000, 100, 10)
}

// --- Benchmark implementations ---

func benchmarkSessionOpen(b *testing.B, numJobs, numQueues, numDepts int) {
	ctrl := gomock.NewController(b)
	defer ctrl.Finish()

	topology := createBenchmarkTopology(numJobs, numQueues, numDepts)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		test_utils.BuildSession(topology, ctrl)
	}
}

func benchmarkOnSessionOpen(b *testing.B, numJobs, numQueues, numDepts int) {
	ctrl := gomock.NewController(b)
	defer ctrl.Finish()

	topology := createBenchmarkTopology(numJobs, numQueues, numDepts)
	ssn := test_utils.BuildSession(topology, ctrl)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		plugin := &reflectjoborder.JobOrderPlugin{}
		plugin.OnSessionOpen(ssn)
	}
}

func benchmarkRepeatedOnSessionOpen(b *testing.B, numJobs, numQueues, numDepts int) {
	ctrl := gomock.NewController(b)
	defer ctrl.Finish()

	topology := createBenchmarkTopology(numJobs, numQueues, numDepts)
	ssn := test_utils.BuildSession(topology, ctrl)

	builder := reflectjoborder.NewBuilder()
	plugin := builder(nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		plugin.OnSessionOpen(ssn)
	}
}

func benchmarkJobsOrderByQueues(b *testing.B, numJobs, numQueues, numDepts int) {
	ctrl := gomock.NewController(b)
	defer ctrl.Finish()

	topology := createBenchmarkTopology(numJobs, numQueues, numDepts)
	ssn := test_utils.BuildSession(topology, ctrl)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		jobsOrderByQueues := utils.NewJobsOrderByQueues(ssn, utils.JobsOrderInitOptions{
			FilterNonPending:  true,
			FilterUnready:     true,
			MaxJobsQueueDepth: ssn.GetJobsDepth(framework.Allocate),
		})
		jobsOrderByQueues.InitializeWithJobs(ssn.ClusterInfo.PodGroupInfos)

		for !jobsOrderByQueues.IsEmpty() {
			jobsOrderByQueues.PopNextJob()
		}
	}
}

// --- Topology helpers ---

func randomSuffix() string {
	b := make([]byte, 4)
	_, _ = rand.Read(b)
	return fmt.Sprintf("%x", b)
}

func createBenchmarkTopology(numJobs, numQueues, numDepts int) test_utils.TestTopologyBasic {
	numNodes := max(numJobs/10, 10)
	nodes := make(map[string]nodes_fake.TestNodeBasic)
	for i := 0; i < numNodes; i++ {
		nodes[fmt.Sprintf("node-%s-%d", randomSuffix(), i)] = nodes_fake.TestNodeBasic{
			GPUs: 8,
		}
	}

	queueNames := make([]string, numQueues)
	for i := 0; i < numQueues; i++ {
		queueNames[i] = fmt.Sprintf("queue-%s-%d", randomSuffix(), i)
	}

	deptNames := make([]string, numDepts)
	for i := 0; i < numDepts; i++ {
		deptNames[i] = fmt.Sprintf("dept-%s-%d", randomSuffix(), i)
	}

	jobs := make([]*jobs_fake.TestJobBasic, numJobs)
	for i := 0; i < numJobs; i++ {
		queueIdx := i % numQueues
		jobs[i] = &jobs_fake.TestJobBasic{
			Name:                fmt.Sprintf("job-%s-%d", randomSuffix(), i),
			RequiredGPUsPerTask: 1,
			Priority:            constants.PriorityTrainNumber,
			QueueName:           queueNames[queueIdx],
			Tasks: []*tasks_fake.TestTaskBasic{
				{State: pod_status.Pending},
			},
		}
	}

	totalGPUs := float64(numNodes * 8)
	gpusPerQueue := totalGPUs / float64(numQueues)
	gpusPerDept := totalGPUs / float64(numDepts)

	queues := make([]test_utils.TestQueueBasic, numQueues)
	for i := 0; i < numQueues; i++ {
		deptIdx := i % numDepts
		queues[i] = test_utils.TestQueueBasic{
			Name:               queueNames[i],
			ParentQueue:        deptNames[deptIdx],
			DeservedGPUs:       gpusPerQueue,
			GPUOverQuotaWeight: 1,
		}
	}

	departments := make([]test_utils.TestDepartmentBasic, numDepts)
	for i := 0; i < numDepts; i++ {
		departments[i] = test_utils.TestDepartmentBasic{
			Name:         deptNames[i],
			DeservedGPUs: gpusPerDept,
		}
	}

	return test_utils.TestTopologyBasic{
		Name:        "benchmark-reflect-job-order",
		Nodes:       nodes,
		Jobs:        jobs,
		Queues:      queues,
		Departments: departments,
		Mocks: &test_utils.TestMock{
			CacheRequirements: &test_utils.CacheMocking{},
		},
	}
}
