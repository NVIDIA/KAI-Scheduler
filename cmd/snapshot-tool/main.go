// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/NVIDIA/KAI-scheduler/pkg/snapshotrunner"
	"github.com/NVIDIA/KAI-scheduler/pkg/snapshottest"
)

func main() {
	fs := flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	verbosity := fs.Int("verbosity", 4, "logging verbosity")
	filename := fs.String("filename", "", "location of the zipped JSON file")
	cpuprofile := fs.String("cpuprofile", "", "write cpu profile to file")
	generateTest := fs.Bool("generate-test", false, "generate integration test from snapshot")
	outputFile := fs.String("output", "", "output Go test file path (default: <snapshot-basename>_test.go)")
	testName := fs.String("test-name", "", "name for the generated test function (default: TestSnapshot<snapshot-basename>)")
	packageName := fs.String("package", "snapshots_test", "package name for generated test file")
	_ = fs.Parse(os.Args[1:])

	if *generateTest {
		// Handle test generation mode
		if filename == nil || *filename == "" {
			fmt.Fprintf(os.Stderr, "Error: --filename is required when generating tests\n")
			fs.Usage()
			os.Exit(1)
		}

		snap, err := snapshotrunner.LoadSnapshot(*filename)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error loading snapshot: %v\n", err)
			os.Exit(1)
		}

		outputPath := *outputFile
		if outputPath == "" {
			outputPath = snapshottest.GenerateOutputPath(*filename)
		}

		testFuncName := *testName
		if testFuncName == "" {
			testFuncName = snapshottest.GenerateTestName(*filename)
		}

		generator := snapshottest.NewTestGenerator(*packageName, testFuncName)
		code, err := generator.Generate(snap)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error generating test code: %v\n", err)
			os.Exit(1)
		}

		if err := os.WriteFile(outputPath, []byte(code), 0644); err != nil {
			fmt.Fprintf(os.Stderr, "Error writing output file: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("Successfully generated test file: %s\n", outputPath)
		return
	}

	// Handle original snapshot execution mode
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
