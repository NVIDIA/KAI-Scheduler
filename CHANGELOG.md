# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

## [v0.4.11] - 2025-07-13

### Fixed
- Fixed a miscalculation where cpu/memory releasing resources were considered idle when requesting GPU fraction/memory

## [v0.4.10] - 2025-06-09

### Changed
- Changed RUNAI-VISIBLE-DEVICES key in GPU sharing configmap to NVIDIA_VISIBLE_DEVICES

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
