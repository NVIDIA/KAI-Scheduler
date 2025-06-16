// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"os"

	"github.com/NVIDIA/KAI-scheduler/cmd/scheduler/app"
)

func main() {
	if err := app.RunApp(); err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err) // bla
		os.Exit(1)
	}
}
