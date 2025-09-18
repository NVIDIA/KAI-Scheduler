// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package api

import (
	"maps"
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

func (c *UsageDBConfig) DeepCopy() *UsageDBConfig {
	return &UsageDBConfig{
		ClientType:             c.ClientType,
		ConnectionString:       c.ConnectionString,
		ConnectionStringEnvVar: c.ConnectionStringEnvVar,
		UsageParams:            c.UsageParams.DeepCopy(),
	}
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

	// ExtraParams are extra parameters for the usage db client, which are client specific.
	ExtraParams map[string]string `yaml:"extraParams" json:"extraParams"`
}

func (up *UsageParams) DeepCopy() *UsageParams {
	return &UsageParams{
		HalfLifePeriod:           up.HalfLifePeriod,
		WindowSize:               up.WindowSize,
		WindowType:               up.WindowType,
		TumblingWindowCronString: up.TumblingWindowCronString,
		ExtraParams:              maps.Clone(up.ExtraParams),
	}
}
