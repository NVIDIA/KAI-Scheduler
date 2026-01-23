# KAI Scheduler Snapshot Plugin and Tool

## Overview

The KAI Scheduler provides a snapshot plugin and tool that allows capturing and analyzing the state of the scheduler and cluster resources. This documentation covers both the snapshot plugin and the snapshot tool.

## Snapshot Plugin

The snapshot plugin is a framework plugin that provides an HTTP endpoint to capture the current state of the scheduler and cluster resources.

### Features

- Captures scheduler configuration and parameters
- Collects raw Kubernetes objects that the scheduler uses to perform its actions including:
  - Pods
  - Nodes
  - Queues
  - PodGroups
  - BindRequests
  - PriorityClasses
  - ConfigMaps
  - PersistentVolumeClaims
  - CSIStorageCapacities
  - StorageClasses
  - CSIDrivers
  - ResourceClaims
  - ResourceSlices
  - DeviceClasses

### Capturing a Snapshot

The plugin registers an HTTP endpoint `/get-snapshot` that returns a ZIP file containing a JSON snapshot of the cluster state.

To capture a snapshot, port-forward to the scheduler pod and call the endpoint:
```bash
kubectl port-forward -n kai-scheduler deployment/kai-scheduler-default 8081 &
sleep 2
curl -vv "localhost:8081/get-snapshot" > snapshot.gzip
```

### Analyzing a Snapshot

Use the snapshot tool to analyze a captured snapshot:
```bash
./bin/snapshot-tool-amd64 --filename snapshot.gzip --verbosity 8
```

See the [Snapshot Tool](#snapshot-tool) section below for more details.

### Response Format

The snapshot is returned as a ZIP file containing a single JSON file (`snapshot.json`) with the following structure:

```json
{
  "config": {
    // Scheduler configuration
  },
  "schedulerParams": {
    // Scheduler parameters
  },
  "rawObjects": {
    // Raw Kubernetes objects
  }
}
```

## Snapshot Tool

The snapshot tool is a command-line utility that can load and analyze snapshots captured by the snapshot plugin.

### Features

- Loads snapshots from ZIP files
- Recreates the scheduler environment from a snapshot
- Supports running scheduler actions on the snapshot data
- Provides detailed logging of operations

### Usage

```bash
snapshot-tool --filename <snapshot-file> [--verbosity <log-level>]
```

#### Arguments

- `--filename`: Path to the snapshot ZIP file (required)
- `--verbosity`: Logging verbosity level (default: 4)

### Example

```bash
# Load and analyze a snapshot
snapshot-tool --filename snapshot.zip

# Load and analyze a snapshot with increased verbosity
snapshot-tool --filename snapshot.zip --verbosity 5
```

## Generating Integration Tests from Snapshots

The snapshot-tool can generate Go integration test code from snapshot files using the `--generate-test` flag. This allows you to convert captured cluster snapshots into executable integration tests that can be run as part of the test suite.

### Features

- Generates boilerplate Go integration test code from snapshot files
- Provides a template structure that is easy to edit and customize
- Includes snapshot summary information (node names, queue names, PodGroup names) as comments
- Creates a starting point for writing integration tests based on real cluster states

### Usage

```bash
snapshot-tool --filename <snapshot-file> --generate-test [options]
```

#### Arguments

- `--filename`: Path to the snapshot ZIP file (required)
- `--generate-test`: Enable test generation mode
- `--output`: Output Go test file path (default: `<snapshot-basename>_test.go`)
- `--test-name`: Name for the generated test function (default: `TestSnapshot<snapshot-basename>`)
- `--package`: Package name for generated test file (default: `snapshots_test`)

### Examples

```bash
# Generate test file with default settings
snapshot-tool --filename snapshot.gzip --generate-test

# Generate test file with custom output path and test name
snapshot-tool \
  --filename snapshot.gzip \
  --generate-test \
  --output pkg/scheduler/actions/integration_tests/snapshots/my_test.go \
  --test-name TestMySnapshot

# Generate test file with custom package name
snapshot-tool \
  --filename snapshot.gzip \
  --generate-test \
  --package my_test_package
```

### Generated Test Structure

The tool generates a boilerplate Go test file containing:

1. A test function that calls `integration_tests_utils.RunTests()`
2. A `getTestsMetadata()` function with a template structure
3. Summary information from the snapshot as comments:
   - Number of nodes, pods, PodGroups, and queues
   - Names of nodes, queues, and PodGroups found in the snapshot
4. TODO comments and example structures for:
   - Jobs (with example job structure)
   - Nodes (with example node structure)
   - Queues (with example queue structure)
   - Mocks (commented out, ready to configure if needed)
   - JobExpectedResults (commented out, ready to add expected outcomes)

The generated file is designed to be easily editable. You should fill in the actual test data based on your requirements and the snapshot information provided in the comments.

### Workflow

1. Capture a snapshot from a running cluster:
   ```bash
   kubectl port-forward -n kai deployment/scheduler 8081 &
   curl "localhost:8081/get-snapshot" > snapshot.gzip
   ```

2. Generate integration test boilerplate:
   ```bash
   snapshot-tool --filename snapshot.gzip --generate-test
   ```

3. Edit the generated test file:
   - Review the snapshot summary comments to understand the cluster state
   - Fill in the test data structures (Jobs, Nodes, Queues) based on your test requirements
   - Add expected results if needed
   - Configure mocks if required

4. Run the test:
   ```bash
   go test ./pkg/scheduler/actions/integration_tests/snapshots/... -v -run TestSnapshot...
   ```

## Implementation Details

### Snapshot Plugin

The snapshot plugin (`pkg/scheduler/plugins/snapshot/snapshot.go`) implements the following key components:

1. `RawKubernetesObjects`: Structure containing all captured Kubernetes objects
2. `Snapshot`: Main structure containing configuration, parameters, and raw objects
3. `snapshotPlugin`: Plugin implementation with HTTP endpoint handler

### Snapshot Tool

The snapshot tool (`cmd/snapshot-tool/main.go`) implements:

1. Snapshot loading and parsing (using `pkg/snapshotrunner`)
2. Fake client creation with snapshot data
3. Scheduler cache initialization
4. Session management
5. Action execution
6. Test generation (when `--generate-test` flag is used, using `pkg/snapshottest`)

The snapshot execution logic has been extracted to `pkg/snapshotrunner/runner.go`, which provides reusable functions for loading and running snapshots. This package is located outside the scheduler package to keep the codebase organized.

### Test Generation

The test generation functionality (`pkg/snapshottest/generator.go`) implements:

1. Snapshot loading using `snapshotrunner.LoadSnapshot()` from `pkg/snapshotrunner`
2. Extraction of summary information from the snapshot (counts and names)
3. Generation of boilerplate Go test code with:
   - Basic test function structure
   - Template `getTestsMetadata()` function with TODO comments
   - Example structures for all test components
   - Snapshot summary information as comments

The generated boilerplate is intentionally minimal and easy to edit, allowing developers to customize the test based on their specific requirements.

## Limitations

- The snapshot tool runs in a simulated environment
- Some real-time cluster features may not be available
- Resource constraints may differ from the original cluster 
