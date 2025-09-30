// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package scores

// TODO: Enhance scoring precision to allow for more than 10 distinct
// score levels per category while maintaining proper category hierarchy
const (
	MaxHighDensity = 9
	ResourceType   = 10
	Availability   = 100
	GpuSharing     = 1000
	Topology       = 10000
	K8sPlugins     = 100000
	NominatedNode  = 1000000
)
