package main

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	snapshotplugin "github.com/NVIDIA/KAI-scheduler/pkg/scheduler/plugins/snapshot"
)

func LoadSnapshot(ctx context.Context, snapshotURL, snapshotFile string) (*snapshotplugin.Snapshot, error) {
	var data []byte
	var err error

	switch {
	case snapshotURL != "":
		data, err = download(ctx, snapshotURL)
	case snapshotFile != "":
		data, err = os.ReadFile(snapshotFile)
	default:
		return nil, errors.New("no snapshot source provided")
	}
	if err != nil {
		return nil, err
	}

	jsonBytes, err := extractSnapshotJSON(data)
	if err != nil {
		return nil, err
	}

	var snap snapshotplugin.Snapshot
	if err := json.Unmarshal(jsonBytes, &snap); err != nil {
		return nil, fmt.Errorf("unmarshal snapshot.json: %w", err)
	}
	return &snap, nil
}

func download(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 8<<10))
		return nil, fmt.Errorf("GET %s: %s: %s", url, resp.Status, strings.TrimSpace(string(b)))
	}

	return io.ReadAll(resp.Body)
}

func extractSnapshotJSON(data []byte) ([]byte, error) {
	// Most commonly: zip file containing snapshot.json.
	if jb, err := extractFromZip(data); err == nil {
		return jb, nil
	}

	// Fallback: raw JSON.
	if json.Valid(data) {
		return data, nil
	}

	return nil, errors.New("unsupported snapshot format (expected zip with snapshot.json or raw JSON)")
}

func extractFromZip(data []byte) ([]byte, error) {
	r, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, err
	}

	var best *zip.File
	for _, f := range r.File {
		base := filepath.Base(f.Name)
		if base == snapshotplugin.SnapshotFileName {
			best = f
			break
		}
		if strings.HasSuffix(strings.ToLower(base), ".json") {
			best = f
		}
	}
	if best == nil {
		return nil, errors.New("zip does not contain snapshot.json")
	}

	rc, err := best.Open()
	if err != nil {
		return nil, err
	}
	defer rc.Close()

	return io.ReadAll(rc)
}
