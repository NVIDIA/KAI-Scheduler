// Copyright 2026 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package framework

import (
	"testing"

	"github.com/kai-scheduler/KAI-scheduler/pkg/scheduler/api/common_info"
	"github.com/stretchr/testify/require"
)

func TestScenarioCheckpointStoreAdmissionAndCopies(t *testing.T) {
	store := NewScenarioCheckpointStore()
	require.NoError(t, store.SetMaxJobs(1))
	first := ScenarioCheckpointKey{Action: Reclaim, JobUID: common_info.PodGroupID("first")}
	second := ScenarioCheckpointKey{Action: Reclaim, JobUID: common_info.PodGroupID("second")}

	bitmap := []byte{1}
	require.Equal(t, ScenarioCheckpointCreated, store.Save(first, ScenarioCheckpoint{RecordedVictims: bitmap}))
	bitmap[0] = 2
	loaded, found := store.Load(first)
	require.True(t, found)
	require.Equal(t, []byte{1}, loaded.RecordedVictims)
	loaded.RecordedVictims[0] = 3
	loaded, found = store.Load(first)
	require.True(t, found)
	require.Equal(t, []byte{1}, loaded.RecordedVictims)

	require.Equal(t, ScenarioCheckpointUpdated, store.Save(first, ScenarioCheckpoint{RecordedVictims: []byte{4}}))
	require.Equal(t, ScenarioCheckpointRejectedCapacity, store.Save(second, ScenarioCheckpoint{}))
	require.Equal(t, 1, store.Len())
}

func TestScenarioCheckpointStoreCapacityAndSweep(t *testing.T) {
	store := NewScenarioCheckpointStore()
	first := ScenarioCheckpointKey{Action: Reclaim, JobUID: common_info.PodGroupID("first")}
	second := ScenarioCheckpointKey{Action: Reclaim, JobUID: common_info.PodGroupID("second")}
	require.Equal(t, ScenarioCheckpointCreated, store.Save(first, ScenarioCheckpoint{RecordedVictims: []byte{1}}))
	require.Equal(t, ScenarioCheckpointCreated, store.Save(second, ScenarioCheckpoint{RecordedVictims: []byte{1}}))
	require.NoError(t, store.SetMaxJobs(1))
	require.Equal(t, 2, store.Len())
	require.Equal(t, ScenarioCheckpointRejectedCapacity, store.Save(ScenarioCheckpointKey{Action: Reclaim, JobUID: "third"}, ScenarioCheckpoint{}))
	require.Equal(t, 1, store.Sweep(map[common_info.PodGroupID]struct{}{first.JobUID: {}}))
	require.Equal(t, 1, store.Len())
	require.NoError(t, store.SetMaxJobs(0))
	require.Equal(t, 0, store.Len())
	require.Equal(t, 0, store.BitmapBytes())
}

func TestScenarioCheckpointStoreRejectsBitmapOverProcessLimit(t *testing.T) {
	store := NewScenarioCheckpointStore()
	bitmap := make([]byte, maxScenarioCheckpointBitmapBytes+1)
	key := ScenarioCheckpointKey{Action: Reclaim, JobUID: "large"}
	require.Equal(t, ScenarioCheckpointRejectedMemory, store.Save(key, ScenarioCheckpoint{RecordedVictims: bitmap}))
	require.Equal(t, 0, store.Len())
}
