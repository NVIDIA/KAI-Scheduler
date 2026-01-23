// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package snapshots_test

import (
	"path/filepath"
	"testing"

	"github.com/NVIDIA/KAI-scheduler/pkg/snapshotrunner"
)

// TestSnapshotsFromFiles discovers snapshot archives under
// pkg/scheduler/actions/integration_tests/snapshots/testdata and runs the
// scheduler actions on each of them using the snapshotrunner.
//
// To add a new snapshot-based integration test, place a snapshot archive
// (for example snapshot.zip or snapshot.gzip) under:
//
//	pkg/scheduler/actions/integration_tests/snapshots/testdata
//
// The test will automatically pick it up and execute the configured actions.
func TestSnapshotsFromFiles(t *testing.T) {
	const testdataDir = "testdata"

	patterns := []string{
		filepath.Join(testdataDir, "*.zip"),
		filepath.Join(testdataDir, "*.gzip"),
		filepath.Join(testdataDir, "*.gz"),
	}

	var files []string
	for _, pattern := range patterns {
		matches, err := filepath.Glob(pattern)
		if err != nil {
			t.Fatalf("failed to glob pattern %q: %v", pattern, err)
		}
		files = append(files, matches...)
	}

	if len(files) == 0 {
		t.Skipf("no snapshot test files found under %s", testdataDir)
	}

	for _, snapshotFile := range files {
		snapshotFile := snapshotFile
		t.Run(filepath.Base(snapshotFile), func(t *testing.T) {
			t.Parallel()

			opts := snapshotrunner.Options{
				Filename:  snapshotFile,
				Verbosity: 4,
			}

			if err := snapshotrunner.Run(opts); err != nil {
				t.Fatalf("snapshotrunner.Run(%q) failed: %v", snapshotFile, err)
			}
		})
	}
}
