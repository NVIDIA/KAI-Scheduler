// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package app

import (
	"flag"

	"github.com/NVIDIA/KAI-scheduler/pkg/common/constants"
	kaiflags "github.com/NVIDIA/KAI-scheduler/pkg/common/flags"
)

const (
	defaultMetricsAddress = ":8080"
)

type Options struct {
	EnableLeaderElection    bool
	SchedulingQueueLabelKey string
	EnableWebhook           bool

	MetricsAddress                 string
	MetricsNamespace               string
	QueueLabelToMetricLabel        kaiflags.StringMapFlag
	QueueLabelToDefaultMetricValue kaiflags.StringMapFlag

	// k8s client options
	Qps   int
	Burst int
}

func InitOptions() *Options {
	o := &Options{}

	flag.BoolVar(&o.EnableLeaderElection, "leader-elect", false, "Enable leader election for controller manager. Enabling this will ensure there is only one active controller manager.")
	flag.StringVar(&o.SchedulingQueueLabelKey, "queue-label-key", constants.DefaultQueueLabel, "Scheduling queue label key name.")
	flag.BoolVar(&o.EnableWebhook, "enable-webhook", true, "Enable webhook for controller manager.")
	flag.StringVar(&o.MetricsAddress, "metrics-listen-address", defaultMetricsAddress, "The address the metrics endpoint binds to.")
	flag.StringVar(&o.MetricsNamespace, "metrics-namespace", constants.DefaultMetricsNamespace, "Metrics namespace.")
	flag.Var(&o.QueueLabelToMetricLabel, "queue-label-to-metric-label", "Map of queue label keys to metric label keys, e.g. 'foo=bar,baz=qux'.")
	flag.Var(&o.QueueLabelToDefaultMetricValue, "queue-label-to-default-metric-value", "Map of queue label keys to default metric values, in case the label doesn't exist on the queue, e.g. 'foo=1,baz=0'.")
	flag.IntVar(&o.Qps, "qps", 50, "Queries per second to the K8s API server")
	flag.IntVar(&o.Burst, "burst", 300, "Burst to the K8s API server")

	return o
}
