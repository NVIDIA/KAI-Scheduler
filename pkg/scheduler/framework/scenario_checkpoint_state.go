// Copyright 2026 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package framework

import (
	"crypto/sha256"
	"encoding/binary"
	"math"
	"sort"
	"time"

	"github.com/kai-scheduler/KAI-scheduler/pkg/scheduler/api/common_info"
	"github.com/kai-scheduler/KAI-scheduler/pkg/scheduler/api/node_info"
	"github.com/kai-scheduler/KAI-scheduler/pkg/scheduler/api/pod_info"
	"github.com/kai-scheduler/KAI-scheduler/pkg/scheduler/api/podgroup_info"
	"github.com/kai-scheduler/KAI-scheduler/pkg/scheduler/api/resource_info"
	"github.com/kai-scheduler/KAI-scheduler/pkg/scheduler/metrics"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type scenarioCheckpointSessionState struct {
	pods              []*pod_info.PodInfo
	podUniverseDigest [sha256.Size]byte
	stateDigest       [sha256.Size]byte
	policyDigest      [sha256.Size]byte
	podGroups         checkpointDigestAccumulator
	podEntities       checkpointDigestAccumulator
	nodeEntities      checkpointDigestAccumulator
}

type checkpointDigestAccumulator struct {
	sum   [sha256.Size]byte
	count uint64
}

// InitializeScenarioCheckpointState creates snapshot-scoped validation data once.
func (ssn *Session) InitializeScenarioCheckpointState() {
	if ssn == nil || ssn.ClusterInfo == nil {
		return
	}
	started := time.Now()
	state := &scenarioCheckpointSessionState{}
	state.pods = checkpointPods(ssn.ClusterInfo.PodGroupInfos)
	state.podUniverseDigest = digestPodUniverse(state.pods)
	state.podGroups, state.podEntities, state.nodeEntities = schedulingStateAccumulators(ssn)
	state.stateDigest = digestSchedulingStateAccumulators(state.podGroups, state.podEntities, state.nodeEntities)
	state.policyDigest = digestSchedulingPolicy(ssn)
	ssn.scenarioCheckpointState = state
	metrics.ObserveScenarioSearchStateDigest("snapshot", time.Since(started))
}

func (ssn *Session) ensureScenarioCheckpointState() *scenarioCheckpointSessionState {
	if ssn == nil {
		return nil
	}
	if ssn.scenarioCheckpointState == nil {
		ssn.InitializeScenarioCheckpointState()
	}
	return ssn.scenarioCheckpointState
}

// ScenarioCheckpointPods returns a shallow copy of the canonical UID-sorted table.
func (ssn *Session) ScenarioCheckpointPods() []*pod_info.PodInfo {
	state := ssn.ensureScenarioCheckpointState()
	if state == nil {
		return nil
	}
	return append([]*pod_info.PodInfo(nil), state.pods...)
}

func (ssn *Session) ScenarioCheckpointPodUniverseDigest() [sha256.Size]byte {
	state := ssn.ensureScenarioCheckpointState()
	if state == nil {
		return [sha256.Size]byte{}
	}
	return state.podUniverseDigest
}

func (ssn *Session) ScenarioCheckpointStateDigest() [sha256.Size]byte {
	state := ssn.ensureScenarioCheckpointState()
	if state == nil {
		return [sha256.Size]byte{}
	}
	return state.stateDigest
}

// UpdateScenarioCheckpointTaskAndNodeDigest applies one statement transition
// without traversing the session snapshot. Callers capture hashes before mutation.
func (ssn *Session) UpdateScenarioCheckpointTaskAndNodeDigest(oldPod [sha256.Size]byte, pod *pod_info.PodInfo, oldNode [sha256.Size]byte, node *node_info.NodeInfo) {
	if ssn == nil || ssn.scenarioCheckpointState == nil {
		return
	}
	started := time.Now()
	state := ssn.scenarioCheckpointState
	state.podEntities.remove(oldPod)
	state.podEntities.add(hashCheckpointPod(pod))
	state.nodeEntities.remove(oldNode)
	state.nodeEntities.add(hashCheckpointNode(node))
	state.stateDigest = digestSchedulingStateAccumulators(state.podGroups, state.podEntities, state.nodeEntities)
	metrics.ObserveScenarioSearchStateDigest("incremental", time.Since(started))
}

func (ssn *Session) ScenarioCheckpointPolicyDigest() [sha256.Size]byte {
	state := ssn.ensureScenarioCheckpointState()
	if state == nil {
		return [sha256.Size]byte{}
	}
	return state.policyDigest
}

func checkpointPods(jobs map[common_info.PodGroupID]*podgroup_info.PodGroupInfo) []*pod_info.PodInfo {
	byUID := make(map[common_info.PodID]*pod_info.PodInfo)
	for _, job := range jobs {
		if job == nil {
			continue
		}
		for uid, pod := range job.GetAllPodsMap() {
			if pod == nil || uid == "" {
				continue
			}
			if existing, found := byUID[uid]; !found || existing == pod {
				byUID[uid] = pod
			}
		}
	}
	pods := make([]*pod_info.PodInfo, 0, len(byUID))
	for _, pod := range byUID {
		pods = append(pods, pod)
	}
	sort.Slice(pods, func(i, j int) bool { return pods[i].UID < pods[j].UID })
	return pods
}

func digestPodUniverse(pods []*pod_info.PodInfo) [sha256.Size]byte {
	h := sha256.New()
	writeCheckpointString(h, "scenario-checkpoint-pod-universe-v1")
	writeCheckpointUint64(h, uint64(len(pods)))
	for _, pod := range pods {
		if pod == nil {
			writeCheckpointString(h, "")
			continue
		}
		writeCheckpointString(h, string(pod.UID))
	}
	return digestSum(h)
}

func digestSchedulingState(ssn *Session) [sha256.Size]byte {
	podGroups, pods, nodes := schedulingStateAccumulators(ssn)
	return digestSchedulingStateAccumulators(podGroups, pods, nodes)
}

func schedulingStateAccumulators(ssn *Session) (checkpointDigestAccumulator, checkpointDigestAccumulator, checkpointDigestAccumulator) {
	var podGroups, pods, nodes checkpointDigestAccumulator
	if ssn == nil || ssn.ClusterInfo == nil {
		return podGroups, pods, nodes
	}
	jobs := make([]*podgroup_info.PodGroupInfo, 0, len(ssn.ClusterInfo.PodGroupInfos))
	for _, job := range ssn.ClusterInfo.PodGroupInfos {
		jobs = append(jobs, job)
	}
	for _, job := range jobs {
		podGroups.add(hashCheckpointPodGroup(job))
	}
	for _, pod := range checkpointPods(ssn.ClusterInfo.PodGroupInfos) {
		pods.add(hashCheckpointPod(pod))
	}
	nodeInfos := make([]*node_info.NodeInfo, 0, len(ssn.ClusterInfo.Nodes))
	for _, node := range ssn.ClusterInfo.Nodes {
		nodeInfos = append(nodeInfos, node)
	}
	for _, node := range nodeInfos {
		nodes.add(hashCheckpointNode(node))
	}
	return podGroups, pods, nodes
}

func digestSchedulingStateAccumulators(podGroups, pods, nodes checkpointDigestAccumulator) [sha256.Size]byte {
	h := sha256.New()
	writeCheckpointString(h, "scenario-checkpoint-state-v2")
	for _, accumulator := range []checkpointDigestAccumulator{podGroups, pods, nodes} {
		writeCheckpointUint64(h, accumulator.count)
		writeCheckpointBytes(h, accumulator.sum[:])
	}
	return digestSum(h)
}

func (a *checkpointDigestAccumulator) add(value [sha256.Size]byte) {
	carry := uint16(0)
	for index := len(a.sum) - 1; index >= 0; index-- {
		total := uint16(a.sum[index]) + uint16(value[index]) + carry
		a.sum[index] = byte(total)
		carry = total >> 8
	}
	a.count++
}

func (a *checkpointDigestAccumulator) remove(value [sha256.Size]byte) {
	borrow := int16(0)
	for index := len(a.sum) - 1; index >= 0; index-- {
		total := int16(a.sum[index]) - int16(value[index]) - borrow
		if total < 0 {
			total += 256
			borrow = 1
		} else {
			borrow = 0
		}
		a.sum[index] = byte(total)
	}
	if a.count > 0 {
		a.count--
	}
}

func digestSchedulingPolicy(ssn *Session) [sha256.Size]byte {
	h := sha256.New()
	writeCheckpointString(h, "scenario-checkpoint-policy-v1")
	for _, registration := range ssn.ScenarioGeneratorRegistrations {
		writeCheckpointString(h, registration.Name)
	}
	if ssn.Config == nil {
		return digestSum(h)
	}
	for _, tier := range ssn.Config.Tiers {
		for _, plugin := range tier.Plugins {
			writeCheckpointString(h, plugin.Name)
			writeCheckpointBool(h, plugin.JobOrderDisabled)
			writeCheckpointBool(h, plugin.TaskOrderDisabled)
			writeCheckpointBool(h, plugin.PreemptableDisabled)
			writeCheckpointBool(h, plugin.ReclaimableDisabled)
			writeCheckpointBool(h, plugin.QueueOrderDisabled)
			writeCheckpointBool(h, plugin.PredicateDisabled)
			writeCheckpointBool(h, plugin.NodeOrderDisabled)
			keys := make([]string, 0, len(plugin.Arguments))
			for key := range plugin.Arguments {
				keys = append(keys, key)
			}
			sort.Strings(keys)
			for _, key := range keys {
				writeCheckpointString(h, key)
				writeCheckpointString(h, plugin.Arguments[key])
			}
		}
	}
	writeCheckpointString(h, "budgets")
	if ssn.Config.ScenarioSearchBudgets != nil {
		budgets := ssn.Config.ScenarioSearchBudgets
		writeCheckpointDurationMap(h, budgets.MaxActionSearchDuration)
		if budgets.MaxJobSearchDuration != nil {
			writeCheckpointInt64(h, int64(budgets.MaxJobSearchDuration.Duration))
		}
		if budgets.MinJobSearchDuration != nil {
			writeCheckpointInt64(h, int64(budgets.MinJobSearchDuration.Duration))
		}
		writeCheckpointDurationMap(h, budgets.MaxGeneratorSearchDuration)
	}
	return digestSum(h)
}

func hashCheckpointPodGroup(job *podgroup_info.PodGroupInfo) [sha256.Size]byte {
	h := sha256.New()
	writeCheckpointString(h, "podgroup-v1")
	if job == nil {
		return digestSum(h)
	}
	writeCheckpointString(h, string(job.UID))
	writeCheckpointString(h, string(job.Queue))
	writeCheckpointInt64(h, int64(job.Priority))
	writeCheckpointString(h, string(job.Preemptibility))
	pods := make([]*pod_info.PodInfo, 0, len(job.GetAllPodsMap()))
	for _, pod := range job.GetAllPodsMap() {
		pods = append(pods, pod)
	}
	sort.Slice(pods, func(i, j int) bool { return pods[i].UID < pods[j].UID })
	for _, pod := range pods {
		writeCheckpointString(h, string(pod.UID))
	}
	return digestSum(h)
}

func hashCheckpointPod(pod *pod_info.PodInfo) [sha256.Size]byte {
	h := sha256.New()
	writeCheckpointString(h, "pod-v1")
	if pod == nil {
		return digestSum(h)
	}
	writeCheckpointString(h, string(pod.UID))
	writeCheckpointString(h, string(pod.Job))
	writeCheckpointString(h, pod.SubGroupName)
	writeCheckpointString(h, pod.NodeName)
	writeCheckpointInt64(h, int64(pod.Status))
	writeCheckpointVector(h, pod.ResReqVector, pod.VectorMap)
	writeCheckpointGPURequirement(h, &pod.GpuRequirement)
	return digestSum(h)
}

func hashCheckpointNode(node *node_info.NodeInfo) [sha256.Size]byte {
	h := sha256.New()
	writeCheckpointString(h, "node-v1")
	if node == nil {
		return digestSum(h)
	}
	writeCheckpointString(h, node.Name)
	writeCheckpointVector(h, node.IdleVector, node.VectorMap)
	writeCheckpointVector(h, node.ReleasingVector, node.VectorMap)
	if node.Node != nil {
		writeCheckpointString(h, node.Node.ResourceVersion)
	}
	return digestSum(h)
}

func writeCheckpointVector(h interface{ Write([]byte) (int, error) }, vector resource_info.ResourceVector, vectorMap *resource_info.ResourceVectorMap) {
	type value struct {
		name   string
		amount float64
	}
	values := make([]value, 0, len(vector))
	for index, amount := range vector {
		name := ""
		if vectorMap != nil {
			name = string(vectorMap.ResourceAt(index))
		}
		values = append(values, value{name: name, amount: amount})
	}
	sort.Slice(values, func(i, j int) bool { return values[i].name < values[j].name })
	writeCheckpointUint64(h, uint64(len(values)))
	for _, value := range values {
		writeCheckpointString(h, value.name)
		writeCheckpointUint64(h, math.Float64bits(value.amount))
	}
}

func writeCheckpointGPURequirement(h interface{ Write([]byte) (int, error) }, requirement *resource_info.GpuResourceRequirement) {
	if requirement == nil {
		writeCheckpointBool(h, false)
		return
	}
	writeCheckpointBool(h, true)
	writeCheckpointInt64(h, requirement.GetNumOfGpuDevices())
	writeCheckpointUint64(h, math.Float64bits(requirement.GpuFractionalPortion()))
	writeCheckpointInt64(h, requirement.GpuMemory())
	writeCheckpointInt64Map(h, requirement.DraGpuCounts())
	mig := make(map[string]int64, len(requirement.MigResources()))
	for name, count := range requirement.MigResources() {
		mig[string(name)] = count
	}
	writeCheckpointInt64Map(h, mig)
}

func writeCheckpointDurationMap(h interface{ Write([]byte) (int, error) }, values map[string]metav1.Duration) {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	writeCheckpointUint64(h, uint64(len(keys)))
	for _, key := range keys {
		writeCheckpointString(h, key)
		writeCheckpointInt64(h, int64(values[key].Duration))
	}
}

func writeCheckpointInt64Map(h interface{ Write([]byte) (int, error) }, values map[string]int64) {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	writeCheckpointUint64(h, uint64(len(keys)))
	for _, key := range keys {
		writeCheckpointString(h, key)
		writeCheckpointInt64(h, values[key])
	}
}

func writeCheckpointString(h interface{ Write([]byte) (int, error) }, value string) {
	writeCheckpointBytes(h, []byte(value))
}
func writeCheckpointBytes(h interface{ Write([]byte) (int, error) }, value []byte) {
	writeCheckpointUint64(h, uint64(len(value)))
	_, _ = h.Write(value)
}
func writeCheckpointUint64(h interface{ Write([]byte) (int, error) }, value uint64) {
	var data [8]byte
	binary.BigEndian.PutUint64(data[:], value)
	_, _ = h.Write(data[:])
}
func writeCheckpointInt64(h interface{ Write([]byte) (int, error) }, value int64) {
	writeCheckpointUint64(h, uint64(value))
}
func writeCheckpointBool(h interface{ Write([]byte) (int, error) }, value bool) {
	if value {
		writeCheckpointUint64(h, 1)
	} else {
		writeCheckpointUint64(h, 0)
	}
}
func digestSum(h interface{ Sum([]byte) []byte }) [sha256.Size]byte {
	var result [sha256.Size]byte
	copy(result[:], h.Sum(nil))
	return result
}
