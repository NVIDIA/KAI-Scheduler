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
}

func (up *UsageParams) SetDefaults() {
	if up.HalfLifePeriod == nil {
		// noop: disabled by default
	}
	if up.WindowSize == nil {
		windowSize := time.Hour * 24 * 7
		up.WindowSize = &windowSize
	}
	if up.WindowType == nil {
		windowType := SlidingWindow
		up.WindowType = &windowType
	}
}

// WindowType defines the type of time window for aggregating usage data
type WindowType string

const (
	// TumblingWindow represents non-overlapping, fixed-size time windows
	// Example: 1-hour windows at 0-1h, 1-2h, 2-3h
	TumblingWindow WindowType = "tumbling"

	// SlidingWindow represents overlapping time windows that slide forward
	// Example: a 1-hour sliding window will consider the usage of the last 1 hour prior to the current time.
	SlidingWindow WindowType = "sliding"
)

// IsValid returns true if the WindowType is a valid value
func (wt WindowType) IsValid() bool {
	switch wt {
	case TumblingWindow, SlidingWindow:
		return true
	default:
		return false
	}
}

// GetDefaultWindowType returns the default window type (sliding)
func GetDefaultWindowType() WindowType {
	return SlidingWindow
}

// GetWindowTypeOrDefault returns the window type if set, otherwise returns the default (sliding)
func (up *UsageParams) GetWindowTypeOrDefault() WindowType {
	if up.WindowType == nil {
		return GetDefaultWindowType()
	}
	return *up.WindowType
}
