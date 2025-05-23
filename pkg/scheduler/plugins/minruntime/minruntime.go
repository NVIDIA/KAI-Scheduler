// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package minruntime

import (
	"slices"
	"time"

	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/common_info"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/pod_info"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/podgroup_info"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/queue_info"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/framework"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/log"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	defaultReclaimMinRuntimeConfig = "defaultReclaimMinRuntime"
	defaultPreemptMinRuntimeConfig = "defaultPreemptMinRuntime"
	reclaimResolveMethod           = "reclaimResolveMethod"
	resolveMethodLCA               = "lca"
	resolveMethodQueue             = "queue"
)

type minruntimePlugin struct {
	defaultReclaimMinRuntime metav1.Duration
	defaultPreemptMinRuntime metav1.Duration
	reclaimResolveMethod     string
	queues                   map[common_info.QueueID]*queue_info.QueueInfo
	podGroupInfos            map[common_info.PodGroupID]*podgroup_info.PodGroupInfo

	preemptProtectionCache map[common_info.PodGroupID]map[common_info.PodGroupID]bool
	reclaimProtectionCache map[common_info.PodGroupID]map[common_info.PodGroupID]bool

	resolver *resolver
}

func parseMinRuntime(arguments map[string]string, minRuntimeConfig string) metav1.Duration {
	minRuntime := arguments[minRuntimeConfig]
	duration, err := time.ParseDuration(minRuntime)
	if err != nil {
		log.InfraLogger.Errorf("Failed to parse %v (%v): %v, using default value 0s", minRuntimeConfig, minRuntime, err)
		duration = 0 * time.Second
	}
	return metav1.Duration{Duration: duration}
}

func New(arguments map[string]string) framework.Plugin {
	plugin := &minruntimePlugin{}

	plugin.defaultReclaimMinRuntime = parseMinRuntime(arguments, defaultReclaimMinRuntimeConfig)
	plugin.defaultPreemptMinRuntime = parseMinRuntime(arguments, defaultPreemptMinRuntimeConfig)

	validResolveMethods := []string{resolveMethodLCA, resolveMethodQueue}
	plugin.reclaimResolveMethod = arguments[reclaimResolveMethod]
	if len(plugin.reclaimResolveMethod) == 0 || !slices.Contains(validResolveMethods, plugin.reclaimResolveMethod) {
		plugin.reclaimResolveMethod = resolveMethodLCA
	}
	// setup caches on plugin init, but they will be reset on session open anyway
	plugin.preemptProtectionCache = make(map[common_info.PodGroupID]map[common_info.PodGroupID]bool)
	plugin.reclaimProtectionCache = make(map[common_info.PodGroupID]map[common_info.PodGroupID]bool)

	return plugin
}

func (mr *minruntimePlugin) Name() string {
	return "minruntime"
}

func (mr *minruntimePlugin) OnSessionOpen(ssn *framework.Session) {
	ssn.AddReclaimVictimFilterFn(mr.reclaimFilterFn)
	ssn.AddPreemptVictimFilterFn(mr.preemptFilterFn)
	ssn.AddReclaimScenarioValidatorFn(mr.reclaimScenarioValidatorFn)
	ssn.AddPreemptScenarioValidatorFn(mr.preemptScenarioValidatorFn)
	// store session data used for lookups
	mr.queues = ssn.Queues
	mr.podGroupInfos = ssn.PodGroupInfos
	// setup caches on session open
	mr.preemptProtectionCache = make(map[common_info.PodGroupID]map[common_info.PodGroupID]bool)
	mr.reclaimProtectionCache = make(map[common_info.PodGroupID]map[common_info.PodGroupID]bool)

	// initialize the resolver
	mr.resolver = NewResolver(mr.queues, mr.defaultPreemptMinRuntime, mr.defaultReclaimMinRuntime)
}

func (mr *minruntimePlugin) OnSessionClose(ssn *framework.Session) {
	mr.queues = nil
	mr.preemptProtectionCache = nil
	mr.reclaimProtectionCache = nil
	mr.resolver = nil
}

func (mr *minruntimePlugin) reclaimFilterFn(pendingJob *podgroup_info.PodGroupInfo, victim *podgroup_info.PodGroupInfo) bool {
	protected := mr.isReclaimMinRuntimeProtected(pendingJob, victim)
	// always return true for elastic jobs, but cache the result for scenario validator
	if victim.IsElastic() {
		return true
	}

	return !protected
}

func (mr *minruntimePlugin) preemptFilterFn(pendingJob *podgroup_info.PodGroupInfo, victim *podgroup_info.PodGroupInfo) bool {
	protected := mr.isPreemptMinRuntimeProtected(pendingJob, victim)
	// always return true for elastic jobs, but cache the result for scenario validator
	if victim.IsElastic() {
		return true
	}
	return !protected
}

func (mr *minruntimePlugin) reclaimScenarioValidatorFn(reclaimer *podgroup_info.PodGroupInfo, victimRepresentatives []*podgroup_info.PodGroupInfo, tasks []*pod_info.PodInfo) bool {
	for _, victimRepresentative := range victimRepresentatives {
		victim, found := mr.podGroupInfos[victimRepresentative.UID]
		if !found {
			continue
		}
		if !victim.IsElastic() {
			continue
		}
		protected := mr.isReclaimMinRuntimeProtected(reclaimer, victim)
		if !protected {
			continue
		}
		numVictimTasks := getVictimCount(victimRepresentative, tasks)
		currentlyRunning := victim.GetActivelyRunningTasksCount()
		if victim.MinAvailable > currentlyRunning-numVictimTasks {
			return false
		}
	}
	return true
}

func (mr *minruntimePlugin) preemptScenarioValidatorFn(preemptor *podgroup_info.PodGroupInfo, victimRepresentatives []*podgroup_info.PodGroupInfo, tasks []*pod_info.PodInfo) bool {
	for _, victimRepresentative := range victimRepresentatives {
		victim, found := mr.podGroupInfos[victimRepresentative.UID]
		if !found {
			continue
		}
		if !victim.IsElastic() {
			continue
		}
		protected := mr.isPreemptMinRuntimeProtected(preemptor, victim)
		if !protected {
			continue
		}
		numVictimTasks := getVictimCount(victimRepresentative, tasks)
		currentlyRunning := victim.GetActivelyRunningTasksCount()
		if victim.MinAvailable > currentlyRunning-numVictimTasks {
			return false
		}
	}
	return true
}

func (mr *minruntimePlugin) isReclaimMinRuntimeProtected(pendingJob *podgroup_info.PodGroupInfo, victim *podgroup_info.PodGroupInfo) bool {
	if mr.reclaimProtectionCache[pendingJob.UID][victim.UID] {
		return true
	}
	// Get the appropriate queue objects for both jobs
	pendingQueue := mr.queues[pendingJob.Queue]
	victimQueue := mr.queues[victim.Queue]

	minRuntime, err := mr.resolver.getReclaimMinRuntime(mr.reclaimResolveMethod, pendingQueue, victimQueue)
	if err != nil {
		log.InfraLogger.Errorf("Failed to get reclaim min runtime: %v", err)
		minRuntime = mr.defaultReclaimMinRuntime
	}

	// If the victim's last start time plus minimum runtime is greater than current time,
	// the victim is protected from reclaim
	if victim.LastStartTimestamp != nil && !victim.LastStartTimestamp.IsZero() {
		protectedUntil := victim.LastStartTimestamp.Add(minRuntime.Duration)
		protected := time.Now().Before(protectedUntil)
		mr.cacheReclaimProtection(pendingJob, victim, protected)
		return protected
	}
	return false
}

func (mr *minruntimePlugin) isPreemptMinRuntimeProtected(pendingJob *podgroup_info.PodGroupInfo, victim *podgroup_info.PodGroupInfo) bool {
	if mr.preemptProtectionCache[pendingJob.UID][victim.UID] {
		return true
	}
	// Get the victim's queue
	victimQueue := mr.queues[victim.Queue]

	minRuntime, err := mr.resolver.getPreemptMinRuntime(victimQueue)
	if err != nil {
		log.InfraLogger.Errorf("Failed to get preempt min runtime: %v", err)
		minRuntime = mr.defaultPreemptMinRuntime
	}

	// If the victim's last start time plus minimum runtime is greater than current time,
	// the victim is protected from preemption
	if victim.LastStartTimestamp != nil && !victim.LastStartTimestamp.IsZero() {
		protectedUntil := victim.LastStartTimestamp.Add(minRuntime.Duration)
		protected := time.Now().Before(protectedUntil)
		mr.cachePreemptProtection(pendingJob, victim, protected)
		return protected
	}

	return false
}

func (mr *minruntimePlugin) cachePreemptProtection(pendingJob *podgroup_info.PodGroupInfo, victim *podgroup_info.PodGroupInfo, protected bool) {
	if mr.preemptProtectionCache[pendingJob.UID] == nil {
		mr.preemptProtectionCache[pendingJob.UID] = make(map[common_info.PodGroupID]bool)
	}
	mr.preemptProtectionCache[pendingJob.UID][victim.UID] = protected
}

func (mr *minruntimePlugin) cacheReclaimProtection(pendingJob *podgroup_info.PodGroupInfo, victim *podgroup_info.PodGroupInfo, protected bool) {
	if mr.reclaimProtectionCache[pendingJob.UID] == nil {
		mr.reclaimProtectionCache[pendingJob.UID] = make(map[common_info.PodGroupID]bool)
	}
	mr.reclaimProtectionCache[pendingJob.UID][victim.UID] = protected
}

func getVictimCount(victimRepresentative *podgroup_info.PodGroupInfo, victimTasks []*pod_info.PodInfo) int32 {
	victimCount := int32(0)
	for _, victimTask := range victimTasks {
		if victimTask.Job == victimRepresentative.UID {
			victimCount++
		}
	}
	return victimCount
}
