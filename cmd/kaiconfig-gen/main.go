// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	kai "github.com/NVIDIA/KAI-scheduler/pkg/apis/kai/v1"
	"sigs.k8s.io/yaml"
)

func main() {
	// Flags
	outputDir := flag.String("out-dir", "docs/kaiconfig", "directory to write the generated YAML file into")
	fileName := flag.String("file-name", "example.yaml", "output file name")
	name := flag.String("name", "kai-config", "metadata.name for the Config resource")
	namespace := flag.String("namespace", "kai-scheduler", "default namespace for operands (spec.namespace)")
	flag.Parse()

	// Build minimal Config and apply defaults
	config := kai.Config{}
	config.APIVersion = "kai.scheduler/v1"
	config.Kind = "Config"
	config.Name = *name

	// If user passed a namespace, set it before defaulting
	config.Spec.Namespace = *namespace

	// Apply defaults on Spec
	config.Spec.SetDefaultsWhereNeeded()

	// Marshal to YAML
	data, err := yaml.Marshal(config)
	if err != nil {
		fatalf("failed marshaling Config to YAML: %v", err)
	}

	// Ensure output directory exists
	if err := os.MkdirAll(*outputDir, 0o755); err != nil {
		fatalf("failed creating output directory %q: %v", *outputDir, err)
	}

	outputPath := filepath.Join(*outputDir, *fileName)
	if err := os.WriteFile(outputPath, data, 0o644); err != nil {
		fatalf("failed writing file %q: %v", outputPath, err)
	}

	fmt.Printf("Wrote defaulted KAI Config to %s\n", outputPath)
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
