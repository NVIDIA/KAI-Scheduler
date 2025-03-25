// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package conf

import (
	"time"

	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
)

type SchedulingNodePoolParams struct {
	NodePoolLabelKey   string
	NodePoolLabelValue string
}

type SchedulerParams struct {
	SchedulerName                     string                    `json:"schedulerName,omitempty"`
	RestrictSchedulingNodes           bool                      `json:"restrictSchedulingNodes,omitempty"`
	PartitionParams                   *SchedulingNodePoolParams `json:"partitionParams,omitempty"`
	IsInferencePreemptible            bool                      `json:"isInferencePreemptible,omitempty"`
	MaxNumberConsolidationPreemptees  int                       `json:"maxNumberConsolidationPreemptees,omitempty"`
	ScheduleCSIStorage                bool                      `json:"scheduleCSIStorage,omitempty"`
	UseSchedulingSignatures           bool                      `json:"useSchedulingSignatures,omitempty"`
	FullHierarchyFairness             bool                      `json:"fullHierarchyFairness,omitempty"`
	NodeLevelScheduler                bool                      `json:"nodeLevelScheduler,omitempty"`
	AllowConsolidatingReclaim         bool                      `json:"allowConsolidatingReclaim,omitempty"`
	NumOfStatusRecordingWorkers       int                       `json:"numOfStatusRecordingWorkers,omitempty"`
	GlobalDefaultStalenessGracePeriod time.Duration             `json:"globalDefaultStalenessGracePeriod,omitempty"`
	SchedulePeriod                    time.Duration             `json:"schedulePeriod,omitempty"`
	DetailedFitErrors                 bool                      `json:"detailedFitErrors,omitempty"`
}

// SchedulerConfiguration defines the configuration of scheduler.
type SchedulerConfiguration struct {
	// Actions defines the actions list of scheduler in order
	Actions string `yaml:"actions" json:"actions"`
	// Tiers defines plugins in different tiers
	Tiers []Tier `yaml:"tiers,omitempty" json:"tiers,omitempty"`
	// QueueDepthPerAction max number of jobs to try for action per queue
	QueueDepthPerAction map[string]int `yaml:"queueDepthPerAction,omitempty" json:"queueDepthPerAction,omitempty"`
}

// Tier defines plugin tier
type Tier struct {
	Plugins []PluginOption `yaml:"plugins" json:"plugins"`
}

// PluginOption defines the options of plugin
type PluginOption struct {
	// The name of Plugin
	Name string `yaml:"name" json:"name"`
	// JobOrderDisabled defines whether jobOrderFn is disabled
	JobOrderDisabled bool `yaml:"disableJobOrder" json:"disableJobOrder"`
	// TaskOrderDisabled defines whether taskOrderFn is disabled
	TaskOrderDisabled bool `yaml:"disableTaskOrder" json:"disableTaskOrder"`
	// PreemptableDisabled defines whether preemptableFn is disabled
	PreemptableDisabled bool `yaml:"disablePreemptable" json:"disablePreemptable"`
	// ReclaimableDisabled defines whether reclaimableFn is disabled
	ReclaimableDisabled bool `yaml:"disableReclaimable" json:"disableReclaimable"`
	// QueueOrderDisabled defines whether queueOrderFn is disabled
	QueueOrderDisabled bool `yaml:"disableQueueOrder" json:"disableQueueOrder"`
	// PredicateDisabled defines whether predicateFn is disabled
	PredicateDisabled bool `yaml:"disablePredicate" json:"disablePredicate"`
	// NodeOrderDisabled defines whether NodeOrderFn is disabled
	NodeOrderDisabled bool `yaml:"disableNodeOrder" json:"disableNodeOrder"`
	// Arguments defines the different arguments that can be given to different plugins
	Arguments map[string]string `yaml:"arguments" json:"arguments"`
}

func (s *SchedulingNodePoolParams) GetLabelSelector() (labels.Selector, error) {
	if s.NodePoolLabelKey == "" {
		return labels.Everything(), nil
	}
	operator := selection.DoesNotExist
	var vals []string
	if len(s.NodePoolLabelValue) > 0 {
		operator = selection.Equals
		vals = []string{s.NodePoolLabelValue}
	}

	requirement, err := labels.NewRequirement(s.NodePoolLabelKey, operator, vals)
	if err != nil {
		return nil, err
	}
	selector := labels.NewSelector().Add(*requirement)
	return selector, nil
}

func (s *SchedulingNodePoolParams) GetLabels() map[string]string {
	if s.NodePoolLabelKey == "" || s.NodePoolLabelValue == "" {
		return map[string]string{}
	}
	return map[string]string{s.NodePoolLabelKey: s.NodePoolLabelValue}
}
