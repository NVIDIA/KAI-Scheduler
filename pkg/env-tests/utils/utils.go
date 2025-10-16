// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package utils

import (
	"sync"
)

var lastUsedPort = 8080
var lock = sync.Mutex{}

func GetNextAvailablePort() int {
	lock.Lock()
	defer lock.Unlock()

	lastUsedPort++
	return lastUsedPort
}
