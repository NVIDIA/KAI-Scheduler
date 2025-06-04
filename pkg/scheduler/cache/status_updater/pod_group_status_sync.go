// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package status_updater

type podGroupStatusSyncResult string

const (
	snapshotStatusIsOlder podGroupStatusSyncResult = "snapshotStatusIsOlder"
	equalStatuses         podGroupStatusSyncResult = "equalStatuses"
	updateRequestIsOlder  podGroupStatusSyncResult = "updateRequestIsOlder"
)
