[![License](https://img.shields.io/badge/License-Apache_2.0-blue.svg)](LICENSE)
# KAI Scheduler
KAI Scheduler is a robust, efficient, and scalable [Kubernetes scheduler](https://kubernetes.io/docs/concepts/scheduling-eviction/kube-scheduler/) that optimizes GPU resource allocation for AI and machine learning workloads.

Designed to manage large-scale GPU clusters, including thousands of nodes, and high-throughput of workloads, makes the KAI Scheduler ideal for extensive and demanding environments.
KAI Scheduler allows administrators of Kubernetes clusters to dynamically allocate GPU resources to workloads. 

KAI Scheduler supports the entire AI lifecycle, from small, interactive jobs that require minimal resources to large training and inference, all within the same cluster. 
It ensures optimal resource allocation while maintaining resource fairness between the different consumers.
It can run alongside other schedulers installed on the cluster.

## Key Features
* [Batch Scheduling](docs/batch/README.md): Ensure all pods in a group are scheduled simultaneously or not at all.
* Bin Packing & Spread Scheduling: Optimize node usage either by minimizing fragmentation (bin-packing) or increasing resiliency and load balancing (spread scheduling).
* [Workload Priority](docs/priority/README.md): Prioritize workloads effectively within queues.
* [Hierarchical Queues](docs/queues/README.md): Manage workloads with two-level queue hierarchies for flexible organizational control.
* [Resource distribution](docs/fairness/README.md#resource-division-algorithm): Customize quotas, over-quota weights, limits, and priorities per queue.
* [Fairness Policies](docs/fairness/README.md#reclaim-strategies): Ensure equitable resource distribution using Dominant Resource Fairness (DRF) and resource reclamation across queues.
* Workload Consolidation: Reallocate running workloads intelligently to reduce fragmentation and increase cluster utilization.
* [Elastic Workloads](docs/elastic/README.md): Dynamically scale workloads within defined minimum and maximum pod counts.
* Dynamic Resource Allocation (DRA): Support vendor-specific hardware resources through Kubernetes ResourceClaims (e.g., GPUs from NVIDIA or AMD).
* [GPU Sharing](docs/gpu-sharing/README.md): Allow multiple workloads to efficiently share single or multiple GPUs, maximizing resource utilization.
* Cloud & On-premise Support: Fully compatible with dynamic cloud infrastructures (including auto-scalers like Karpenter) as well as static on-premise deployments.

## Prerequisites
Before installing KAI Scheduler, ensure you have:

- A running Kubernetes cluster
- [Helm](https://helm.sh/docs/intro/install) CLI installed
- [NVIDIA GPU-Operator](https://github.com/NVIDIA/gpu-operator) installed in order to schedule workloads that request GPU resources

## Installation
KAI Scheduler will be installed in `kai-scheduler` namespace. When submitting workloads make sure to use a dedicated namespace.

### Installation Methods
KAI Scheduler can be installed:

- **From Production (Recommended)**
- **From Source (Build it Yourself)**

#### Install from Production
Locate the latest release version in [releases](https://github.com/NVIDIA/KAI-Scheduler/releases) page.
Run the following command after replacing `<VERSION>` with the desired release version:
```sh
helm upgrade -i kai-scheduler oci://ghcr.io/nvidia/kai-scheduler/kai-scheduler -n kai-scheduler --create-namespace --version <VERSION>
```
#### Build from Source
Follow the instructions [here](docs/developer/building-from-source.md)

## Quick Start
To start scheduling workloads with KAI Scheduler, please continue to [Quick Start example](docs/quickstart/README.md)

## Support and Getting Help
We’d love to hear from you! Here's how to reach out:

- Technical Questions, Bugs, and Feature Requests: Please open [an issue on GitHub](https://github.com/NVIDIA/KAI-scheduler/issues/new) for anything related to technical support, bug reports, or feature suggestions. This helps us track and address them efficiently.
- General Discussion & Roadmap Topics: For broader conversations—like roadmap discussions, scheduling strategies, or working group coordination—join the [CNCF Slack workspace](https://communityinviter.com/apps/cloud-native/cncf) and drop by the [#batch-wg](https://cloud-native.slack.com/archives/C02Q5DFF3MM) channel.

