// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package podgroup

import (
	"fmt"

	"github.com/NVIDIA/KAI-scheduler/pkg/apis/scheduling/v2alpha2"
)

const nonPreemptiblePriorityThreshold = 100

// CalculatePreemptibility computes the preemptibility of a podgroup.
// When preemptibility is not explicitly specified, the determination is based on the podgroup's priority.
func CalculatePreemptibility(preemptibility v2alpha2.Preemptibility, getPriority func() (int32, error)) (v2alpha2.Preemptibility, error) {
	switch preemptibility {
	case v2alpha2.Preemptible:
		return v2alpha2.Preemptible, nil
	case v2alpha2.NonPreemptible:
		return v2alpha2.NonPreemptible, nil
	}

	priority, err := getPriority()
	if err != nil {
		return "", fmt.Errorf("failed to get podgroup's priority: %w", err)
	}

	if priority < nonPreemptiblePriorityThreshold {
		return v2alpha2.Preemptible, nil
	}
	return v2alpha2.NonPreemptible, nil
}
