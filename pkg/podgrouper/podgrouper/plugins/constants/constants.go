// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package constants

const (
	PodGroupNamePrefix = "pg"
	ProjectLabelKey    = "project"
	PriorityLabelKey   = "priorityClassName"
	UserLabelKey       = "user"
	JobIdKey           = "runai/job-id"

	BuildPriorityClass     = "build"
	TrainPriorityClass     = "train"
	InferencePriorityClass = "inference"

	DefaultQueueName = "default-queue"
)
