// Copyright 2026 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package solvers

import (
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"sort"
	"time"

	"github.com/kai-scheduler/KAI-scheduler/pkg/scheduler/api/common_info"
	"github.com/kai-scheduler/KAI-scheduler/pkg/scheduler/api/node_info"
	"github.com/kai-scheduler/KAI-scheduler/pkg/scheduler/api/pod_info"
	"github.com/kai-scheduler/KAI-scheduler/pkg/scheduler/api/podgroup_info"
	"github.com/kai-scheduler/KAI-scheduler/pkg/scheduler/framework"
	"github.com/kai-scheduler/KAI-scheduler/pkg/scheduler/metrics"
)

func checkpointKey(action framework.ActionType, job *podgroup_info.PodGroupInfo) framework.ScenarioCheckpointKey {
	if job == nil {
		return framework.ScenarioCheckpointKey{}
	}
	return framework.ScenarioCheckpointKey{Action: action, JobUID: job.UID}
}

func checkpointsEnabled(ssn *framework.Session, action framework.ActionType) bool {
	return action == framework.Reclaim && ssn != nil && ssn.ScenarioCheckpointStore != nil
}

func checkpointInputFingerprint(ctx *SolveContext, baseFeasible map[string]*node_info.NodeInfo, generatorName string, cursorVersion uint16) [sha256.Size]byte {
	h := sha256.New()
	checkpointWriteString(h, "scenario-checkpoint-input-v1")
	if ctx == nil || ctx.Session == nil {
		return checkpointDigest(h)
	}
	checkpointWriteString(h, string(ctx.ActionType))
	state := ctx.Session.ScenarioCheckpointStateDigest()
	checkpointWriteBytes(h, state[:])
	policy := ctx.Session.ScenarioCheckpointPolicyDigest()
	checkpointWriteBytes(h, policy[:])
	universe := ctx.Session.ScenarioCheckpointPodUniverseDigest()
	checkpointWriteBytes(h, universe[:])
	checkpointWriteUint64(h, uint64(ctx.ProbeK))
	checkpointWritePartialJob(h, ctx.PartialPendingJob)
	names := make([]string, 0, len(baseFeasible))
	for name := range baseFeasible {
		names = append(names, name)
	}
	sort.Strings(names)
	checkpointWriteUint64(h, uint64(len(names)))
	for _, name := range names {
		checkpointWriteString(h, name)
	}
	checkpointWriteString(h, generatorName)
	checkpointWriteUint64(h, uint64(cursorVersion))
	return checkpointDigest(h)
}

func checkpointWritePartialJob(h interface{ Write([]byte) (int, error) }, job *podgroup_info.PodGroupInfo) {
	if job == nil {
		checkpointWriteString(h, "")
		return
	}
	checkpointWriteString(h, string(job.UID))
	pods := make([]*pod_info.PodInfo, 0, len(job.GetAllPodsMap()))
	for _, pod := range job.GetAllPodsMap() {
		pods = append(pods, pod)
	}
	sort.Slice(pods, func(i, j int) bool { return pods[i].UID < pods[j].UID })
	checkpointWriteUint64(h, uint64(len(pods)))
	for _, pod := range pods {
		checkpointWriteString(h, string(pod.UID))
	}
}

func checkpointWriteString(h interface{ Write([]byte) (int, error) }, value string) {
	checkpointWriteBytes(h, []byte(value))
}
func checkpointWriteBytes(h interface{ Write([]byte) (int, error) }, value []byte) {
	checkpointWriteUint64(h, uint64(len(value)))
	_, _ = h.Write(value)
}
func checkpointWriteUint64(h interface{ Write([]byte) (int, error) }, value uint64) {
	var data [8]byte
	binary.BigEndian.PutUint64(data[:], value)
	_, _ = h.Write(data[:])
}
func checkpointDigest(h interface{ Sum([]byte) []byte }) [sha256.Size]byte {
	var result [sha256.Size]byte
	copy(result[:], h.Sum(nil))
	return result
}

func encodeRecordedVictims(ssn *framework.Session, victims []*pod_info.PodInfo) ([]byte, error) {
	pods := ssn.ScenarioCheckpointPods()
	bitmap := make([]byte, (len(pods)+7)/8)
	for _, victim := range victims {
		if victim == nil {
			return nil, fmt.Errorf("nil recorded victim")
		}
		ordinal := sort.Search(len(pods), func(index int) bool { return pods[index].UID >= victim.UID })
		if ordinal == len(pods) || pods[ordinal].UID != victim.UID {
			return nil, fmt.Errorf("recorded victim %q is absent from Pod table", victim.UID)
		}
		bitmap[ordinal/8] |= 1 << uint(ordinal%8)
	}
	return bitmap, nil
}

func decodeRecordedVictims(ssn *framework.Session, bitmap []byte) ([]*pod_info.PodInfo, []*podgroup_info.PodGroupInfo, error) {
	pods := ssn.ScenarioCheckpointPods()
	expectedBytes := (len(pods) + 7) / 8
	if len(bitmap) != expectedBytes {
		return nil, nil, fmt.Errorf("recorded victim bitmap has length %d, want %d", len(bitmap), expectedBytes)
	}
	if remainder := len(pods) % 8; remainder != 0 && len(bitmap) > 0 && bitmap[len(bitmap)-1]&^byte((1<<uint(remainder))-1) != 0 {
		return nil, nil, fmt.Errorf("recorded victim bitmap has trailing bits")
	}
	byJob := make(map[common_info.PodGroupID][]*pod_info.PodInfo)
	for ordinal, pod := range pods {
		if bitmap[ordinal/8]&(1<<uint(ordinal%8)) == 0 {
			continue
		}
		if pod == nil || pod.Job == "" {
			return nil, nil, fmt.Errorf("recorded victim at ordinal %d is invalid", ordinal)
		}
		job := ssn.ClusterInfo.PodGroupInfos[pod.Job]
		if job == nil {
			return nil, nil, fmt.Errorf("recorded victim job %q is absent", pod.Job)
		}
		if _, found := job.GetAllPodsMap()[pod.UID]; !found {
			return nil, nil, fmt.Errorf("recorded victim %q ownership is inconsistent", pod.UID)
		}
		byJob[pod.Job] = append(byJob[pod.Job], pod)
	}
	jobs := make([]common_info.PodGroupID, 0, len(byJob))
	for jobID := range byJob {
		jobs = append(jobs, jobID)
	}
	sort.Slice(jobs, func(i, j int) bool { return jobs[i] < jobs[j] })
	var victims []*pod_info.PodInfo
	representatives := make([]*podgroup_info.PodGroupInfo, 0, len(jobs))
	for _, jobID := range jobs {
		tasks := byJob[jobID]
		sort.Slice(tasks, func(i, j int) bool { return tasks[i].UID < tasks[j].UID })
		victims = append(victims, tasks...)
		representative := ssn.ClusterInfo.PodGroupInfos[jobID].CloneWithTasks(tasks)
		adjustSubGroupsMinAvailable(representative)
		adjustSubGroupsMinSubGroup(representative.RootSubGroupSet)
		representatives = append(representatives, representative)
	}
	return victims, representatives, nil
}

func loadScenarioCheckpoint(ctx *SolveContext, baseFeasible map[string]*node_info.NodeInfo, generatorName string) (*framework.ScenarioCheckpoint, error) {
	started := time.Now()
	result := "miss"
	defer func() { metrics.ObserveScenarioSearchCheckpointValidation(result, time.Since(started)) }()
	if ctx == nil || !checkpointsEnabled(ctx.Session, ctx.ActionType) || ctx.PartialPendingJob == nil {
		return nil, nil
	}
	key := checkpointKey(ctx.ActionType, ctx.PartialPendingJob)
	checkpoint, found := ctx.Session.ScenarioCheckpointStore.Load(key)
	if !found {
		return nil, nil
	}
	if checkpoint.GeneratorName != generatorName || int(checkpoint.SolverCursor.ProbeK) != ctx.ProbeK {
		result = "other_generator"
		return nil, nil
	}
	if checkpoint.GeneratorCursor.Version == 0 || checkpoint.InputFingerprint != checkpointInputFingerprint(ctx, baseFeasible, generatorName, checkpoint.GeneratorCursor.Version) || checkpoint.PodUniverseFingerprint != ctx.Session.ScenarioCheckpointPodUniverseDigest() {
		ctx.Session.ScenarioCheckpointStore.Delete(key)
		result = "invalid"
		return nil, fmt.Errorf("scenario checkpoint validation failed")
	}
	victims, jobs, err := decodeRecordedVictims(ctx.Session, checkpoint.RecordedVictims)
	if err != nil {
		ctx.Session.ScenarioCheckpointStore.Delete(key)
		result = "invalid"
		return nil, err
	}
	ctx.RecordedVictimsTasks = victims
	ctx.RecordedVictimsJobs = jobs
	result = "hit"
	return &checkpoint, nil
}

func saveScenarioCheckpoint(ctx *SolveContext, baseFeasible map[string]*node_info.NodeInfo, generatorName string, cursor framework.ScenarioGeneratorCursor, victims []*pod_info.PodInfo, stopReason SearchResultReason) {
	if ctx == nil || !checkpointsEnabled(ctx.Session, ctx.ActionType) || cursor.Version == 0 {
		return
	}
	bitmap, err := encodeRecordedVictims(ctx.Session, victims)
	if err != nil {
		return
	}
	ctx.Session.ScenarioCheckpointStore.Save(checkpointKey(ctx.ActionType, ctx.PartialPendingJob), framework.ScenarioCheckpoint{
		InputFingerprint:       checkpointInputFingerprint(ctx, baseFeasible, generatorName, cursor.Version),
		PodUniverseFingerprint: ctx.Session.ScenarioCheckpointPodUniverseDigest(),
		GeneratorName:          generatorName,
		GeneratorCursor:        cursor,
		SolverCursor:           framework.JobSolverCursor{ProbeK: uint32(ctx.ProbeK)},
		RecordedVictims:        bitmap,
		StopReason:             string(stopReason),
	})
}

func deleteScenarioCheckpoint(ctx *SolveContext) {
	if ctx != nil && checkpointsEnabled(ctx.Session, ctx.ActionType) && ctx.PartialPendingJob != nil {
		ctx.Session.ScenarioCheckpointStore.Delete(checkpointKey(ctx.ActionType, ctx.PartialPendingJob))
	}
}
