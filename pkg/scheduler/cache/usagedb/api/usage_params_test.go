// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package api

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUsageParams_SetDefaults(t *testing.T) {
	tests := []struct {
		name     string
		input    *UsageParams
		expected *UsageParams
	}{
		{
			name:  "empty params should set defaults",
			input: &UsageParams{},
			expected: &UsageParams{
				HalfLifePeriod: nil, // should remain nil (disabled by default)
				WindowSize:     &[]time.Duration{time.Hour * 24 * 7}[0],
				WindowType:     &[]WindowType{SlidingWindow}[0],
			},
		},
		{
			name: "params with half life set should preserve it",
			input: &UsageParams{
				HalfLifePeriod: &[]time.Duration{30 * time.Minute}[0],
			},
			expected: &UsageParams{
				HalfLifePeriod: &[]time.Duration{30 * time.Minute}[0],
				WindowSize:     &[]time.Duration{time.Hour * 24 * 7}[0],
				WindowType:     &[]WindowType{SlidingWindow}[0],
			},
		},
		{
			name: "params with window size set should preserve it",
			input: &UsageParams{
				WindowSize: &[]time.Duration{2 * time.Hour}[0],
			},
			expected: &UsageParams{
				HalfLifePeriod: nil,
				WindowSize:     &[]time.Duration{2 * time.Hour}[0],
				WindowType:     &[]WindowType{SlidingWindow}[0],
			},
		},
		{
			name: "params with window type set should preserve it",
			input: &UsageParams{
				WindowType: &[]WindowType{TumblingWindow}[0],
			},
			expected: &UsageParams{
				HalfLifePeriod: nil,
				WindowSize:     &[]time.Duration{time.Hour * 24 * 7}[0],
				WindowType:     &[]WindowType{TumblingWindow}[0],
			},
		},
		{
			name: "all params set should preserve all",
			input: &UsageParams{
				HalfLifePeriod: &[]time.Duration{45 * time.Minute}[0],
				WindowSize:     &[]time.Duration{3 * time.Hour}[0],
				WindowType:     &[]WindowType{TumblingWindow}[0],
			},
			expected: &UsageParams{
				HalfLifePeriod: &[]time.Duration{45 * time.Minute}[0],
				WindowSize:     &[]time.Duration{3 * time.Hour}[0],
				WindowType:     &[]WindowType{TumblingWindow}[0],
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.input.SetDefaults()

			if tt.expected.HalfLifePeriod == nil {
				assert.Nil(t, tt.input.HalfLifePeriod)
			} else {
				require.NotNil(t, tt.input.HalfLifePeriod)
				assert.Equal(t, *tt.expected.HalfLifePeriod, *tt.input.HalfLifePeriod)
			}

			require.NotNil(t, tt.input.WindowSize)
			assert.Equal(t, *tt.expected.WindowSize, *tt.input.WindowSize)

			require.NotNil(t, tt.input.WindowType)
			assert.Equal(t, *tt.expected.WindowType, *tt.input.WindowType)
		})
	}
}

func TestUsageParams_GetWindowTypeOrDefault(t *testing.T) {
	tests := []struct {
		name     string
		input    *UsageParams
		expected WindowType
	}{
		{
			name:     "nil window type should return default",
			input:    &UsageParams{WindowType: nil},
			expected: SlidingWindow,
		},
		{
			name:     "sliding window type should return sliding",
			input:    &UsageParams{WindowType: &[]WindowType{SlidingWindow}[0]},
			expected: SlidingWindow,
		},
		{
			name:     "tumbling window type should return tumbling",
			input:    &UsageParams{WindowType: &[]WindowType{TumblingWindow}[0]},
			expected: TumblingWindow,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.input.GetWindowTypeOrDefault()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestUsageDBConfig_GetUsageParams(t *testing.T) {
	tests := []struct {
		name     string
		config   *UsageDBConfig
		expected *UsageParams
	}{
		{
			name: "nil usage params should return defaults",
			config: &UsageDBConfig{
				ClientType:       "prometheus",
				ConnectionString: "http://localhost:9090",
				UsageParams:      nil,
			},
			expected: &UsageParams{
				HalfLifePeriod: nil,
				WindowSize:     &[]time.Duration{time.Hour * 24 * 7}[0],
				WindowType:     &[]WindowType{SlidingWindow}[0],
			},
		},
		{
			name: "empty usage params should return defaults",
			config: &UsageDBConfig{
				ClientType:       "prometheus",
				ConnectionString: "http://localhost:9090",
				UsageParams:      &UsageParams{},
			},
			expected: &UsageParams{
				HalfLifePeriod: nil,
				WindowSize:     &[]time.Duration{time.Hour * 24 * 7}[0],
				WindowType:     &[]WindowType{SlidingWindow}[0],
			},
		},
		{
			name: "partial usage params should fill in defaults",
			config: &UsageDBConfig{
				ClientType:       "prometheus",
				ConnectionString: "http://localhost:9090",
				UsageParams: &UsageParams{
					HalfLifePeriod: &[]time.Duration{30 * time.Minute}[0],
				},
			},
			expected: &UsageParams{
				HalfLifePeriod: &[]time.Duration{30 * time.Minute}[0],
				WindowSize:     &[]time.Duration{time.Hour * 24 * 7}[0],
				WindowType:     &[]WindowType{SlidingWindow}[0],
			},
		},
		{
			name: "full usage params should be preserved",
			config: &UsageDBConfig{
				ClientType:       "prometheus",
				ConnectionString: "http://localhost:9090",
				UsageParams: &UsageParams{
					HalfLifePeriod: &[]time.Duration{45 * time.Minute}[0],
					WindowSize:     &[]time.Duration{2 * time.Hour}[0],
					WindowType:     &[]WindowType{TumblingWindow}[0],
				},
			},
			expected: &UsageParams{
				HalfLifePeriod: &[]time.Duration{45 * time.Minute}[0],
				WindowSize:     &[]time.Duration{2 * time.Hour}[0],
				WindowType:     &[]WindowType{TumblingWindow}[0],
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.config.GetUsageParams()
			require.NotNil(t, result)

			if tt.expected.HalfLifePeriod == nil {
				assert.Nil(t, result.HalfLifePeriod)
			} else {
				require.NotNil(t, result.HalfLifePeriod)
				assert.Equal(t, *tt.expected.HalfLifePeriod, *result.HalfLifePeriod)
			}

			require.NotNil(t, result.WindowSize)
			assert.Equal(t, *tt.expected.WindowSize, *result.WindowSize)

			require.NotNil(t, result.WindowType)
			assert.Equal(t, *tt.expected.WindowType, *result.WindowType)
		})
	}
}

func TestUsageDBConfig_GetUsageParams_ImmutableOriginal(t *testing.T) {
	// Test that GetUsageParams doesn't modify the original config
	originalParams := &UsageParams{
		HalfLifePeriod: &[]time.Duration{30 * time.Minute}[0],
	}

	config := &UsageDBConfig{
		ClientType:       "prometheus",
		ConnectionString: "http://localhost:9090",
		UsageParams:      originalParams,
	}

	result := config.GetUsageParams()

	// Modify the result
	newWindowSize := 5 * time.Hour
	result.WindowSize = &newWindowSize

	// Original should remain unchanged
	assert.Nil(t, originalParams.WindowSize)
	assert.Equal(t, 30*time.Minute, *originalParams.HalfLifePeriod)
}

func TestWindowType_IsValid(t *testing.T) {
	tests := []struct {
		name       string
		windowType WindowType
		expected   bool
	}{
		{
			name:       "tumbling window is valid",
			windowType: TumblingWindow,
			expected:   true,
		},
		{
			name:       "sliding window is valid",
			windowType: SlidingWindow,
			expected:   true,
		},
		{
			name:       "empty string is invalid",
			windowType: WindowType(""),
			expected:   false,
		},
		{
			name:       "random string is invalid",
			windowType: WindowType("invalid"),
			expected:   false,
		},
		{
			name:       "mixed case tumbling is invalid",
			windowType: WindowType("Tumbling"),
			expected:   false,
		},
		{
			name:       "mixed case sliding is invalid",
			windowType: WindowType("Sliding"),
			expected:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.windowType.IsValid()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetDefaultWindowType(t *testing.T) {
	result := GetDefaultWindowType()
	assert.Equal(t, SlidingWindow, result)
}

func TestWindowTypeConstants(t *testing.T) {
	// Test that constants have expected values
	assert.Equal(t, "tumbling", string(TumblingWindow))
	assert.Equal(t, "sliding", string(SlidingWindow))
}

func TestUsageParams_ZeroValues(t *testing.T) {
	// Test behavior with zero duration values
	zeroDuration := time.Duration(0)
	params := &UsageParams{
		HalfLifePeriod: &zeroDuration,
		WindowSize:     &zeroDuration,
	}

	params.SetDefaults()

	// Zero values should be preserved, not replaced with defaults
	require.NotNil(t, params.HalfLifePeriod)
	assert.Equal(t, time.Duration(0), *params.HalfLifePeriod)

	require.NotNil(t, params.WindowSize)
	assert.Equal(t, time.Duration(0), *params.WindowSize)

	require.NotNil(t, params.WindowType)
	assert.Equal(t, SlidingWindow, *params.WindowType)
}
