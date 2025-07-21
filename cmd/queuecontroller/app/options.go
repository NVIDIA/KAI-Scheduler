// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package app

import (
	"flag"

	"github.com/NVIDIA/KAI-scheduler/pkg/common/constants"
	kaiflags "github.com/NVIDIA/KAI-scheduler/pkg/common/flags"
)

const (
	defaultSchedulingQueueLabelKey = "kai.scheduler/queue"
	defaultMetricsAddress          = ":8080"
)

type Options struct {
	EnableLeaderElection    bool
	SchedulingQueueLabelKey string

	MetricsAddress                 string
	MetricsNamespace               string
	QueueLabelToMetricLabel        kaiflags.StringMapFlag
	QueueLabelToDefaultMetricValue kaiflags.StringMapFlag
}

func (o *Options) AddFlags(fs *flag.FlagSet) {
	fs.BoolVar(&o.EnableLeaderElection, "leader-elect", false, "Enable leader election for controller manager. Enabling this will ensure there is only one active controller manager.")
	fs.StringVar(&o.SchedulingQueueLabelKey, "queue-label-key", defaultSchedulingQueueLabelKey, "Scheduling queue label key name.")
	fs.StringVar(&o.MetricsAddress, "metrics-listen-address", defaultMetricsAddress, "The address the metrics endpoint binds to.")
	fs.StringVar(&o.MetricsNamespace, "metrics-namespace", constants.DefaultMetricsNamespace, "Metrics namespace.")
	fs.Var(&o.QueueLabelToMetricLabel, "queue-label-to-metric-label", "Map of queue label keys to metric label keys, e.g. 'foo=bar,baz=qux'.")
	fs.Var(&o.QueueLabelToDefaultMetricValue, "queue-label-to-default-metric-value", "Map of queue label keys to default metric values, in case the label doesn't exist on the queue, e.g. 'foo=1,baz=0'.")
}
