// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package podgroup

import (
	"errors"
	"testing"

	"github.com/NVIDIA/KAI-scheduler/pkg/apis/scheduling/v2alpha2"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/constants"
	"github.com/stretchr/testify/assert"
)

func TestIsPreemptible(t *testing.T) {
	tests := []struct {
		name           string
		preemptibility v2alpha2.Preemptibility
		getPriority    func() (int32, error)
		expectedResult bool
		expectedError  bool
	}{
		{
			name:           "explicitly preemptible",
			preemptibility: v2alpha2.Preemptible,
			getPriority:    func() (int32, error) { return 1000, nil },
			expectedResult: true,
			expectedError:  false,
		},
		{
			name:           "explicitly non-preemptible",
			preemptibility: v2alpha2.NonPreemptible,
			getPriority:    func() (int32, error) { return 50, nil },
			expectedResult: false,
			expectedError:  false,
		},
		{
			name:           "unspecified with high priority (non-preemptible)",
			preemptibility: "",
			getPriority:    func() (int32, error) { return 1000, nil },
			expectedResult: false,
			expectedError:  false,
		},
		{
			name:           "unspecified with low priority (preemptible)",
			preemptibility: "",
			getPriority:    func() (int32, error) { return 50, nil },
			expectedResult: true,
			expectedError:  false,
		},
		{
			name:           "unspecified with priority equal to build number (non-preemptible)",
			preemptibility: "",
			getPriority:    func() (int32, error) { return constants.PriorityBuildNumber, nil },
			expectedResult: false,
			expectedError:  false,
		},
		{
			name:           "unspecified with priority just below build number (preemptible)",
			preemptibility: "",
			getPriority:    func() (int32, error) { return constants.PriorityBuildNumber - 1, nil },
			expectedResult: true,
			expectedError:  false,
		},
		{
			name:           "unspecified with priority just above build number (non-preemptible)",
			preemptibility: "",
			getPriority:    func() (int32, error) { return constants.PriorityBuildNumber + 1, nil },
			expectedResult: false,
			expectedError:  false,
		},
		{
			name:           "unspecified with priority getter error",
			preemptibility: "",
			getPriority:    func() (int32, error) { return 0, errors.New("priority lookup failed") },
			expectedResult: false,
			expectedError:  true,
		},
		{
			name:           "unspecified with zero priority (preemptible)",
			preemptibility: "",
			getPriority:    func() (int32, error) { return 0, nil },
			expectedResult: true,
			expectedError:  false,
		},
		{
			name:           "unspecified with negative priority (preemptible)",
			preemptibility: "",
			getPriority:    func() (int32, error) { return -100, nil },
			expectedResult: true,
			expectedError:  false,
		},
		{
			name:           "priority getter returns error with explicit preemptible",
			preemptibility: v2alpha2.Preemptible,
			getPriority:    func() (int32, error) { return 0, errors.New("priority lookup failed") },
			expectedResult: true,
			expectedError:  false,
		},
		{
			name:           "priority getter returns error with explicit non-preemptible",
			preemptibility: v2alpha2.NonPreemptible,
			getPriority:    func() (int32, error) { return 0, errors.New("priority lookup failed") },
			expectedResult: false,
			expectedError:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := IsPreemptible(tt.preemptibility, tt.getPriority)

			if tt.expectedError {
				assert.Error(t, err)
				assert.False(t, result)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedResult, result)
			}
		})
	}
}
