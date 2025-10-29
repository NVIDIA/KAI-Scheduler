// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"k8s.io/klog/v2"
	"sigs.k8s.io/yaml"

	env_tests "github.com/NVIDIA/KAI-scheduler/pkg/env-tests"
	"github.com/NVIDIA/KAI-scheduler/pkg/env-tests/timeaware"
)

var (
	inputFile  = flag.String("input", "example_config.yaml", "Path to input YAML configuration file (use '-' for stdin)")
	outputFile = flag.String("output", "simulation_results.csv", "Path to output CSV file (use '-' for stdout, default: simulation_results.csv)")
)

func main() {
	//klog.InitFlags(flag.CommandLine)
	flag.Parse()

	if err := run(*inputFile, *outputFile); err != nil {
		klog.Fatalf("Error: %v", err)
	}

	klog.Flush()
}

// run executes the simulation with the given input and output file paths.
// Use "-" for stdin/stdout respectively.
func run(inputFile, outputFile string) error {
	ctx := context.Background()

	simulation, err := readConfig(inputFile)
	if err != nil {
		return fmt.Errorf("failed to read configuration: %w", err)
	}

	simulation.SetDefaults()

	klog.Info("Configuration loaded successfully")
	klog.V(1).Infof("  Queues: %d", len(simulation.Queues))
	klog.V(1).Infof("  Jobs: %d", len(simulation.Jobs))
	klog.V(1).Infof("  Nodes: %d", len(simulation.Nodes))
	klog.V(1).Infof("  Cycles: %d", simulation.Cycles)
	klog.V(1).Infof("  WindowSize: %d", simulation.WindowSize)

	// Set up test environment
	klog.Info("Setting up test environment...")
	cfg, ctrlClient, testEnv, err := env_tests.SetupEnvTest(nil)
	if err != nil {
		return fmt.Errorf("failed to setup test environment: %w", err)
	}
	defer func() {
		klog.Info("Shutting down test environment...")
		if err := testEnv.Stop(); err != nil {
			klog.Errorf("Error stopping test environment: %v", err)
		}
	}()

	// Run simulation
	klog.Info("Running simulation...")
	allocationHistory, err, cleanupErr := timeaware.RunSimulation(ctx, ctrlClient, cfg, *simulation)
	if err != nil {
		return fmt.Errorf("failed to run simulation: %w", err)
	}
	if cleanupErr != nil {
		klog.Warningf("Cleanup error: %v", cleanupErr)
	}

	klog.Info("Simulation completed successfully")

	// Write output
	if err := writeOutput(outputFile, allocationHistory.ToCSV()); err != nil {
		return fmt.Errorf("failed to write output: %w", err)
	}

	return nil
}

// readConfig reads the configuration from a file or stdin.
func readConfig(inputFile string) (*timeaware.TimeAwareSimulation, error) {
	var data []byte
	var err error
	if inputFile == "-" {
		klog.Info("Reading configuration from stdin...")
		data, err = io.ReadAll(os.Stdin)
		if err != nil {
			return nil, fmt.Errorf("failed to read configuration from stdin: %w", err)
		}
	} else {
		klog.Infof("Reading configuration from file: %s", inputFile)
		data, err = os.ReadFile(inputFile)
		if err != nil {
			return nil, fmt.Errorf("failed to read configuration from file: %w", err)
		}
	}

	var simulation timeaware.TimeAwareSimulation
	if err := yaml.Unmarshal(data, &simulation); err != nil {
		return nil, fmt.Errorf("failed to parse YAML configuration: %w", err)
	}
	return &simulation, nil
}

// writeOutput writes the CSV data to a file or stdout.
func writeOutput(outputFile, csvData string) error {
	if outputFile == "-" {
		klog.Info("Writing results to stdout...")
		fmt.Print(csvData)
		return nil
	}

	klog.Infof("Writing results to file: %s", outputFile)
	if err := os.WriteFile(outputFile, []byte(csvData), 0644); err != nil {
		return err
	}
	klog.Infof("Results written to: %s", outputFile)
	return nil
}

func findProjectRoot() (string, error) {
	// Try to find the project root by looking for go.mod
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}

	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			// Reached the root without finding go.mod
			return "", fmt.Errorf("could not find project root (go.mod)")
		}
		dir = parent
	}
}
