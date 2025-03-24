/*
Copyright 2025 NVIDIA CORPORATION
SPDX-License-Identifier: Apache-2.0
*/
package utils

import (
	"math/rand"
	"time"
)

var generatedNames = make(map[string]bool)

func GenerateRandomK8sName(l int) string {
	str := "abcdefghijklmnopqrstuvwxyz"
	bytes := []byte(str)
	var result []byte
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	for i := 0; i < l; i++ {
		result = append(result, bytes[r.Intn(len(bytes))])
	}
	if generatedNames[string(result)] {
		return GenerateRandomK8sName(l)
	}
	generatedNames[string(result)] = true
	return string(result)
}
