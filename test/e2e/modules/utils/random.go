/*
Copyright 2025 NVIDIA CORPORATION
SPDX-License-Identifier: Apache-2.0
*/
package utils

import "math/rand"

func RandomIntBetween(min, max int) int {
	return rand.Intn(max-min) + min
}
