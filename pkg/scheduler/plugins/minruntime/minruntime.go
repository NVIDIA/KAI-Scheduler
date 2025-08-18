// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package minruntime

import (
	"github.com/xhit/go-str2duration/v2"
	"slices"
	"time"

	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/common_info"
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

	preemptProtectionCache map[common_info.PodGroupID]bool
	reclaimProtectionCache map[common_info.PodGroupID]map[common_info.PodGroupID]bool

	resolver *resolver
}

func parseMinRuntime(arguments map[string]string, minRuntimeConfig string) metav1.Duration {
	minRuntime := arguments[minRuntimeConfig]
	if len(minRuntime) == 0 {
		return metav1.Duration{Duration: 0 * time.Second}
	}
	duration, err := str2duration.ParseDuration(minRuntime)
	if err != nil {
		log.InfraLogger.Errorf("Failed to parse %v (%v): %v, using default value 0s", minRuntimeConfig, minRuntime, err)
		duration = 0 * time.Second
	}

	if duration < 0 {
		log.InfraLogger.Errorf("Parsed %v (%v) is negative, using default value 0s", minRuntimeConfig, minRuntime)
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
	if len(plugin.reclaimResolveMethod) == 0 {
		plugin.reclaimResolveMethod = resolveMethodLCA
	}
	if !slices.Contains(validResolveMethods, plugin.reclaimResolveMethod) {
		log.InfraLogger.Errorf("Invalid reclaim resolve method %v, using default value %v", plugin.reclaimResolveMethod, resolveMethodLCA)
		plugin.reclaimResolveMethod = resolveMethodLCA
	}
	// setup caches on plugin init, but they will be reset on session open anyway
	plugin.preemptProtectionCache = make(map[common_info.PodGroupID]bool)
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
	mr.queues = ssn.Queues
	mr.preemptProtectionCache = make(map[common_info.PodGroupID]bool)
	mr.reclaimProtectionCache = make(map[common_info.PodGroupID]map[common_info.PodGroupID]bool)
	mr.resolver = NewResolver(mr.queues, mr.defaultPreemptMinRuntime, mr.defaultReclaimMinRuntime)
}

func (mr *minruntimePlugin) OnSessionClose(ssn *framework.Session) {
	mr.queues = nil
	mr.preemptProtectionCache = nil
	mr.reclaimProtectionCache = nil
	mr.resolver = nil
}

func (mr *minruntimePlugin) reclaimFilterFn(pendingJob *podgroup_info.PodGroupInfo, victim *podgroup_info.PodGroupInfo) bool {
	// always return true for elastic jobs, they are checked in scenario validator
	if victim.IsElastic() {
		return true
	}
	return !mr.isReclaimMinRuntimeProtected(pendingJob, victim)
}

func (mr *minruntimePlugin) preemptFilterFn(pendingJob *podgroup_info.PodGroupInfo, victim *podgroup_info.PodGroupInfo) bool {
	// always return true for elastic jobs, they are checked in scenario validator
	if victim.IsElastic() {
		return true
	}
	return !mr.isPreemptMinRuntimeProtected(pendingJob, victim)
}

func (mr *minruntimePlugin) reclaimScenarioValidatorFn(scenario api.ScenarioInfo) bool {
	reclaimer := scenario.GetPreemptor()
	for _, victimInfo := range scenario.GetVictims() {
		if !victimInfo.Job.IsElastic() {
			continue
		}
		protected := mr.isReclaimMinRuntimeProtected(reclaimer, victimInfo.Job)
		if !protected {
			continue
		}
		numVictimTasks := int32(len(victimInfo.Tasks))
		currentlyRunning := victimInfo.Job.GetActivelyRunningTasksCount()
		if victimInfo.Job.GetDefaultMinAvailable() > currentlyRunning-numVictimTasks {
			return false
		}
	}
	return true
}

func (mr *minruntimePlugin) preemptScenarioValidatorFn(scenario api.ScenarioInfo) bool {
	preemptor := scenario.GetPreemptor()
	for _, victimInfo := range scenario.GetVictims() {
		if !victimInfo.Job.IsElastic() {
			continue
		}
		protected := mr.isPreemptMinRuntimeProtected(preemptor, victimInfo.Job)
		if !protected {
			continue
		}
		numVictimTasks := int32(len(victimInfo.Tasks))
		currentlyRunning := victimInfo.Job.GetActivelyRunningTasksCount()
		if victimInfo.Job.GetDefaultMinAvailable() > currentlyRunning-numVictimTasks {
			return false
		}
	}
	return true
}

func (mr *minruntimePlugin) isReclaimMinRuntimeProtected(pendingJob *podgroup_info.PodGroupInfo, victim *podgroup_info.PodGroupInfo) bool {
	if cached, ok := mr.reclaimProtectionCache[pendingJob.UID][victim.UID]; ok {
		return cached
	}
	pendingQueue := mr.queues[pendingJob.Queue]
	victimQueue := mr.queues[victim.Queue]

	minRuntime, err := mr.resolver.getReclaimMinRuntime(mr.reclaimResolveMethod, pendingQueue, victimQueue)
	if err != nil {
		log.InfraLogger.Errorf("Failed to get reclaim min runtime for pending job %v and victim %v: %v", pendingJob.NamespacedName, victim.NamespacedName, err)
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

func (mr *minruntimePlugin) isPreemptMinRuntimeProtected(_ *podgroup_info.PodGroupInfo, victim *podgroup_info.PodGroupInfo) bool {
	if cached, ok := mr.preemptProtectionCache[victim.UID]; ok {
		return cached
	}
	victimQueue := mr.queues[victim.Queue]

	minRuntime, err := mr.resolver.getPreemptMinRuntime(victimQueue)
	if err != nil {
		log.InfraLogger.Errorf("Failed to get preempt min runtime for victim %v: %v", victim.NamespacedName, err)
		minRuntime = mr.defaultPreemptMinRuntime
	}

	// If the victim's last start time plus minimum runtime is greater than current time,
	// the victim is protected from preemption
	if victim.LastStartTimestamp != nil && !victim.LastStartTimestamp.IsZero() {
		protectedUntil := victim.LastStartTimestamp.Add(minRuntime.Duration)
		protected := time.Now().Before(protectedUntil)
		mr.cachePreemptProtection(victim, protected)
		return protected
	}

	return false
}

func (mr *minruntimePlugin) cachePreemptProtection(victim *podgroup_info.PodGroupInfo, protected bool) {
	mr.preemptProtectionCache[victim.UID] = protected
}

func (mr *minruntimePlugin) cacheReclaimProtection(pendingJob *podgroup_info.PodGroupInfo, victim *podgroup_info.PodGroupInfo, protected bool) {
	if mr.reclaimProtectionCache[pendingJob.UID] == nil {
		mr.reclaimProtectionCache[pendingJob.UID] = make(map[common_info.PodGroupID]bool)
	}
	mr.reclaimProtectionCache[pendingJob.UID][victim.UID] = protected
}
