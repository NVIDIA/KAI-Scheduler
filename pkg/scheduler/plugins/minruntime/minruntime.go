package minruntime

import (
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
	defaultReclaimMinRuntime *metav1.Duration
	defaultPreemptMinRuntime *metav1.Duration
	reclaimResolveMethod     string
	queues                   map[common_info.QueueID]*queue_info.QueueInfo

	preemptMinRuntimeCache map[common_info.QueueID]metav1.Duration
	reclaimMinRuntimeCache map[common_info.QueueID]map[common_info.QueueID]metav1.Duration

	preemptProtectionCache map[common_info.PodGroupID]map[common_info.PodGroupID]bool
	reclaimProtectionCache map[common_info.PodGroupID]map[common_info.PodGroupID]bool
}

func New(arguments map[string]string) framework.Plugin {
	plugin := &minruntimePlugin{}
	for key, value := range arguments {
		switch key {
		case defaultReclaimMinRuntimeConfig:
			duration, err := time.ParseDuration(value)
			if err != nil {
				log.InfraLogger.Errorf("Failed to parse %v: %v, using default value 0", key, err)
				duration = time.Duration(0 * time.Second)
			}
			plugin.defaultReclaimMinRuntime = &metav1.Duration{Duration: duration}
		case defaultPreemptMinRuntimeConfig:
			duration, err := time.ParseDuration(value)
			if err != nil {
				log.InfraLogger.Errorf("Failed to parse %v: %v, using default value 0", key, err)
				duration = time.Duration(0 * time.Second)
			}
			plugin.defaultPreemptMinRuntime = &metav1.Duration{Duration: duration}
		case reclaimResolveMethod:
			plugin.reclaimResolveMethod = value
		}

	}
	// Initialize with default values if not provided
	if plugin.defaultReclaimMinRuntime == nil {
		plugin.defaultReclaimMinRuntime = &metav1.Duration{Duration: time.Duration(0 * time.Second)}
	}
	if plugin.defaultPreemptMinRuntime == nil {
		plugin.defaultPreemptMinRuntime = &metav1.Duration{Duration: time.Duration(0 * time.Second)}
	}
	if plugin.reclaimResolveMethod == "" {
		plugin.reclaimResolveMethod = resolveMethodLCA
	}
	// setup caches on plugin init, but they will be reset on session open anyway
	plugin.preemptProtectionCache = make(map[common_info.PodGroupID]map[common_info.PodGroupID]bool)
	plugin.reclaimProtectionCache = make(map[common_info.PodGroupID]map[common_info.PodGroupID]bool)
	plugin.preemptMinRuntimeCache = make(map[common_info.QueueID]metav1.Duration)
	plugin.reclaimMinRuntimeCache = make(map[common_info.QueueID]map[common_info.QueueID]metav1.Duration)
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
	// setup caches on session open
	mr.preemptProtectionCache = make(map[common_info.PodGroupID]map[common_info.PodGroupID]bool)
	mr.reclaimProtectionCache = make(map[common_info.PodGroupID]map[common_info.PodGroupID]bool)
	mr.preemptMinRuntimeCache = make(map[common_info.QueueID]metav1.Duration)
	mr.reclaimMinRuntimeCache = make(map[common_info.QueueID]map[common_info.QueueID]metav1.Duration)
}

func (mr *minruntimePlugin) OnSessionClose(ssn *framework.Session) {
	mr.queues = nil
	mr.preemptProtectionCache = nil
	mr.reclaimProtectionCache = nil
	mr.preemptMinRuntimeCache = nil
	mr.reclaimMinRuntimeCache = nil
}

func (mr *minruntimePlugin) reclaimFilterFn(pendingJob *podgroup_info.PodGroupInfo, victim *podgroup_info.PodGroupInfo) bool {
	protected := mr.isReclaimMinRuntimeProtected(pendingJob, victim)
	// always return true for elastic jobs, but cache the result for scenario validator
	if victim.MinAvailable < int32(len(victim.PodInfos)) {
		return true
	}

	return !protected
}

func (mr *minruntimePlugin) preemptFilterFn(pendingJob *podgroup_info.PodGroupInfo, victim *podgroup_info.PodGroupInfo) bool {
	protected := mr.isPreemptMinRuntimeProtected(pendingJob, victim)
	// always return true for elastic jobs, but cache the result for scenario validator
	if victim.MinAvailable < int32(len(victim.PodInfos)) {

		return true
	}
	return !protected
}

func (mr *minruntimePlugin) reclaimScenarioValidatorFn(reclaimer *podgroup_info.PodGroupInfo, victims []*podgroup_info.PodGroupInfo, tasks []*pod_info.PodInfo) bool {
	for _, victim := range victims {
		protected := mr.isReclaimMinRuntimeProtected(reclaimer, victim)
		if protected {
			numVictimTasks := countVictimTasks(victim, tasks)
			currentlyRunning := victim.GetActivelyRunningTasksCount()
			if victim.MinAvailable > currentlyRunning-numVictimTasks {
				return false
			}
		}
	}
	return true
}

func (mr *minruntimePlugin) preemptScenarioValidatorFn(preemptor *podgroup_info.PodGroupInfo, victims []*podgroup_info.PodGroupInfo, tasks []*pod_info.PodInfo) bool {
	for _, victim := range victims {
		protected := mr.isPreemptMinRuntimeProtected(preemptor, victim)
		if protected {
			numVictimTasks := countVictimTasks(victim, tasks)
			currentlyRunning := victim.GetActivelyRunningTasksCount()
			if victim.MinAvailable > currentlyRunning-numVictimTasks {
				return false
			}
		}
	}
	return true
}

func (mr *minruntimePlugin) isReclaimMinRuntimeProtected(pendingJob *podgroup_info.PodGroupInfo, victim *podgroup_info.PodGroupInfo) bool {
	if mr.reclaimProtectionCache[pendingJob.UID][victim.UID] {
		return true
	}
	// Get the appropriate queue objects for both jobs
	pendingQueue, foundPending := mr.getQueueForJob(pendingJob)
	victimQueue, foundVictim := mr.getQueueForJob(victim)

	// If we can't find the queues, use default min runtime
	var minRuntime metav1.Duration
	if !foundPending || !foundVictim {
		minRuntime = *mr.defaultReclaimMinRuntime
	} else {
		minRuntime = mr.getReclaimMinRuntime(mr.reclaimResolveMethod, pendingQueue, victimQueue)
	}

	// If the victim's last start time plus minimum runtime is greater than current time,
	// the victim is protected from reclaim
	if victim.LastStartTimestamp != nil && !victim.LastStartTimestamp.IsZero() {
		protectedUntil := victim.LastStartTimestamp.Add(minRuntime.Duration)
		if time.Now().Before(protectedUntil) {
			mr.cacheReclaimProtection(pendingJob, victim)
			return true
		}
	}
	return false
}

func (mr *minruntimePlugin) isPreemptMinRuntimeProtected(pendingJob *podgroup_info.PodGroupInfo, victim *podgroup_info.PodGroupInfo) bool {
	if mr.preemptProtectionCache[pendingJob.UID][victim.UID] {
		return true
	}

	// Get the victim's queue
	victimQueue, found := mr.getQueueForJob(victim)

	// If we can't find the queue, use default min runtime
	var minRuntime metav1.Duration
	if !found {
		minRuntime = *mr.defaultPreemptMinRuntime
	} else {
		minRuntime = mr.getPreemptMinRuntime(victimQueue)
	}

	// If the victim's last start time plus minimum runtime is greater than current time,
	// the victim is protected from preemption
	if victim.LastStartTimestamp != nil && !victim.LastStartTimestamp.IsZero() {
		protectedUntil := victim.LastStartTimestamp.Add(minRuntime.Duration)
		if time.Now().Before(protectedUntil) {
			mr.cachePreemptProtection(pendingJob, victim)
			return true
		}
	}

	return false
}

func (mr *minruntimePlugin) cachePreemptProtection(pendingJob *podgroup_info.PodGroupInfo, victim *podgroup_info.PodGroupInfo) {
	if mr.preemptProtectionCache[pendingJob.UID] == nil {
		mr.preemptProtectionCache[pendingJob.UID] = make(map[common_info.PodGroupID]bool)
	}
	mr.preemptProtectionCache[pendingJob.UID][victim.UID] = true
}

func (mr *minruntimePlugin) cacheReclaimProtection(pendingJob *podgroup_info.PodGroupInfo, victim *podgroup_info.PodGroupInfo) {
	if mr.reclaimProtectionCache[pendingJob.UID] == nil {
		mr.reclaimProtectionCache[pendingJob.UID] = make(map[common_info.PodGroupID]bool)
	}
	mr.reclaimProtectionCache[pendingJob.UID][victim.UID] = true
}

func (mr *minruntimePlugin) getQueueForJob(job *podgroup_info.PodGroupInfo) (*queue_info.QueueInfo, bool) {
	if mr.queues == nil {
		return nil, false
	}

	queue, found := mr.queues[job.Queue]
	return queue, found
}

func countVictimTasks(victim *podgroup_info.PodGroupInfo, tasks []*pod_info.PodInfo) int32 {
	count := int32(0)
	for _, task := range tasks {
		if task.Job == victim.UID {
			count++
		}
	}
	return count
}
