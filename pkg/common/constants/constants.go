// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package constants

const (
	AppLabelName              = "app"
	GpuResource               = "nvidia.com/gpu"
	UnlimitedResourceQuantity = float64(-1)
	DefaultQueuePriority      = 100
	DefaultNodePoolName       = "default"

	// Pod Groups
	PodGrouperWarning   = "PodGrouperWarning"
	TopOwnerMetadataKey = "run.ai/top-owner-metadata"

	// Annotations
	PodGroupAnnotationForPod = "pod-group-name"
	GpuFraction              = "gpu-fraction"
	GpuMemory                = "gpu-memory"
	ReceivedResourceType     = "received-resource-type"
	GpuFractionsNumDevices   = "gpu-fraction-num-devices"
	RunaiGpuLimit            = "runai-gpu-limit"
	MpsAnnotation            = "mps"
	StalePodgroupTimeStamp   = "kai.scheduler/stale-podgroup-timestamp"
	LastStartTimeStamp       = "kai.scheduler/last-start-timestamp"

	// Labels
	GPUGroup                 = "runai-gpu-group"
	MultiGpuGroupLabelPrefix = GPUGroup + "/"
	MigEnabledLabel          = "node-role.kubernetes.io/runai-mig-enabled"
	MigStrategyLabel         = "nvidia.com/mig.strategy"
	GpuCountLabel            = "nvidia.com/gpu.count"
)
