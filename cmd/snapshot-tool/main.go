// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/snapshotrunner"
)

func main() {
	fs := flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	verbosity := fs.Int("verbosity", 4, "logging verbosity")
	filename := fs.String("filename", "", "location of the zipped JSON file")
	cpuprofile := fs.String("cpuprofile", "", "write cpu profile to file")
	_ = fs.Parse(os.Args[1:])
	if filename == nil || len(*filename) == 0 {
		fs.Usage()
		return
	}

	opts := snapshotrunner.Options{
		Filename:   *filename,
		Verbosity:  int(*verbosity),
		CPUProfile: *cpuprofile,
	}

	if err := snapshotrunner.Run(opts); err != nil {
		fmt.Printf("Failed to run snapshot: %v\n", err)
	}
}
