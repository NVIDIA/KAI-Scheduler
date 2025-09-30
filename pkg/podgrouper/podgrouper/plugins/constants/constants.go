// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package constants

const (
	PodGroupNamePrefix     = "pg"
	ProjectLabelKey        = "project"
	PriorityLabelKey       = "priorityClassName"
	PreemptibilityLabelKey = "kai.scheduler/preemptibility"
	UserLabelKey           = "user"

	BuildPriorityClass     = "build"
	TrainPriorityClass     = "train"
	InferencePriorityClass = "inference"

	DefaultPrioritiesConfigMapTypesKey = "types"

	DefaultQueueName = "default-queue"
)
