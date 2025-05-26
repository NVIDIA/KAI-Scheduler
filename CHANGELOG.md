# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

## [v0.4.9]

### Fixed
- Fixed pod status scheduled race condition between the scheduler and the pod binding

## [v0.4.8]

### Fixed
- Queue order function now takes into account potential victims, resulting in better reclaim scenarios.

### CHANGED
- Cached GetDeservedShare and GetFairShare function in the scheduler PodGroupInfo to improve performance.
- Added cache to the binder resource reservation client.
- More Caching and improvements to PodGroupInfo class.
- Update pod labels after scheduling decision concurrently in the background.

## [v0.4.7]
