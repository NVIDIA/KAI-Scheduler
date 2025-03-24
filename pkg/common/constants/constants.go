// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package constants

const (
	AppLabelName              = "app"
	GpuResource               = "nvidia.com/gpu"
	ScalingPodAppLabelValue   = "scaling-pod"
	UnlimitedResourceQuantity = float64(-1)
	DefaultQueuePriority      = 100
	DefaultNodePoolName       = "default"

	// Pod Groups
	PodGrouperWarning   = "PodGrouperWarning"
	TopOwnerMetadataKey = "run.ai/top-owner-metadata"

	// Annotations
	PodGroupAnnotationForPod    = "pod-group-name"
	RunaiGpuFraction            = "gpu-fraction"
	RunaiGpuMemory              = "gpu-memory"
	GPUFractionsNumDevices      = "gpu-fraction-num-devices"
	ReceivedResourceType        = "received-resource-type"
	RunaiGpuFractionsNumDevices = "gpu-fraction-num-devices"
	RunaiGpuLimit               = "runai-gpu-limit"
	MpsAnnotation               = "mps"
	StalePodgroupTimeStamp      = "run.ai/stale-podgroup-timestamp"

	// Labels
	NodePoolNameLabel        = "runai/node-pool"
	GPUGroup                 = "runai-gpu-group"
	MultiGpuGroupLabelPrefix = GPUGroup + "/"
	MigEnabledLabel          = "node-role.kubernetes.io/runai-mig-enabled"
	MigStrategyLabel         = "nvidia.com/mig.strategy"
	GpuCountLabel            = "nvidia.com/gpu.count"
	QueueLabelKey            = "runai/queue"

	// Namespaces
	SystemPodsNamespace       = "kai-scheduler"
	RunaiReservationNamespace = "runai-reservation"
)
