# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

## [Unreleased]

### Changed
- Changed RUNAI-VISIBLE-DEVICES key in GPU sharing configmap to NVIDIA_VISIBLE_DEVICES

### Fixed
- Fixed a miscalculation where cpu/memory releasing resources were considered idle when requesting GPU fraction/memory

## [v0.7.3] - 2025-07-08

### Removed
- Removed GPU sharing configmap name resolution from env vars and volumes

## [v0.7.2] - 2025-07-07
### Added
- Added LeaderWorkerSet support in the podGrouper. Each replica will be given a separate podGroup.

## [v0.7.1] - 2025-07-07

### Added
- Added kueue topology CRD to kai installations

### Fixed
- Fixed cases where reclaim validation operated on outdated info, allowing invalid reclaim scenarios

## [v0.7.0] - 2025-07-02

### Added
- Added optional pod and namespace label selectors to limit the scope of monitored pods
- Added a plugin extension point for scheduler plugins to add annotations to BindRequests
- Added support for Grove

### Changed
- Changed `run.ai/top-owner-metadata` to `kai.scheduler/top-owner-matadata`

## [v0.6.0] - 2025-06-16

### Changed
- Changed `runai-reservation` namespace to `kai-resource-reservation`. For migration guide, refer to this [doc](docs/migrationguides/README.md)
- Changed `runai/queue` label key to `kai.scheduler/queue`. For migration guide, refer to [doc](docs/migrationguides/README.md)

### Fixed
- Fixed pod status scheduled race condition between the scheduler and the pod binding
- Removed redundant `replicas` key for binder from `values.yaml` as it is not used and not supported

### Removed
- Removed `runai-job-id` and `runai/job-id` annotations from pods and podgroups

### Added
- Added [minruntime](docs/plugins/minruntime.md) plugin, allowing PodGroups to run for a configurable amount of time without being reclaimed/preempted.
- PodGroup Controller that will update podgroups statuses with allocation data.
- Queue Controller that will update queues statuses with allocation data.


## [v0.5.1] - 2025-05-20

### Added
- Added support for [k8s pod scheduling gates](https://kubernetes.io/docs/concepts/scheduling-eviction/pod-scheduling-readiness/)
- nodeSelector, affinity and tolerations configurable with global value definitions
- Added `PreemptMinRuntime` and `ReclaimMinRuntime` properties to queue CRD
- Scheduler now adds a "LastStartTimestamp" to podgroup on allocation

### Changed
- Queue order function now takes into account potential victims, resulting in better reclaim scenarios.

### Fixed
- Fixed preempt/reclaim of elastic workloads only taking one pod.
- Scheduler now doesn't label pods' nodepool when nodepool label value is empty
