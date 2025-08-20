// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package api

import "github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/queue_info"

type Interface interface {
	GetResourceUsage() (*queue_info.ClusterUsage, error)
}
type UsageDBConfig struct {
	ClientType             string `yaml:"clientType" json:"clientType"`
	ConnectionString       string `yaml:"connectionString" json:"connectionString"`
	ConnectionStringEnvVar string `yaml:"connectionStringEnvVar" json:"connectionStringEnvVar"`
}
