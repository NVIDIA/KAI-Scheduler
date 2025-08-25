// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package common

// Pyroscope configures pyroscope client connection
type Pyroscope struct {
	Address           string `json:"address,omitempty"`
	MutexProfilerRate *int   `json:"mutexProfilerRate,omitempty"`
	BlockProfilerRate *int   `json:"blockProfilerRate,omitempty"`
}
