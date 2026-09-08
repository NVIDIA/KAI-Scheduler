// Copyright 2026 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package framework

import (
	"fmt"
	"sync"
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

func TestScenarioCheckpointStoreDefaultLimitAndConcurrentUpdates(t *testing.T) {
	store := NewScenarioCheckpointStore()
	for index := 0; index < DefaultScenarioCheckpointMaxJobs; index++ {
		key := ScenarioCheckpointKey{Action: Reclaim, JobUID: common_info.PodGroupID(fmt.Sprintf("job-%d", index))}
		require.Equal(t, ScenarioCheckpointCreated, store.Save(key, ScenarioCheckpoint{RecordedVictims: []byte{byte(index)}}))
	}
	require.Equal(t, ScenarioCheckpointRejectedCapacity, store.Save(ScenarioCheckpointKey{Action: Reclaim, JobUID: "overflow"}, ScenarioCheckpoint{}))

	key := ScenarioCheckpointKey{Action: Reclaim, JobUID: "job-0"}
	var workers sync.WaitGroup
	for worker := 0; worker < 8; worker++ {
		workers.Add(1)
		go func(value byte) {
			defer workers.Done()
			for iteration := 0; iteration < 100; iteration++ {
				store.Save(key, ScenarioCheckpoint{RecordedVictims: []byte{value}})
				store.Load(key)
			}
		}(byte(worker))
	}
	workers.Wait()
	require.Equal(t, DefaultScenarioCheckpointMaxJobs, store.Len())
}

func BenchmarkScenarioCheckpointStoreSaveLoad(b *testing.B) {
	for _, podCount := range []int{16_000, 32_000} {
		b.Run(fmt.Sprintf("pods=%d", podCount), func(b *testing.B) {
			store := NewScenarioCheckpointStore()
			key := ScenarioCheckpointKey{Action: Reclaim, JobUID: "benchmark"}
			bitmap := make([]byte, (podCount+7)/8)
			b.SetBytes(int64(len(bitmap)))
			b.ReportAllocs()
			b.ResetTimer()
			for index := 0; index < b.N; index++ {
				store.Save(key, ScenarioCheckpoint{RecordedVictims: bitmap})
				store.Load(key)
			}
		})
	}
}
