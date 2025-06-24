// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"os"

	"github.com/NVIDIA/KAI-scheduler/cmd/podgroupcontroller/app"
)

func main() {
	if err := app.Run(); err != nil {
		fmt.Printf("Error while running the app: %v", err)
		os.Exit(1)
	}
}
