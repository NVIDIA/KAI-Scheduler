# Time-Aware Fairness Simulator

A standalone tool for running time-aware fairness simulations for the KAI Scheduler. This tool simulates queue allocations over time and generates allocation history data that can be analyzed to understand fairness behavior.

## Quick Start

```bash
# Build the simulator
cd /path/to/KAI-Scheduler
go build -o bin/time-aware-simulator ./cmd/time-aware-simulator

# Run with example configuration
./bin/time-aware-simulator -input cmd/time-aware-simulator/example_config.yaml -v=1

# Output will be written to allocation_history_<timestamp>.csv
# You can then analyze the results with Python, Excel, or other tools
```

## Overview

The Time-Aware Fairness Simulator creates a Kubernetes test environment using `envtest`, sets up the KAI scheduler components, and simulates workload submissions to multiple queues. It tracks how resources are allocated over time and produces CSV output for analysis.

## Prerequisites

- Go 1.21 or later
- `envtest` binaries (automatically downloaded by the setup-envtest tool if not present)
- **The binary must be executed from within the KAI Scheduler repository** - it needs access to the CRD files in `deployments/kai-scheduler/crds/` and `deployments/external-crds/`

## Building

From the project root directory:

```bash
# Option 1: Use make (recommended - builds for all platforms)
make time-aware-simulator

# Option 2: Build directly with go
go build -o bin/time-aware-simulator ./cmd/time-aware-simulator
```

The Makefile will build:
- `bin/time-aware-simulator-amd64` - Linux AMD64 binary
- `bin/time-aware-simulator-arm64` - Linux ARM64 binary  
- Docker image: `registry/local/kai-scheduler/time-aware-simulator:0.0.0`

For local development on macOS/Windows, use the direct `go build` command which builds for your native platform.

## Usage

### Basic Usage

Run a simulation using a YAML configuration file:

```bash
./bin/time-aware-simulator -input example_config.yaml
```

This will:
1. Read the configuration from `example_config.yaml`
2. Set up a test Kubernetes environment
3. Run the simulation
4. Write results to `allocation_history_<timestamp>.csv`

See `example_config.yaml` in this directory for a well-commented example configuration.

### Command-Line Options

- `-input <file>`: Path to the YAML configuration file (default: `example_config.yaml`)
  - Use `-input -` to read from stdin
- `-output <file>`: Path to the output CSV file (default: `simulation_results.csv`)
  - Use `-output -` to write to stdout
- `-v <level>`: Log verbosity level (default: 0)
  - `-v=0`: Info level (basic progress messages)
  - `-v=1`: Detailed info (includes configuration details)
  - `-v=2`: Debug level (maximum verbosity)
- `-logtostderr`: Log to stderr instead of files (enabled by default)
- `-alsologtostderr`: Log to both files and stderr

The tool uses `klog` for logging, so all standard klog flags are available. See `--help` for the full list.

### Examples

**Read from file, write to default output:**
```bash
./bin/time-aware-simulator -input simulation_config_oscillating.yaml
```

**Read from stdin, write to stdout:**
```bash
cat simulation_config.yaml | ./bin/time-aware-simulator -input - -output -
```

**Verbose output with custom output file:**
```bash
./bin/time-aware-simulator -input config.yaml -output results.csv -v=1
```

**Pipe output directly to analysis tool:**
```bash
./bin/time-aware-simulator -input config.yaml -output - | python analyze.py
```

## Configuration Format

The input configuration is a YAML file with the following structure:

```yaml
# Number of simulation cycles to run (default: 100)
Cycles: 1024

# Window size for usage tracking in seconds (default: 5)
WindowSize: 256

# Half-life period for exponential decay in seconds (default: 0, disabled)
HalfLifePeriod: 128

# K-value for fairness calculation (default: 1.0)
KValue: 1.0

# Node configuration
Nodes:
  - GPUs: 16      # Number of GPUs per node
    Count: 1      # Number of nodes with this configuration

# Queue definitions
Queues:
  - Name: test-department
    Parent: ""                  # Empty for top-level queue
    Priority: 100               # Queue priority (optional, default: 100)
    DeservedGPUs: 0.0          # Base quota (optional, default: 0)
    Weight: 1.0                # Over-quota weight (optional, default: 1.0)
  
  - Name: test-queue1
    Parent: test-department     # Parent queue
    Priority: 100
    DeservedGPUs: 0.0
    Weight: 1.0
  
  - Name: test-queue2
    Parent: test-department
    Priority: 100
    DeservedGPUs: 0.0
    Weight: 1.0

# Job submissions per queue
Jobs:
  test-queue1:
    GPUs: 16        # GPUs per pod
    NumPods: 1      # Pods per job
    NumJobs: 100    # Number of jobs to submit
  
  test-queue2:
    GPUs: 16
    NumPods: 1
    NumJobs: 100
```

### Configuration Parameters

#### Global Parameters

- **Cycles**: Number of simulation cycles (each cycle is ~10ms). More cycles = longer simulation
- **WindowSize**: Time window (in seconds) for tracking usage history. Affects how quickly the fairness algorithm responds to changes
- **HalfLifePeriod**: Half-life for exponential decay (in seconds). If set to 0, decay is disabled
- **KValue**: Fairness sensitivity parameter. Higher values make the algorithm more aggressive in correcting imbalances

#### Queue Parameters

- **Name**: Unique queue identifier
- **Parent**: Name of parent queue (empty string for department/top-level queues)
- **Priority**: Queue priority (higher = more important)
- **DeservedGPUs**: Base quota allocation (guaranteed resources)
- **Weight**: Over-quota weight (relative share of excess resources)

#### Job Parameters

- **GPUs**: Number of GPUs requested per pod
- **NumPods**: Number of pods in each job
- **NumJobs**: Number of jobs to submit to this queue

## Output Format

The output is a CSV file with the following columns:

- **Time**: Simulation cycle number
- **QueueID**: Name of the queue
- **Allocation**: Actual GPU allocation at this time step
- **FairShare**: Calculated fair share for the queue at this time step

Example output:
```csv
Time,QueueID,Allocation,FairShare
0,test-queue1,16.000000,8.000000
0,test-queue2,0.000000,8.000000
1,test-queue1,16.000000,7.800000
1,test-queue2,0.000000,8.200000
...
```

## Analysis

The CSV output can be analyzed using various tools:

### Python/Pandas

```python
import pandas as pd
import matplotlib.pyplot as plt

df = pd.read_csv('allocation_history.csv')

# Plot allocations over time
for queue in df['QueueID'].unique():
    queue_data = df[df['QueueID'] == queue]
    plt.plot(queue_data['Time'], queue_data['Allocation'], label=queue)

plt.xlabel('Time')
plt.ylabel('GPU Allocation')
plt.legend()
plt.show()
```

### Jupyter Notebook

See `pkg/env-tests/plot_allocation_history.ipynb` for an example analysis notebook.

## Example Scenarios

### Two-Queue Oscillation

Demonstrates oscillation behavior between two competing queues:

```yaml
Cycles: 1024
WindowSize: 256
HalfLifePeriod: 128
Nodes:
  - GPUs: 16
    Count: 1
Queues:
  - Name: dept
    Parent: ""
  - Name: queue1
    Parent: dept
  - Name: queue2
    Parent: dept
Jobs:
  queue1:
    GPUs: 16
    NumPods: 1
    NumJobs: 100
  queue2:
    GPUs: 16
    NumPods: 1
    NumJobs: 100
```

### Burst Workload

Tests how the system handles a bursty workload alongside steady workloads:

```yaml
Cycles: 1024
WindowSize: 256
KValue: 1.0
Nodes:
  - GPUs: 16
    Count: 1
Queues:
  - Name: dept
    Parent: ""
  - Name: steady1
    Parent: dept
  - Name: steady2
    Parent: dept
  - Name: burst
    Parent: dept
Jobs:
  steady1:
    GPUs: 1
    NumPods: 1
    NumJobs: 1000
  steady2:
    GPUs: 1
    NumPods: 1
    NumJobs: 1000
  burst:
    GPUs: 12
    NumPods: 1
    NumJobs: 1000
```

### Three-Way Oscillation

Tests more complex fairness with three competing queues:

```yaml
Cycles: 1024
WindowSize: 256
KValue: 10.0
Nodes:
  - GPUs: 16
    Count: 1
Queues:
  - Name: dept
    Parent: ""
  - Name: queue1
    Parent: dept
  - Name: queue2
    Parent: dept
  - Name: queue3
    Parent: dept
Jobs:
  queue1:
    GPUs: 16
    NumPods: 1
    NumJobs: 1000
  queue2:
    GPUs: 16
    NumPods: 1
    NumJobs: 1000
  queue3:
    GPUs: 16
    NumPods: 1
    NumJobs: 1000
```

## Troubleshooting

### "Could not find project root"

Make sure you're running the simulator from within the KAI Scheduler project directory, or that the binary can access the `deployments/` directory with CRD definitions.

### "Failed to start test environment"

Ensure that:
1. The `setup-envtest` tool is available and has downloaded the necessary binaries
2. You have the required Kubernetes API binaries in your `PATH` or the tool will download them

### "Failed to create test namespace/queue/node"

The test environment may not have started correctly. Try running with `-verbose` to see more detailed error messages.

## Development

To modify the simulator:

1. Edit `main.go` for CLI handling and environment setup
2. Edit `pkg/env-tests/timeaware/timeaware.go` for simulation logic
3. Rebuild with `go build -o bin/time-aware-simulator ./cmd/time-aware-simulator`

## License

Copyright 2025 NVIDIA CORPORATION

SPDX-License-Identifier: Apache-2.0

