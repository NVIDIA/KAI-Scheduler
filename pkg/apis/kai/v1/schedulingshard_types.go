/*
Copyright 2023.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	"github.com/NVIDIA/KAI-scheduler/pkg/apis/kai/v1/common"
)

const (
	binpackStrategy  = "binpack"
	defaultImageName = "runai-scheduler"
)

// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// SchedulingShardSpec defines the desired state of SchedulingShard
type SchedulingShardSpec struct {
	// Important: Run "make" to regenerate code after modifying this file

	// Args specifies the CLI arguments for the scheduler
	// +kubebuilder:validation:Optional
	Args *Args `json:"args,omitempty"`

	// PartitionLabelValue is the value for the partition label
	// +kubebuilder:validation:Optional
	PartitionLabelValue string `json:"partitionLabelValue,omitempty"`
}

func (s *SchedulingShardSpec) SetDefaultsWhereNeeded() {
	if s.Args == nil {
		s.Args = &Args{}
	}
	s.Args.SetDefaultWhereNeeded()
}

// Args defines command line arguments for the pod-grouper
type Args struct {
	// Verbosity specifies the logging level for the runai-scheduler
	// +kubebuilder:validation:Optional
	Verbosity *int `json:"verbosity,omitempty"`

	// PlacementStrategy is the placement scheduler strategy
	// +kubebuilder:validation:Optional
	PlacementStrategy *PlacementStrategy `json:"placementStrategy,omitempty"`

	// DefaultSchedulerPeriod specifies the default period for the scheduler
	// +kubebuilder:validation:Optional
	DefaultSchedulerPeriod *string `json:"defaultSchedulerPeriod,omitempty"`

	// DefaultSchedulerName specifies the default name for the scheduler
	// +kubebuilder:validation:Optional
	DefaultSchedulerName *string `json:"defaultSchedulerName,omitempty"`

	// RestrictNodeScheduling specifies if the scheduler should restrict cpu workload to cpu nodes and gpu workloads to
	// gpu nodes
	// +kubebuilder:validation:Optional
	RestrictNodeScheduling *bool `json:"restrictNodeScheduling,omitempty"`

	// DetailedFitErrors configures whether the scheduler records node details in fit errors in podgroup events
	// +kubebuilder:validation:Optional
	DetailedFitErrors *bool `json:"detailedFitErrors,omitempty"`

	// AdvancedCSIScheduling configures whether the scheduler will consider CSI storage for reclaiming jobs
	// +kubebuilder:validation:Optional
	AdvancedCSIScheduling *bool `json:"advancedCSIScheduling,omitempty"`

	// MaxNumberConsolidationPreemptees specifies the max number of potential preemptees
	// considered in a single consolidation action
	// +kubebuilder:validation:Optional
	MaxNumberConsolidationPreemptees *int `json:"maxNumberConsolidationPreemptees,omitempty"`

	// ClientQPS specifies the qps parameter for the scheduler kube client
	// +kubebuilder:validation:Optional
	ClientQPS *int `json:"clientQPS,omitempty"`

	// ClientBurst specifies the burst parameter for the scheduler kube client
	// +kubebuilder:validation:Optional
	ClientBurst *int `json:"clientBurst,omitempty"`

	// Profiling specifies profiling configuration
	// +kubebuilder:validation:Optional
	Profiling *bool `json:"profiling,omitempty"`

	// Pyroscope Profiler configuration
	// +kubebuilder:validation:Optional
	Pyroscope *common.Pyroscope `json:"pyroscope,omitempty"`

	// UseSchedulingSignatures use scheduling signatures to avoid duplicate scheduling attempts for identical jobs.
	// +kubebuilder:validation:Optional
	UseSchedulingSignatures *bool `json:"useSchedulingSignatures,omitempty"`

	// FullHierarchyFairness specifies whether fairness is enforced across projects and departments levels
	// +kubebuilder:validation:Optional
	FullHierarchyFairness *bool `json:"fullHierarchyFairness,omitempty"`

	// AllowConsolidatingReclaim specifies whether pipelined pods towards should count towards 'reclaimed' resources
	// +kubebuilder:validation:Optional
	AllowConsolidatingReclaim *bool `json:"allowConsolidatingReclaim,omitempty"`

	// DefaultStalenessGracePeriod specifies the default staleness grace period for jobs
	// +kubebuilder:validation:Optional
	DefaultStalenessGracePeriod *string `json:"defaultStalenessGracePeriod,omitempty"`

	// NumOfStatusRecordingWorkers specifies the max number of go routines spawned to update pod and podgroups conditions and events
	// +kubebuilder:validation:Optional
	NumOfStatusRecordingWorkers *int `json:"numOfStatusRecordingWorkers,omitempty"`

	// QueueDepthPerAction max number of jobs to try for action per queue
	// +kubebuilder:validation:Optional
	QueueDepthPerAction map[string]int `json:"queueDepthPerAction,omitempty"`

	// PreemptMinRuntime specifies the minimum runtime of a job in queue before it can be preempted
	// +kubebuilder:validation:Optional
	PreemptMinRuntime *string `json:"preemptMinRuntime,omitempty"`

	// ReclaimMinRuntime specifies the minimum runtime of a job in queue before it can be reclaimed
	// +kubebuilder:validation:Optional
	ReclaimMinRuntime *string `json:"reclaimMinRuntime,omitempty"`

	// PluginServerPort specifies the port to bind for plugin server requests
	// +kubebuilder:validation:Optional
	PluginServerPort *int `json:"pluginServerPort,omitempty"`

	// UpdatePodEvictionCondition configures whether the scheduler updates pod eviction conditions to reflect status
	// +kubebuilder:validation:Optional
	UpdatePodEvictionCondition *bool `json:"updatePodEvictionCondition,omitempty"`
}

func (a *Args) SetDefaultWhereNeeded() {
	if a.PlacementStrategy == nil {
		a.PlacementStrategy = &PlacementStrategy{}
	}
	a.PlacementStrategy.SetDefaultWhereNeeded()
	if a.Verbosity == nil {
		a.Verbosity = ptr.To(3)
	}
	if a.DefaultSchedulerPeriod == nil {
		a.DefaultSchedulerPeriod = ptr.To("5s")
	}
	if a.DefaultSchedulerName == nil {
		a.DefaultSchedulerName = ptr.To(defaultImageName)
	}
	if a.RestrictNodeScheduling == nil {
		a.RestrictNodeScheduling = ptr.To(false)
	}
	if a.DetailedFitErrors == nil {
		a.DetailedFitErrors = ptr.To(true)
	}
	if a.AdvancedCSIScheduling == nil {
		a.AdvancedCSIScheduling = ptr.To(false)
	}
	if a.Profiling == nil {
		a.Profiling = ptr.To(false)
	}
	if a.UseSchedulingSignatures == nil {
		a.UseSchedulingSignatures = ptr.To(true)
	}
	if a.AllowConsolidatingReclaim == nil {
		a.AllowConsolidatingReclaim = ptr.To(true)
	}
	if a.PluginServerPort == nil {
		a.PluginServerPort = ptr.To(8081)
	}
	if a.UpdatePodEvictionCondition == nil {
		a.UpdatePodEvictionCondition = ptr.To(false)
	}
}

// PlacementStrategy defines the scheduling strategy of NodePool
type PlacementStrategy struct {
	// Gpu scheduling strategy (binpack/spread)
	// +kubebuilder:validation:Optional
	Gpu *string `json:"gpu,omitempty"`

	// Cpu scheduling strategy (binpack/spread)
	// +kubebuilder:validation:Optional
	Cpu *string `json:"cpu,omitempty"`
}

func (p *PlacementStrategy) SetDefaultWhereNeeded() {
	if p.Gpu == nil {
		p.Gpu = ptr.To(binpackStrategy)
	}
	if p.Cpu == nil {
		p.Cpu = ptr.To(binpackStrategy)
	}
}

// SchedulingShardStatus defines the observed state of SchedulingShard
type SchedulingShardStatus struct {
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster

// SchedulingShard is the Schema for the schedulingshards API
type SchedulingShard struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   SchedulingShardSpec   `json:"spec,omitempty"`
	Status SchedulingShardStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// SchedulingShardList contains a list of SchedulingShard
type SchedulingShardList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []SchedulingShard `json:"items"`
}

func init() {
	SchemeBuilder.Register(&SchedulingShard{}, &SchedulingShardList{})
}
