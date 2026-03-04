# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

## [Unreleased]

### Fixed

- Updated resource enumeration logic to exclude resources with count of 0. [#1120](https://github.com/NVIDIA/KAI-Scheduler/issues/1120)

## [v0.13.0] - 2026-03-02
### Added
- Added `global.nodeSelector` propagation from Helm values to Config CR, ensuring operator-created sub-component deployments (admission, binder, scheduler, pod-grouper, etc.) receive the configured nodeSelector [#1102](https://github.com/NVIDIA/KAI-Scheduler/pull/1102) [yuanchen8911](https://github.com/yuanchen8911)
- Added `plugins` and `actions` fields to SchedulingShard spec, allowing per-shard customization of scheduler plugin/action enablement, priority, and arguments [gshaibi](https://github.com/gshaibi)
- Added support for Kubeflow Trainer v2 TrainJob workloads via skipTopOwner grouper pattern
- Added `binder.cdiEnabled` Helm value to allow explicit override of CDI auto-detection for environments without ClusterPolicy
- Added metric for tracking evicted pods in pod groups, including nodepool, eviction action, and gang size
- Block scheduling of pods with shared (non-template) DRA GPU claims that lack a queue label or have a mismatched queue label [gshaibi](https://github.com/gshaibi)
- Added the option to disable prometheus service monitor creation [#810](https://github.com/NVIDIA/KAI-Scheduler/pull/810) [itsomri](https://github.com/itsomri)
- Fixed prometheus instance deprecation - ensure single instance [#779](https://github.com/NVIDIA/KAI-Scheduler/pull/779) [itsomri](https://github.com/itsomri)
- Added clear error messages for jobs referencing missing or orphan queues, reporting via events and conditions [#820](https://github.com/NVIDIA/KAI-Scheduler/pull/820) [gshaibi](https://github.com/gshaibi)
- Added rule selector for resource accounting prometheus [#818](https://github.com/NVIDIA/KAI-Scheduler/pull/818) [itsomri](https://github.com/itsomri)
- Made accounting labels configurable [#818](https://github.com/NVIDIA/KAI-Scheduler/pull/818) [itsomri](https://github.com/itsomri)
- Added support for Grove hierarchical topology constraints in PodGroup subgroups
- Added support for n-level queue hierarchies [#858](https://github.com/NVIDIA/KAI-Scheduler/pull/858) [gshaibi](https://github.com/gshaibi)
- Added labels and annotations propagation from topOwner in SkipTopOwner grouper [#861](https://github.com/NVIDIA/KAI-Scheduler/pull/861) [SiorMeir](https://github.com/siormeir)
- Added scheduler name match conditions to admission webhooks to improve cluster stability
- Add Gpu Dra claims and resource slices accounting for the purpose of resource management and quota guarantees. *** This change doesn't support shared gpu claims or gpu claims with FirstAvailable *** [#900](https://github.com/NVIDIA/KAI-Scheduler/pull/900) [davidLif](https://github.com/davidLif) 
- Added DRA resources recording to snapshot [#830](https://github.com/NVIDIA/KAI-Scheduler/pull/830)
- Temporarily Prevent device-plugin GPU pods on DRA-only nodes - until translation between device-plugin notation and DRA is implemented
- Implemented subgroups for pytorchjobs [#935](https://github.com/NVIDIA/KAI-Scheduler/pull/935) [itsomri](https://github.com/itsomri)
- Made KAI images distroless [#745](https://github.com/NVIDIA/KAI-Scheduler/pull/745) [dttung2905](https://github.com/dttung2905)
- Allow setting empty gpuPodRuntimeClassName during helm install [#972](https://github.com/NVIDIA/KAI-Scheduler/pull/972) [steved](https://github.com/steved)
- Created scale tests scenarios for running scale tests for KAI [#967](https://github.com/NVIDIA/KAI-Scheduler/pull/967)
- Implemented block-level segmentation for pytorchjobs [#938](https://github.com/NVIDIA/KAI-Scheduler/pull/938) [itsomri](https://github.com/itsomri)
- Added scale test environment setup script and updated service monitors for KAI scheduler [#1031](https://github.com/NVIDIA/KAI-Scheduler/pull/1031)
- Implemented subgroups for leaderworkerset [#1046](https://github.com/NVIDIA/KAI-Scheduler/pull/1046) [davidLif](https://github.com/davidLif) 
- Added discovery data to snapshot for more accurate debugging [#1047](https://github.com/NVIDIA/KAI-Scheduler/pull/1047) [itsomri](https://github.com/itsomri)
- Implemented subgroup segmentation (with topology segment definitions) for leaderworkerset [#1058](https://github.com/NVIDIA/KAI-Scheduler/pull/10586) [davidLif](https://github.com/davidLif)

### Fixed
- Fixed plugin server (snapshot and job-order endpoints) listening on all interfaces by binding to localhost only.

## [v0.4.18] - 2026-01-25

## [v0.4.17] - 2026-01-07

### Fixed
- Fixed a bug where the scheduler would not re-try updating podgroup status after failure
- GPU Memory pods are not reclaimed or consolidated correctly
- Fixed GPU memory pods Fair Share and Queue Order calculations

## [v0.4.13-16] - ???

### Fixed
- kai-scheduler will not ignore pod spec.overhead field
- Fixed wrong GPU memory unit conversion from node `nvidia.com/gpu.memory` labels
- Fixed incorrect MIG GPU usage calculation leading to wrong scheduling decision

## [v0.4.12] - 2025-07-18

### Fixed
- Fixed a scenario where only GPU resources where checked for job and node, causing it to be bound instead of being pipelined

## [v0.4.11] - 2025-07-13

### Fixed
- Fixed a miscalculation where cpu/memory releasing resources were considered idle when requesting GPU fraction/memory

## [v0.4.10] - 2025-06-09

### Fixed
- Fix scheduler pod group status synchronization between incoming update and in-cluster data

## [v0.4.9] - 2025-05-27

### Fixed
- Fixed pod status scheduled race condition between the scheduler and the pod binding
- Scheduler now doesn't label pods' nodepool when nodepool label value is empty

## [v0.4.8]

### Fixed
- Queue order function now takes into account potential victims, resulting in better reclaim scenarios.

### CHANGED
- Cached GetDeservedShare and GetFairShare function in the scheduler PodGroupInfo to improve performance.
- Added cache to the binder resource reservation client.
- More Caching and improvements to PodGroupInfo class.
- Update pod labels after scheduling decision concurrently in the background.

## [v0.4.7]
