// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package storageclass_info

import (
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/common_info"
)

type StorageClassInfo struct {
	ID          common_info.StorageClassID `json:"id"`
	Provisioner string                     `json:"provisioner"`
}
