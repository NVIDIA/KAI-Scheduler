// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"strings"

	v1 "k8s.io/api/core/v1"

	enginev2 "github.com/NVIDIA/KAI-scheduler/pkg/apis/scheduling/v2"
	enginev2alpha2 "github.com/NVIDIA/KAI-scheduler/pkg/apis/scheduling/v2alpha2"
	"github.com/NVIDIA/KAI-scheduler/pkg/common/constants"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/pod_status"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/plugins/snapshot"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/test_utils"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/test_utils/jobs_fake"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/test_utils/nodes_fake"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/test_utils/tasks_fake"
)

type TestGenerator struct {
	packageName  string
	testFuncName string
}

func NewTestGenerator(packageName, testFuncName string) *TestGenerator {
	return &TestGenerator{
		packageName:  packageName,
		testFuncName: testFuncName,
	}
}

func (g *TestGenerator) Generate(snap *snapshot.Snapshot) (string, error) {
	var b strings.Builder

	// Write header
	b.WriteString("// Copyright 2025 NVIDIA CORPORATION\n")
	b.WriteString("// SPDX-License-Identifier: Apache-2.0\n")
	b.WriteString("//\n")
	b.WriteString("// Code generated from snapshot. DO NOT EDIT.\n\n")
	b.WriteString(fmt.Sprintf("package %s\n\n", g.packageName))

	// Write imports
	b.WriteString("import (\n")
	b.WriteString("\t\"testing\"\n\n")
	b.WriteString("\t\"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/actions/integration_tests/integration_tests_utils\"\n")
	b.WriteString("\t\"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/pod_status\"\n")
	b.WriteString("\t\"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/test_utils\"\n")
	b.WriteString("\t\"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/test_utils/jobs_fake\"\n")
	b.WriteString("\t\"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/test_utils/nodes_fake\"\n")
	b.WriteString("\t\"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/test_utils/tasks_fake\"\n")
	b.WriteString(")\n\n")

	// Write test function
	b.WriteString(fmt.Sprintf("func %s(t *testing.T) {\n", g.testFuncName))
	b.WriteString("\tintegration_tests_utils.RunTests(t, getTestsMetadata())\n")
	b.WriteString("}\n\n")

	// Write metadata function
	b.WriteString("func getTestsMetadata() []integration_tests_utils.TestTopologyMetadata {\n")
	b.WriteString("\treturn []integration_tests_utils.TestTopologyMetadata{\n")
	b.WriteString("\t\t{\n")

	// Generate TestTopologyBasic
	topology := g.convertSnapshotToTestTopology(snap.RawObjects)
	b.WriteString(g.generateTestTopologyBasic(topology))

	b.WriteString("\t\t},\n")
	b.WriteString("\t}\n")
	b.WriteString("}\n")

	return b.String(), nil
}

type testTopology struct {
	name   string
	jobs   []*jobs_fake.TestJobBasic
	nodes  map[string]nodes_fake.TestNodeBasic
	queues []test_utils.TestQueueBasic
}

func (g *TestGenerator) convertSnapshotToTestTopology(raw *snapshot.RawKubernetesObjects) *testTopology {
	topology := &testTopology{
		name:   "snapshot-generated-test",
		jobs:   []*jobs_fake.TestJobBasic{},
		nodes:  make(map[string]nodes_fake.TestNodeBasic),
		queues: []test_utils.TestQueueBasic{},
	}

	// Convert nodes
	for _, node := range raw.Nodes {
		nodeBasic := g.convertNode(node)
		topology.nodes[node.Name] = nodeBasic
	}

	// Convert queues
	queueMap := make(map[string]*enginev2.Queue)
	for _, queue := range raw.Queues {
		queueMap[queue.Name] = queue
		topology.queues = append(topology.queues, g.convertQueue(queue))
	}

	// Group pods by PodGroup
	podGroupsMap := make(map[string][]*v1.Pod)
	for _, pg := range raw.PodGroups {
		podGroupsMap[pg.Name] = []*v1.Pod{}
	}

	for _, pod := range raw.Pods {
		pgName := pod.Annotations[constants.PodGroupAnnotationForPod]
		if pgName != "" {
			podGroupsMap[pgName] = append(podGroupsMap[pgName], pod)
		}
	}

	// Convert PodGroups to Jobs
	for _, pg := range raw.PodGroups {
		pods := podGroupsMap[pg.Name]
		if len(pods) == 0 {
			continue
		}

		job := g.convertPodGroupToJob(pg, pods, queueMap)
		if job != nil {
			topology.jobs = append(topology.jobs, job)
		}
	}

	return topology
}

func (g *TestGenerator) convertNode(node *v1.Node) nodes_fake.TestNodeBasic {
	gpus := 0
	if gpuQty, ok := node.Status.Capacity[v1.ResourceName("nvidia.com/gpu")]; ok {
		gpus = int(gpuQty.Value())
	}

	cpuMillis := float64(0)
	if cpuQty, ok := node.Status.Allocatable[v1.ResourceName("cpu")]; ok {
		cpuMillis = float64(cpuQty.MilliValue())
	}

	return nodes_fake.TestNodeBasic{
		GPUs:      gpus,
		CPUMillis: cpuMillis,
		Labels:    node.Labels,
	}
}

func (g *TestGenerator) convertQueue(queue *enginev2.Queue) test_utils.TestQueueBasic {
	qb := test_utils.TestQueueBasic{
		Name:           queue.Name,
		ParentQueue:    queue.Spec.ParentQueue,
		DeservedGPUs:   0,
		MaxAllowedGPUs: -1,
	}

	if queue.Spec.Resources != nil {
		if queue.Spec.Resources.GPU.Quota > 0 {
			qb.DeservedGPUs = float64(queue.Spec.Resources.GPU.Quota)
		}
		if queue.Spec.Resources.GPU.Limit >= 0 {
			qb.MaxAllowedGPUs = float64(queue.Spec.Resources.GPU.Limit)
		}
		qb.GPUOverQuotaWeight = float64(queue.Spec.Resources.GPU.OverQuotaWeight)
	}

	return qb
}

func (g *TestGenerator) convertPodGroupToJob(pg *enginev2alpha2.PodGroup, pods []*v1.Pod, queueMap map[string]*enginev2.Queue) *jobs_fake.TestJobBasic {
	if len(pods) == 0 {
		return nil
	}

	// Use first pod to determine job properties
	firstPod := pods[0]

	// Extract GPU requirement from first pod
	gpusPerTask := float64(0)
	if gpuQty, ok := firstPod.Spec.Containers[0].Resources.Requests[v1.ResourceName("nvidia.com/gpu")]; ok {
		gpusPerTask = float64(gpuQty.Value())
	}

	cpuPerTask := float64(0)
	if cpuQty, ok := firstPod.Spec.Containers[0].Resources.Requests[v1.ResourceName("cpu")]; ok {
		cpuPerTask = float64(cpuQty.MilliValue())
	}

	memoryPerTask := float64(0)
	if memQty, ok := firstPod.Spec.Containers[0].Resources.Requests[v1.ResourceName("memory")]; ok {
		memoryPerTask = float64(memQty.Value())
	}

	// Determine priority from pod's priority class or default
	priority := int32(0)
	if firstPod.Spec.PriorityClassName != "" {
		// Try to find priority class value (simplified - would need to look up)
		priority = 100 // default
	}

	// Get queue name
	queueName := pg.Spec.Queue
	if queueName == "" {
		queueName = "default"
	}

	// Convert pods to tasks
	tasks := []*tasks_fake.TestTaskBasic{}
	for _, pod := range pods {
		task := &tasks_fake.TestTaskBasic{
			State:    g.convertPodPhaseToStatus(pod.Status.Phase),
			NodeName: pod.Spec.NodeName,
		}
		tasks = append(tasks, task)
	}

	return &jobs_fake.TestJobBasic{
		Name:                  pg.Name,
		Namespace:             pg.Namespace,
		QueueName:             queueName,
		RequiredGPUsPerTask:   gpusPerTask,
		RequiredCPUsPerTask:   cpuPerTask,
		RequiredMemoryPerTask: memoryPerTask,
		Priority:              priority,
		Tasks:                 tasks,
	}
}

func (g *TestGenerator) convertPodPhaseToStatus(phase v1.PodPhase) pod_status.PodStatus {
	switch phase {
	case v1.PodPending:
		return pod_status.Pending
	case v1.PodRunning:
		return pod_status.Running
	case v1.PodSucceeded:
		return pod_status.Succeeded
	case v1.PodFailed:
		return pod_status.Failed
	default:
		return pod_status.Pending
	}
}

func (g *TestGenerator) generateTestTopologyBasic(topology *testTopology) string {
	var b strings.Builder

	b.WriteString("\t\t\tTestTopologyBasic: test_utils.TestTopologyBasic{\n")
	b.WriteString(fmt.Sprintf("\t\t\t\tName: %q,\n", topology.name))

	// Jobs
	b.WriteString("\t\t\t\tJobs: []*jobs_fake.TestJobBasic{\n")
	for _, job := range topology.jobs {
		b.WriteString(g.generateTestJobBasic(job))
	}
	b.WriteString("\t\t\t\t},\n")

	// Nodes
	b.WriteString("\t\t\t\tNodes: map[string]nodes_fake.TestNodeBasic{\n")
	for name, node := range topology.nodes {
		b.WriteString(fmt.Sprintf("\t\t\t\t\t%q: {\n", name))
		b.WriteString(fmt.Sprintf("\t\t\t\t\t\tGPUs: %d,\n", node.GPUs))
		if node.CPUMillis > 0 {
			b.WriteString(fmt.Sprintf("\t\t\t\t\t\tCPUMillis: %.0f,\n", node.CPUMillis))
		}
		b.WriteString("\t\t\t\t\t},\n")
	}
	b.WriteString("\t\t\t\t},\n")

	// Queues
	b.WriteString("\t\t\t\tQueues: []test_utils.TestQueueBasic{\n")
	for _, queue := range topology.queues {
		b.WriteString(g.generateTestQueueBasic(queue))
	}
	b.WriteString("\t\t\t\t},\n")

	// Mocks - estimate based on number of pending jobs
	pendingJobCount := 0
	for _, job := range topology.jobs {
		for _, task := range job.Tasks {
			if task.State == pod_status.Pending {
				pendingJobCount++
			}
		}
	}
	if pendingJobCount > 0 {
		b.WriteString("\t\t\t\tMocks: &test_utils.TestMock{\n")
		b.WriteString("\t\t\t\t\tCacheRequirements: &test_utils.CacheMocking{\n")
		b.WriteString(fmt.Sprintf("\t\t\t\t\t\tNumberOfCacheBinds: %d,\n", pendingJobCount))
		b.WriteString("\t\t\t\t\t},\n")
		b.WriteString("\t\t\t\t},\n")
	}

	b.WriteString("\t\t\t},\n")

	return b.String()
}

func (g *TestGenerator) generateTestJobBasic(job *jobs_fake.TestJobBasic) string {
	var b strings.Builder

	b.WriteString("\t\t\t\t\t{\n")
	b.WriteString(fmt.Sprintf("\t\t\t\t\t\tName: %q,\n", job.Name))
	b.WriteString(fmt.Sprintf("\t\t\t\t\t\tNamespace: %q,\n", job.Namespace))
	b.WriteString(fmt.Sprintf("\t\t\t\t\t\tQueueName: %q,\n", job.QueueName))
	b.WriteString(fmt.Sprintf("\t\t\t\t\t\tRequiredGPUsPerTask: %.1f,\n", job.RequiredGPUsPerTask))
	if job.RequiredCPUsPerTask > 0 {
		b.WriteString(fmt.Sprintf("\t\t\t\t\t\tRequiredCPUsPerTask: %.0f,\n", job.RequiredCPUsPerTask))
	}
	if job.RequiredMemoryPerTask > 0 {
		b.WriteString(fmt.Sprintf("\t\t\t\t\t\tRequiredMemoryPerTask: %.0f,\n", job.RequiredMemoryPerTask))
	}
	b.WriteString(fmt.Sprintf("\t\t\t\t\t\tPriority: %d,\n", job.Priority))

	// Tasks
	b.WriteString("\t\t\t\t\t\tTasks: []*tasks_fake.TestTaskBasic{\n")
	for _, task := range job.Tasks {
		b.WriteString("\t\t\t\t\t\t\t{\n")
		b.WriteString(fmt.Sprintf("\t\t\t\t\t\t\t\tState: pod_status.%s,\n", task.State.String()))
		if task.NodeName != "" {
			b.WriteString(fmt.Sprintf("\t\t\t\t\t\t\t\tNodeName: %q,\n", task.NodeName))
		}
		b.WriteString("\t\t\t\t\t\t\t},\n")
	}
	b.WriteString("\t\t\t\t\t\t},\n")

	b.WriteString("\t\t\t\t\t},\n")

	return b.String()
}

func (g *TestGenerator) generateTestQueueBasic(queue test_utils.TestQueueBasic) string {
	var b strings.Builder

	b.WriteString("\t\t\t\t\t{\n")
	b.WriteString(fmt.Sprintf("\t\t\t\t\t\tName: %q,\n", queue.Name))
	if queue.ParentQueue != "" {
		b.WriteString(fmt.Sprintf("\t\t\t\t\t\tParentQueue: %q,\n", queue.ParentQueue))
	}
	b.WriteString(fmt.Sprintf("\t\t\t\t\t\tDeservedGPUs: %.1f,\n", queue.DeservedGPUs))
	if queue.MaxAllowedGPUs >= 0 {
		b.WriteString(fmt.Sprintf("\t\t\t\t\t\tMaxAllowedGPUs: %.1f,\n", queue.MaxAllowedGPUs))
	}
	if queue.GPUOverQuotaWeight > 0 {
		b.WriteString(fmt.Sprintf("\t\t\t\t\t\tGPUOverQuotaWeight: %.1f,\n", queue.GPUOverQuotaWeight))
	}
	b.WriteString("\t\t\t\t\t},\n")

	return b.String()
}
