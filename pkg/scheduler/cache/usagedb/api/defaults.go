// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package api

import "time"

func (p *UsageParams) SetDefaults() {
	if p.HalfLifePeriod == nil {
		// noop: disabled by default
	}
	if p.WindowSize == nil {
		windowSize := time.Hour * 24 * 7
		p.WindowSize = &windowSize
	}
	if p.WindowType == nil {
		windowType := SlidingWindow
		p.WindowType = &windowType
	}
	if p.FetchInterval == nil {
		fetchInterval := 1 * time.Minute
		p.FetchInterval = &fetchInterval
	}
	if p.StalenessPeriod == nil {
		stalenessPeriod := 5 * time.Minute
		p.StalenessPeriod = &stalenessPeriod
	}
	if p.WaitTimeout == nil {
		waitTimeout := 1 * time.Minute
		p.WaitTimeout = &waitTimeout
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

func (p *UsageParams) GetExtraDurationParamOrDefault(key string, defaultValue time.Duration) time.Duration {
	if p.ExtraParams == nil {
		return defaultValue
	}

	value, exists := p.ExtraParams[key]
	if !exists {
		return defaultValue
	}

	duration, err := time.ParseDuration(value)
	if err != nil {
		return defaultValue
	}

	return duration
}

func (p *UsageParams) GetExtraStringParamOrDefault(key string, defaultValue string) string {
	if p.ExtraParams == nil {
		return defaultValue
	}

	value, exists := p.ExtraParams[key]
	if !exists {
		return defaultValue
	}

	return value
}
