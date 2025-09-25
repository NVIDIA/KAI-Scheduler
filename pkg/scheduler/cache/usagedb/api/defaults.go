// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package api

import "time"

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
	if up.FetchInterval == nil {
		fetchInterval := 1 * time.Minute
		up.FetchInterval = &fetchInterval
	}
	if up.StalenessPeriod == nil {
		stalenessPeriod := 5 * time.Minute
		up.StalenessPeriod = &stalenessPeriod
	}
	if up.WaitTimeout == nil {
		waitTimeout := 1 * time.Minute
		up.WaitTimeout = &waitTimeout
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

func (up *UsageParams) GetExtraDurationParamOrDefault(key string, defaultValue time.Duration) time.Duration {
	if up.ExtraParams == nil {
		return defaultValue
	}

	value, exists := up.ExtraParams[key]
	if !exists {
		return defaultValue
	}

	duration, err := time.ParseDuration(value)
	if err != nil {
		return defaultValue
	}

	return duration
}

func (up *UsageParams) GetExtraStringParamOrDefault(key string, defaultValue string) string {
	if up.ExtraParams == nil {
		return defaultValue
	}

	value, exists := up.ExtraParams[key]
	if !exists {
		return defaultValue
	}

	return value
}
