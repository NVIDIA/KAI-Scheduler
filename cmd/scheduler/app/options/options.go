// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package options

import (
	"time"

	"github.com/spf13/pflag"
	utilfeature "k8s.io/apiserver/pkg/util/feature"

	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/log"
)

const (
	defaultSchedulerName               = "kai-scheduler"
	defaultMetricsNamespace            = "kai"
	defaultSchedulerPeriod             = time.Second
	defaultStalenessGracePeriod        = 60 * time.Second
	defaultListenAddress               = ":8080"
	defaultProfilerApiPort             = "8182"
	defaultVerbosityLevel              = 3
	defaultMaxConsolidationPreemptees  = 16
	defaultDetailedFitError            = false
	DefaultPyroscopeMutexProfilerRate  = 5
	DefaultPyroscopeBlockProfilerRate  = 5
	defaultNumOfStatusRecordingWorkers = 5
	defaultNodePoolLabelKey            = ""
)

// ServerOption is the main context object for the controller manager.
type ServerOption struct {
	SchedulerName                     string
	SchedulerConf                     string
	SchedulePeriod                    time.Duration
	EnableLeaderElection              bool
	PrintVersion                      bool
	MetricsNamespace                  string
	RestrictSchedulingNodes           bool
	NodePoolLabelKey                  string
	NodePoolLabelValue                string
	ListenAddress                     string
	EnableProfiler                    bool
	ProfilerApiPort                   string
	PyroscopeAddress                  string
	PyroscopeMutexProfilerRate        int
	PyroscopeBlockProfilerRate        int
	Verbosity                         int
	IsInferencePreemptible            bool
	MaxNumberConsolidationPreemptees  int
	DetailedFitErrors                 bool
	ScheduleCSIStorage                bool
	UseSchedulingSignatures           bool
	FullHierarchyFairness             bool
	NodeLevelScheduler                bool
	AllowConsolidatingReclaim         bool
	NumOfStatusRecordingWorkers       int
	GlobalDefaultStalenessGracePeriod time.Duration
	PluginServerPort                  int

	QPS   int
	Burst int
}

// NewServerOption creates a new CMServer with a default config.
func NewServerOption() *ServerOption {
	s := ServerOption{}
	return &s
}

// AddFlags adds flags for a specific CMServer to the specified FlagSet
func (s *ServerOption) AddFlags(fs *pflag.FlagSet) {
	// kai-scheduler will ignore pods with scheduler names other than specified with the option
	fs.StringVar(&s.SchedulerName, "scheduler-name", defaultSchedulerName, "The scheduler name in pod spec that handled by this scheduler")
	fs.BoolVar(&s.RestrictSchedulingNodes, "restrict-node-scheduling", false, "kai-scheduler will allocate jobs only to restricted nodes")
	fs.StringVar(&s.NodePoolLabelKey, "nodepool-label-key", defaultNodePoolLabelKey, "The label key by which to filter scheduling nodepool")
	fs.StringVar(&s.NodePoolLabelValue, "partition-label-value", "", "The label value by which to filter scheduling partition")
	fs.StringVar(&s.SchedulerConf, "scheduler-conf", "", "The absolute path of scheduler configuration file")
	fs.DurationVar(&s.SchedulePeriod, "schedule-period", defaultSchedulerPeriod, "The period between each scheduling cycle")
	fs.BoolVar(&s.EnableLeaderElection, "leader-elect", false,
		"Start a leader election client and gain leadership before "+
			"executing the main loop. Enable this when running replicated kai-scheduler for high availability")
	fs.BoolVar(&s.PrintVersion, "version", true, "Show version")
	fs.StringVar(&s.MetricsNamespace, "metrics-namespace", defaultMetricsNamespace, "Metrics namespace")
	fs.StringVar(&s.ListenAddress, "listen-address", defaultListenAddress, "The address to listen on for HTTP requests")
	fs.BoolVar(&s.EnableProfiler, "enable-profiler", false, "Enable profiler")
	fs.StringVar(&s.ProfilerApiPort, "profiler-port", defaultProfilerApiPort, "The port to listen for profiler api requests")
	fs.StringVar(&s.PyroscopeAddress, "pyroscope-address", "", "The url of pyroscope")
	fs.IntVar(&s.PyroscopeMutexProfilerRate, "pyroscope-mutex-profiler-rate", DefaultPyroscopeMutexProfilerRate, "Mutex Profiler rate")
	fs.IntVar(&s.PyroscopeBlockProfilerRate, "pyroscope-block-profiler-rate", DefaultPyroscopeBlockProfilerRate, "Block Profiler rate")
	fs.IntVar(&s.Verbosity, "v", defaultVerbosityLevel, "Verbosity level")
	fs.BoolVar(&s.IsInferencePreemptible, "inference-preemptible", false, "Consider Inference workloads as preemptible")
	fs.IntVar(&s.MaxNumberConsolidationPreemptees, "max-consolidation-preemptees", defaultMaxConsolidationPreemptees, "Maximum number of consolidation preemptees. Defaults to 16")
	fs.IntVar(&s.QPS, "qps", 50, "Queries per second to the K8s API server")
	fs.IntVar(&s.Burst, "burst", 300, "Burst to the K8s API server")
	fs.BoolVar(&s.DetailedFitErrors, "detailed-fit-errors", defaultDetailedFitError, "Write detailed fit errors for every node on every podgroup")
	fs.BoolVar(&s.ScheduleCSIStorage, "schedule-csi-storage", false, "Enables advanced scheduling (preempt, reclaim) for csi storage objects")
	fs.BoolVar(&s.UseSchedulingSignatures, "use-scheduling-signatures", true, "Use scheduling signatures to avoid duplicate scheduling attempts for identical jobs")
	fs.BoolVar(&s.FullHierarchyFairness, "full-hierarchy-fairness", true, "Fairness across project and department levels")
	fs.BoolVar(&s.NodeLevelScheduler, "node-level-scheduler", false, "Node level scheduler is enforced in all core powered pods")
	fs.BoolVar(&s.AllowConsolidatingReclaim, "allow-consolidating-reclaim", true, "Do not count pipelined pods towards 'reclaimed' resources")
	fs.IntVar(&s.NumOfStatusRecordingWorkers, "num-of-status-recording-workers", defaultNumOfStatusRecordingWorkers, "specifies the max number of go routines spawned to update pod and podgroups conditions and events. Defaults to 5")
	fs.DurationVar(&s.GlobalDefaultStalenessGracePeriod, "default-staleness-grace-period", defaultStalenessGracePeriod, "Global default staleness grace period duration. Negative values means infinite. Defaults to 60s")
	fs.IntVar(&s.PluginServerPort, "plugin-server-port", 8081, "The port to bind for plugin server requests")

	utilfeature.DefaultMutableFeatureGate.AddFlag(fs)
}

func (so *ServerOption) ValidateOptions() error {
	pflag.Parse()
	pflag.VisitAll(func(flag *pflag.Flag) {
		log.InfraLogger.V(1).Infof("FLAG: --%s=%q", flag.Name, flag.Value)
	})
	return nil
}
