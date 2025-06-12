// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package app

import (
	"flag"
)

type Options struct {
	EnableLeaderElection    bool
	SchedulingQueueLabelKey string
}

func (o *Options) AddFlags(fs *flag.FlagSet) {
	fs.BoolVar(&o.EnableLeaderElection, "leader-elect", false, "Enable leader election for controller manager. Enabling this will ensure there is only one active controller manager.")
	fs.StringVar(&o.SchedulingQueueLabelKey, "queue-label-key", "runai/queue", "Scheduling queue label key name")
}
