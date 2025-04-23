// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package metrics

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto" // auto-registry collectors in default registry
)

const (
	// OnSessionOpen label
	OnSessionOpen = "OnSessionOpen"

	// OnSessionClose label
	OnSessionClose = "OnSessionClose"
)

var (
	currentAction string
	subsystemName string

	e2eSchedulingLatency = promauto.NewGauge(
		prometheus.GaugeOpts{
			Subsystem: subsystemName,
			Name:      "e2e_scheduling_latency_milliseconds",
			Help:      "E2e scheduling latency in milliseconds (scheduling algorithm + binding), as a gauge",
		},
	)

	openSessionLatency = promauto.NewGauge(
		prometheus.GaugeOpts{
			Subsystem: subsystemName,
			Name:      "open_session_latency_milliseconds",
			Help:      "Open session latency in milliseconds, including all plugins, as a gauge",
		},
	)

	closeSessionLatency = promauto.NewGauge(
		prometheus.GaugeOpts{
			Subsystem: subsystemName,
			Name:      "close_session_latency_milliseconds",
			Help:      "Close session latency in milliseconds, including all plugins, as a gauge",
		},
	)

	pluginSchedulingLatency = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Subsystem: subsystemName,
			Name:      "plugin_scheduling_latency_milliseconds",
			Help:      "Plugin scheduling latency in milliseconds, as a gauge",
		}, []string{"plugin", "OnSession"})

	actionSchedulingLatency = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Subsystem: subsystemName,
			Name:      "action_scheduling_latency_milliseconds",
			Help:      "Action scheduling latency in milliseconds, as a gauge",
		}, []string{"action"})

	taskSchedulingLatency = promauto.NewHistogram(
		prometheus.HistogramOpts{
			Subsystem: subsystemName,
			Name:      "task_scheduling_latency_milliseconds",
			Help:      "Task scheduling latency in milliseconds",
			Buckets:   prometheus.ExponentialBuckets(5, 2, 10),
		},
	)

	taskBindLatency = promauto.NewHistogram(
		prometheus.HistogramOpts{
			Subsystem: subsystemName,
			Name:      "task_bind_latency_milliseconds",
			Help:      "Task bind latency histogram in milliseconds",
			Buckets:   prometheus.ExponentialBuckets(5, 2, 10),
		},
	)

	podgroupsScheduledByAction = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Subsystem: subsystemName,
			Name:      "podgroups_scheduled_by_action",
			Help:      "Count of podgroups scheduled per action",
		}, []string{"action"})

	podgroupsConsideredByAction = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Subsystem: subsystemName,
			Name:      "podgroups_acted_on_by_action",
			Help:      "Count of podgroups tried per action",
		}, []string{"action"})

	scenariosSimulatedByAction = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Subsystem: subsystemName,
			Name:      "scenarios_simulation_by_action",
			Help:      "Count of scenarios simulated per action",
		}, []string{"action"})

	scenariosFilteredByAction = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Subsystem: subsystemName,
			Name:      "scenarios_filtered_by_action",
			Help:      "Count of scenarios filtered per action",
		}, []string{"action"})

	preemptionAttempts = promauto.NewCounter(
		prometheus.CounterOpts{
			Subsystem: subsystemName,
			Name:      "total_preemption_attempts",
			Help:      "Total preemption attempts in the cluster till now",
		},
	)

	queueFairShareCPU = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Subsystem: subsystemName,
			Name:      "queue_fair_share_cpu_cores",
			Help:      "CPU Fair share of queue, as a gauge. Value is in Cores",
		}, []string{"queue_name"})
	queueFairShareMemory = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Subsystem: subsystemName,
			Name:      "queue_fair_share_memory_gb",
			Help:      "Memory Fair share of queue, as a gauge. Value is in GB",
		}, []string{"queue_name"})
	queueFairShareGPU = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Subsystem: subsystemName,
			Name:      "queue_fair_share_gpu",
			Help:      "GPU Fair share of queue, as a gauge. Values in GPU devices",
		}, []string{"queue_name"})
)

// UpdateOpenSessionDuration updates latency for open session, including all plugins
func UpdateOpenSessionDuration(startTime time.Time) {
	duration := Duration(startTime).Milliseconds()
	openSessionLatency.Set(float64(duration))
}

// UpdateCloseSessionDuration updates latency for close session, including all plugins
func UpdateCloseSessionDuration(startTime time.Time) {
	duration := Duration(startTime).Milliseconds()
	closeSessionLatency.Set(float64(duration))
}

// UpdatePluginDuration updates latency for every plugin
func UpdatePluginDuration(pluginName, OnSessionStatus string, duration time.Duration) {
	pluginSchedulingLatency.WithLabelValues(pluginName, OnSessionStatus).Set(float64(duration.Milliseconds()))
}

// UpdateActionDuration updates latency for every action
func UpdateActionDuration(actionName string, duration time.Duration) {
	actionSchedulingLatency.WithLabelValues(actionName).Set(float64(duration.Milliseconds()))
}

// UpdateE2eDuration updates entire end to end scheduling latency
func UpdateE2eDuration(startTime time.Time) {
	duration := Duration(startTime).Milliseconds()
	e2eSchedulingLatency.Set(float64(duration))
}

// UpdateTaskScheduleDuration updates single task scheduling latency
func UpdateTaskScheduleDuration(duration time.Duration) {
	taskSchedulingLatency.Observe(float64(duration.Milliseconds()))
}

func SetSubSystemName(name string) {
	subsystemName = name
}

func SetCurrentAction(action string) {
	currentAction = action
}

func IncPodgroupScheduledByAction() {
	podgroupsScheduledByAction.WithLabelValues(currentAction).Inc()
}

func IncPodgroupsConsideredByAction() {
	podgroupsConsideredByAction.WithLabelValues(currentAction).Inc()
}

func IncScenarioSimulatedByAction() {
	scenariosSimulatedByAction.WithLabelValues(currentAction).Inc()
}

func IncScenarioFilteredByAction() {
	scenariosFilteredByAction.WithLabelValues(currentAction).Inc()
}

// UpdateTaskBindDuration updates single task bind latency, including bind request creation
func UpdateTaskBindDuration(startTime time.Time) {
	duration := Duration(startTime).Milliseconds()
	taskBindLatency.Observe(float64(duration))
}

// UpdateQueueFairShare updates fair share of queue for a resource
func UpdateQueueFairShare(queueName string, cpu, memory, gpu float64) {
	queueFairShareCPU.WithLabelValues(queueName).Set(cpu)
	queueFairShareMemory.WithLabelValues(queueName).Set(memory)
	queueFairShareGPU.WithLabelValues(queueName).Set(gpu)
}

func ResetQueueFairShare() {
	queueFairShareCPU.Reset()
	queueFairShareMemory.Reset()
	queueFairShareGPU.Reset()
}

// RegisterPreemptionAttempts records number of attempts for preemption
func RegisterPreemptionAttempts() {
	preemptionAttempts.Inc()
}

// Duration get the time since specified start
func Duration(start time.Time) time.Duration {
	return time.Since(start)
}
