// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package constants

const (
	AppLabelName              = "app"
	GpuResource               = "nvidia.com/gpu"
	NvidiaGpuMemory           = "nvidia.com/gpu.memory"
	UnlimitedResourceQuantity = float64(-1)

	DefaultQueuePriority                  = 100
	DefaultNodePoolName                   = "default"
	DefaultMetricsNamespace               = "kai"
	DefaultQueueLabel                     = "kai.scheduler/queue"
	DefaultSchedulerName                  = "kai-scheduler"
	DefaultKAINamespace                   = "kai-scheduler"
	DefaultResourceReservationName        = "kai-resource-reservation"
	DefaultScaleAdjustName                = "kai-scale-adjust"
	DefaultKAIConfigSingeltonInstanceName = "kai-config"
	DefaultNodePoolLabelKey               = ""
	DefaultRuntimeClassName               = "nvidia"

	DefaultCPUWorkerNodeLabelKey = "node-role.kubernetes.io/cpu-worker"
	DefaultGPUWorkerNodeLabelKey = "node-role.kubernetes.io/gpu-worker"
	DefaultMIGWorkerNodeLabelKey = "node-role.kubernetes.io/mig-enabled"

	// Pod Groups
	PodGrouperWarning   = "PodGrouperWarning"
	TopOwnerMetadataKey = "kai.scheduler/top-owner-metadata"

	// Annotations
	PodGroupAnnotationForPod = "pod-group-name"
	GpuFraction              = "gpu-fraction"
	GpuMemory                = "gpu-memory"
	ReceivedResourceType     = "received-resource-type"
	GpuFractionsNumDevices   = "gpu-fraction-num-devices"
	MpsAnnotation            = "mps"
	StalePodgroupTimeStamp   = "kai.scheduler/stale-podgroup-timestamp"
	LastStartTimeStamp       = "kai.scheduler/last-start-timestamp"

	// Labels
	GPUGroup                 = "runai-gpu-group"
	MultiGpuGroupLabelPrefix = GPUGroup + "/"
	MigStrategyLabel         = "nvidia.com/mig.strategy"
	GpuCountLabel            = "nvidia.com/gpu.count"
	SubGroupLabelKey         = "kai.scheduler/subgroup-name"
)

// QueueValidatedVersions returns the list of queue versions that we validate with a webhook. This will be used by the
// kai operator when installing webhooks. When changing this, test for backwards compatibility.
func QueueValidatedVersions() []string {
	return []string{"v2"}
}
