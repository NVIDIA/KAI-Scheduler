// Copyright 2026 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package framework

import (
	"fmt"
	"sync"

	"github.com/kai-scheduler/KAI-scheduler/pkg/scheduler/api/common_info"
	"github.com/kai-scheduler/KAI-scheduler/pkg/scheduler/metrics"
)

const (
	DefaultScenarioCheckpointMaxJobs = 32
	MaxScenarioCheckpointJobs        = 4096
	maxScenarioCheckpointBitmapBytes = 4 << 20
	ScenarioGeneratorCursorDataSize  = 32
)

type ScenarioCheckpointKey struct {
	Action ActionType
	JobUID common_info.PodGroupID
}

type ScenarioGeneratorCursor struct {
	Version uint16
	Data    [ScenarioGeneratorCursorDataSize]byte
}

type ResumableScenarioGenerator interface {
	ScenarioGenerator
	Cursor() (ScenarioGeneratorCursor, bool)
	Restore(ScenarioGeneratorCursor) error
}

type JobSolverCursor struct {
	Phase  uint8
	Lo     uint32
	Hi     uint32
	ProbeK uint32
}

// ScenarioCheckpoint deliberately retains no scheduler objects.
type ScenarioCheckpoint struct {
	InputFingerprint       [32]byte
	PodUniverseFingerprint [32]byte
	GeneratorName          string
	GeneratorCursor        ScenarioGeneratorCursor
	// StateOnly resumes at a fresh next-probe generator without Restore.
	StateOnly       bool
	SolverCursor    JobSolverCursor
	RecordedVictims []byte
	StopReason      string
}

type ScenarioCheckpointSaveResult uint8

const (
	ScenarioCheckpointCreated ScenarioCheckpointSaveResult = iota
	ScenarioCheckpointUpdated
	ScenarioCheckpointRejectedCapacity
	ScenarioCheckpointRejectedMemory
)

// ScenarioCheckpointStore keeps bounded process-local checkpoint state.
type ScenarioCheckpointStore struct {
	mu          sync.Mutex
	maxJobs     int
	bitmapBytes int
	entries     map[ScenarioCheckpointKey]ScenarioCheckpoint
}

func NewScenarioCheckpointStore() *ScenarioCheckpointStore {
	return &ScenarioCheckpointStore{
		maxJobs: DefaultScenarioCheckpointMaxJobs,
		entries: make(map[ScenarioCheckpointKey]ScenarioCheckpoint),
	}
}

// SetMaxJobs changes future admission. Zero clears and disables the store; lowering a
// positive limit deliberately preserves admitted entries.
func (s *ScenarioCheckpointStore) SetMaxJobs(maxJobs int) error {
	if s == nil {
		return nil
	}
	if maxJobs < 0 || maxJobs > MaxScenarioCheckpointJobs {
		return fmt.Errorf("scenario checkpoint maxJobs must be between 0 and %d", MaxScenarioCheckpointJobs)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.maxJobs = maxJobs
	if maxJobs == 0 {
		s.entries = make(map[ScenarioCheckpointKey]ScenarioCheckpoint)
		s.bitmapBytes = 0
	}
	s.observeLocked("configure", "ok")
	return nil
}

func (s *ScenarioCheckpointStore) Load(key ScenarioCheckpointKey) (ScenarioCheckpoint, bool) {
	if s == nil {
		return ScenarioCheckpoint{}, false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	checkpoint, found := s.entries[key]
	if !found {
		metrics.IncScenarioSearchCheckpoint("load", "miss")
		return ScenarioCheckpoint{}, false
	}
	checkpoint.RecordedVictims = append([]byte(nil), checkpoint.RecordedVictims...)
	metrics.IncScenarioSearchCheckpoint("load", "hit")
	return checkpoint, true
}

func (s *ScenarioCheckpointStore) Save(key ScenarioCheckpointKey, checkpoint ScenarioCheckpoint) ScenarioCheckpointSaveResult {
	if s == nil {
		return ScenarioCheckpointRejectedCapacity
	}
	checkpoint.RecordedVictims = append([]byte(nil), checkpoint.RecordedVictims...)
	s.mu.Lock()
	defer s.mu.Unlock()
	previous, found := s.entries[key]
	if !found && (s.maxJobs == 0 || len(s.entries) >= s.maxJobs) {
		metrics.IncScenarioSearchCheckpoint("save", "rejected_capacity")
		return ScenarioCheckpointRejectedCapacity
	}
	delta := len(checkpoint.RecordedVictims)
	if found {
		delta -= len(previous.RecordedVictims)
	}
	if delta > 0 && s.bitmapBytes > maxScenarioCheckpointBitmapBytes-delta {
		metrics.IncScenarioSearchCheckpoint("save", "rejected_memory")
		return ScenarioCheckpointRejectedMemory
	}
	s.bitmapBytes += delta
	s.entries[key] = checkpoint
	if found {
		s.observeLocked("save", "updated")
		return ScenarioCheckpointUpdated
	}
	s.observeLocked("save", "created")
	return ScenarioCheckpointCreated
}

func (s *ScenarioCheckpointStore) Delete(key ScenarioCheckpointKey) bool {
	if s == nil {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	checkpoint, found := s.entries[key]
	if !found {
		return false
	}
	delete(s.entries, key)
	s.bitmapBytes -= len(checkpoint.RecordedVictims)
	s.observeLocked("delete", "deleted")
	return true
}

// Sweep removes entries whose jobs are absent from a newly opened snapshot.
func (s *ScenarioCheckpointStore) Sweep(liveJobs map[common_info.PodGroupID]struct{}) int {
	if s == nil {
		return 0
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	removed := 0
	for key, checkpoint := range s.entries {
		if _, found := liveJobs[key.JobUID]; found {
			continue
		}
		delete(s.entries, key)
		s.bitmapBytes -= len(checkpoint.RecordedVictims)
		removed++
	}
	if removed > 0 {
		s.observeLocked("sweep", "deleted")
	}
	return removed
}

func (s *ScenarioCheckpointStore) observeLocked(operation, result string) {
	metrics.IncScenarioSearchCheckpoint(operation, result)
	metrics.SetScenarioSearchCheckpointStore(len(s.entries), s.bitmapBytes)
}

func (s *ScenarioCheckpointStore) Len() int {
	if s == nil {
		return 0
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.entries)
}

func (s *ScenarioCheckpointStore) BitmapBytes() int {
	if s == nil {
		return 0
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.bitmapBytes
}
