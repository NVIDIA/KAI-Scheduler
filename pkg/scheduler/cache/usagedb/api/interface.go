// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package api

import (
	"time"

	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/queue_info"
)

type Interface interface {
	GetResourceUsage() (*queue_info.ClusterUsage, error)
}
type UsageDBConfig struct {
	ClientType             string       `yaml:"clientType" json:"clientType"`
	ConnectionString       string       `yaml:"connectionString" json:"connectionString"`
	ConnectionStringEnvVar string       `yaml:"connectionStringEnvVar" json:"connectionStringEnvVar"`
	UsageParams            *UsageParams `yaml:"usageParams" json:"usageParams"`
}

// GetUsageParams returns the usage params if set, and default params if not set.
func (c *UsageDBConfig) GetUsageParams() *UsageParams {
	up := UsageParams{}
	if c.UsageParams != nil {
		up = *c.UsageParams
	}
	up.SetDefaults()
	return &up
}

// UsageParams defines common params for all usage db clients. Some clients may not support all the params.
type UsageParams struct {
	// Half life period of the usage. If not set, or set to 0, the usage will not be decayed.
	HalfLifePeriod *time.Duration `yaml:"halfLifePeriod" json:"halfLifePeriod"`
	// Window size of the usage. Default is 1 week.
	WindowSize *time.Duration `yaml:"windowSize" json:"windowSize"`
	// Window type for time-series aggregation. If not set, defaults to sliding.
	WindowType *WindowType `yaml:"windowType" json:"windowType"`
	// A cron string used to determine when to reset resource usage for all queues.
	TumblingWindowCronString string `yaml:"tumblingWindowCronString" json:"tumblingWindowCronString"`
	// Fetch interval of the usage. Default is 1 minute.
	FetchInterval *time.Duration `yaml:"fetchInterval" json:"fetchInterval"`
	// Staleness period of the usage. Default is 5 minutes.
	StalenessPeriod *time.Duration `yaml:"stalenessPeriod" json:"stalenessPeriod"`
	// Wait timeout of the usage. Default is 1 minute.
	WaitTimeout *time.Duration `yaml:"waitTimeout" json:"waitTimeout"`

	// ExtraParams are extra parameters for the usage db client, which are client specific.
	ExtraParams map[string]string `yaml:"extraParams" json:"extraParams"`
}
