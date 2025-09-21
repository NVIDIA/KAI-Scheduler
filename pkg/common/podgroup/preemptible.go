package podgroup

import (
	"fmt"

	"github.com/NVIDIA/KAI-scheduler/pkg/apis/scheduling/v2alpha2"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/constants"
)

func IsPreemptible(preemptibility v2alpha2.Preemptibility, getPriority func() (int32, error)) (bool, error) {
	switch preemptibility {
	case v2alpha2.Preemptible:
		return true, nil
	case v2alpha2.NonPreemptible:
		return false, nil
	default:
		priority, err := getPriority()
		if err != nil {
			return false, fmt.Errorf("failed to get podgroup's priority: %w", err)
		}
		return priority < constants.PriorityBuildNumber, nil
	}
}
