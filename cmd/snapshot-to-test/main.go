// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/snapshotrunner"
)

func main() {
	fs := flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	snapshotFile := fs.String("snapshot", "", "path to snapshot zip file (required)")
	outputFile := fs.String("output", "", "output Go test file path (default: <snapshot-basename>_test.go)")
	testName := fs.String("test-name", "", "name for the generated test function (default: TestSnapshot<snapshot-basename>)")
	packageName := fs.String("package", "snapshots_test", "package name for generated test file")
	_ = fs.Parse(os.Args[1:])

	if snapshotFile == nil || *snapshotFile == "" {
		fmt.Fprintf(os.Stderr, "Error: --snapshot is required\n")
		fs.Usage()
		os.Exit(1)
	}

	snap, err := snapshotrunner.LoadSnapshot(*snapshotFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading snapshot: %v\n", err)
		os.Exit(1)
	}

	outputPath := *outputFile
	if outputPath == "" {
		base := strings.TrimSuffix(filepath.Base(*snapshotFile), filepath.Ext(*snapshotFile))
		base = strings.TrimSuffix(base, ".gzip")
		outputPath = base + "_test.go"
	}

	testFuncName := *testName
	if testFuncName == "" {
		base := strings.TrimSuffix(filepath.Base(*snapshotFile), filepath.Ext(*snapshotFile))
		base = strings.TrimSuffix(base, ".gzip")
		// Convert to PascalCase for function name
		parts := strings.Split(strings.ReplaceAll(base, "-", "_"), "_")
		for i, part := range parts {
			if len(part) > 0 {
				parts[i] = strings.ToUpper(part[:1]) + part[1:]
			}
		}
		testFuncName = "TestSnapshot" + strings.Join(parts, "")
	}

	generator := NewTestGenerator(*packageName, testFuncName)
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
}
