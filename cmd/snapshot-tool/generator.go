// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"path/filepath"
	"strings"

	snapshotplugin "github.com/NVIDIA/KAI-scheduler/pkg/scheduler/plugins/snapshot"
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

func (g *TestGenerator) Generate(snap *snapshotplugin.Snapshot) (string, error) {
	var b strings.Builder

	b.WriteString("// Copyright 2025 NVIDIA CORPORATION\n")
	b.WriteString("// SPDX-License-Identifier: Apache-2.0\n")
	b.WriteString("//\n")
	b.WriteString("// Code generated from snapshot.\n")
	b.WriteString("// This is a boilerplate file that you can edit to customize your test.\n\n")
	b.WriteString(fmt.Sprintf("package %s\n\n", g.packageName))

	b.WriteString("import (\n")
	b.WriteString("\t\"testing\"\n\n")
	b.WriteString("\t\"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/actions/integration_tests/integration_tests_utils\"\n")
	b.WriteString("\t\"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/pod_status\"\n")
	b.WriteString("\t\"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/test_utils\"\n")
	b.WriteString("\t\"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/test_utils/jobs_fake\"\n")
	b.WriteString("\t\"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/test_utils/nodes_fake\"\n")
	b.WriteString("\t\"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/test_utils/tasks_fake\"\n")
	b.WriteString(")\n\n")

	summary := g.extractSnapshotSummary(snap)

	b.WriteString(fmt.Sprintf("func %s(t *testing.T) {\n", g.testFuncName))
	b.WriteString("\tintegration_tests_utils.RunTests(t, getTestsMetadata())\n")
	b.WriteString("}\n\n")

	b.WriteString("func getTestsMetadata() []integration_tests_utils.TestTopologyMetadata {\n")
	b.WriteString("\treturn []integration_tests_utils.TestTopologyMetadata{\n")
	b.WriteString("\t\t{\n")
	b.WriteString("\t\t\tTestTopologyBasic: test_utils.TestTopologyBasic{\n")

	b.WriteString(generateSummaryComments(summary))

	b.WriteString("\t\t\t\tName: \"test-name\", // TODO: Update with descriptive test name\n")
	b.WriteString("\n")

	b.WriteString("\t\t\t\tJobs: []*jobs_fake.TestJobBasic{\n")
	b.WriteString("\t\t\t\t\t// TODO: Add jobs based on snapshot\n")
	b.WriteString("\t\t\t\t\t// Example:\n")
	b.WriteString("\t\t\t\t\t// {\n")
	b.WriteString("\t\t\t\t\t// \tName: \"job-name\",\n")
	b.WriteString("\t\t\t\t\t// \tNamespace: \"default\",\n")
	b.WriteString("\t\t\t\t\t// \tQueueName: \"queue-name\",\n")
	b.WriteString("\t\t\t\t\t// \tRequiredGPUsPerTask: 1.0,\n")
	b.WriteString("\t\t\t\t\t// \tPriority: 100,\n")
	b.WriteString("\t\t\t\t\t// \tTasks: []*tasks_fake.TestTaskBasic{\n")
	b.WriteString("\t\t\t\t\t// \t\t{State: pod_status.Pending},\n")
	b.WriteString("\t\t\t\t\t// \t},\n")
	b.WriteString("\t\t\t\t\t// },\n")
	b.WriteString("\t\t\t\t},\n")
	b.WriteString("\n")

	b.WriteString("\t\t\t\tNodes: map[string]nodes_fake.TestNodeBasic{\n")
	b.WriteString("\t\t\t\t\t// TODO: Add nodes based on snapshot\n")
	b.WriteString("\t\t\t\t\t// Example:\n")
	b.WriteString("\t\t\t\t\t// \"node-name\": {\n")
	b.WriteString("\t\t\t\t\t// \tGPUs: 4,\n")
	b.WriteString("\t\t\t\t\t// \tCPUMillis: 8000,\n")
	b.WriteString("\t\t\t\t\t// },\n")
	b.WriteString("\t\t\t\t},\n")
	b.WriteString("\n")

	b.WriteString("\t\t\t\tQueues: []test_utils.TestQueueBasic{\n")
	b.WriteString("\t\t\t\t\t// TODO: Add queues based on snapshot\n")
	b.WriteString("\t\t\t\t\t// Example:\n")
	b.WriteString("\t\t\t\t\t// {\n")
	b.WriteString("\t\t\t\t\t// \tName: \"queue-name\",\n")
	b.WriteString("\t\t\t\t\t// \tDeservedGPUs: 2.0,\n")
	b.WriteString("\t\t\t\t\t// \tMaxAllowedGPUs: 4.0,\n")
	b.WriteString("\t\t\t\t\t// },\n")
	b.WriteString("\t\t\t\t},\n")
	b.WriteString("\n")

	b.WriteString("\t\t\t\t// TODO: Configure mocks if needed\n")
	b.WriteString("\t\t\t\t// Mocks: &test_utils.TestMock{\n")
	b.WriteString("\t\t\t\t// \tCacheRequirements: &test_utils.CacheMocking{\n")
	b.WriteString("\t\t\t\t// \t\tNumberOfCacheBinds: 1,\n")
	b.WriteString("\t\t\t\t// \t},\n")
	b.WriteString("\t\t\t\t// },\n")
	b.WriteString("\n")

	b.WriteString("\t\t\t\t// TODO: Add expected results for jobs\n")
	b.WriteString("\t\t\t\t// JobExpectedResults: map[string]test_utils.TestExpectedResultBasic{\n")
	b.WriteString("\t\t\t\t// \t\"job-name\": {\n")
	b.WriteString("\t\t\t\t// \t\tNodeName: \"node-name\",\n")
	b.WriteString("\t\t\t\t// \t\tGPUsRequired: 1.0,\n")
	b.WriteString("\t\t\t\t// \t\tStatus: pod_status.Running,\n")
	b.WriteString("\t\t\t\t// \t},\n")
	b.WriteString("\t\t\t\t// },\n")

	b.WriteString("\t\t\t},\n")
	b.WriteString("\t\t\tRoundsUntilMatch: 1, // TODO: Adjust based on test requirements\n")
	b.WriteString("\t\t},\n")
	b.WriteString("\t}\n")
	b.WriteString("}\n")

	return b.String(), nil
}

type snapshotSummary struct {
	nodeCount     int
	podCount      int
	podGroupCount int
	queueCount    int
	nodeNames     []string
	queueNames    []string
	podGroupNames []string
}

func (g *TestGenerator) extractSnapshotSummary(snap *snapshotplugin.Snapshot) snapshotSummary {
	summary := snapshotSummary{
		nodeNames:     []string{},
		queueNames:    []string{},
		podGroupNames: []string{},
	}

	if snap.RawObjects != nil {
		summary.nodeCount = len(snap.RawObjects.Nodes)
		summary.podCount = len(snap.RawObjects.Pods)
		summary.podGroupCount = len(snap.RawObjects.PodGroups)
		summary.queueCount = len(snap.RawObjects.Queues)

		for _, node := range snap.RawObjects.Nodes {
			summary.nodeNames = append(summary.nodeNames, node.Name)
		}
		for _, queue := range snap.RawObjects.Queues {
			summary.queueNames = append(summary.queueNames, queue.Name)
		}
		for _, pg := range snap.RawObjects.PodGroups {
			summary.podGroupNames = append(summary.podGroupNames, pg.Name)
		}
	}

	return summary
}

func generateSummaryComments(summary snapshotSummary) string {
	var b strings.Builder
	b.WriteString("\t\t\t\t// Snapshot summary:\n")
	b.WriteString(fmt.Sprintf("\t\t\t\t// - Nodes: %d\n", summary.nodeCount))
	b.WriteString(fmt.Sprintf("\t\t\t\t// - Pods: %d\n", summary.podCount))
	b.WriteString(fmt.Sprintf("\t\t\t\t// - PodGroups: %d\n", summary.podGroupCount))
	b.WriteString(fmt.Sprintf("\t\t\t\t// - Queues: %d\n", summary.queueCount))

	if len(summary.nodeNames) > 0 {
		b.WriteString("\t\t\t\t// - Node names: ")
		for i, name := range summary.nodeNames {
			if i > 0 {
				b.WriteString(", ")
			}
			b.WriteString(name)
		}
		b.WriteString("\n")
	}

	if len(summary.queueNames) > 0 {
		b.WriteString("\t\t\t\t// - Queue names: ")
		for i, name := range summary.queueNames {
			if i > 0 {
				b.WriteString(", ")
			}
			b.WriteString(name)
		}
		b.WriteString("\n")
	}

	if len(summary.podGroupNames) > 0 {
		b.WriteString("\t\t\t\t// - PodGroup names: ")
		for i, name := range summary.podGroupNames {
			if i > 0 {
				b.WriteString(", ")
			}
			b.WriteString(name)
		}
		b.WriteString("\n")
	}

	b.WriteString("\n")
	return b.String()
}

// GenerateTestName converts a snapshot filename to a test function name.
func GenerateTestName(snapshotFile string) string {
	base := strings.TrimSuffix(filepath.Base(snapshotFile), filepath.Ext(snapshotFile))
	base = strings.TrimSuffix(base, ".gzip")
	parts := strings.Split(strings.ReplaceAll(base, "-", "_"), "_")
	for i, part := range parts {
		if len(part) > 0 {
			parts[i] = strings.ToUpper(part[:1]) + part[1:]
		}
	}
	return "TestSnapshot" + strings.Join(parts, "")
}

// GenerateOutputPath converts a snapshot filename to an output file path.
func GenerateOutputPath(snapshotFile string) string {
	base := strings.TrimSuffix(filepath.Base(snapshotFile), filepath.Ext(snapshotFile))
	base = strings.TrimSuffix(base, ".gzip")
	return base + "_test.go"
}
