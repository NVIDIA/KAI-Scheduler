/*
Copyright 2025 NVIDIA CORPORATION
SPDX-License-Identifier: Apache-2.0
*/
package feature_flags

import (
	"fmt"
	"strings"
)

func genericArgsUpdater[T any](args []string, parameterName string, value *T) []string {
	updatedArgs := make([]string, 0)
	for _, arg := range args {
		if !strings.HasPrefix(arg, parameterName) {
			updatedArgs = append(updatedArgs, arg)
		}
	}
	if value != nil {
		return append(updatedArgs, fmt.Sprintf("%s%v", parameterName, *value))
	}

	return updatedArgs
}
