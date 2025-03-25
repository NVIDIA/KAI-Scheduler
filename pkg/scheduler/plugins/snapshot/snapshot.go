// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package snapshot

import (
	"archive/zip"
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/conf"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/framework"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/log"
)

const (
	SnapshotFileName = "snapshot.json"
)

type Snapshot struct {
	ClusterInfo     *api.ClusterInfo             `json:"snapshot"`
	Config          *conf.SchedulerConfiguration `json:"config"`
	SchedulerParams *conf.SchedulerParams        `json:"schedulerParams"`
}

type snapshotPlugin struct {
	session *framework.Session
}

func (sp *snapshotPlugin) Name() string {
	return "snapshot"
}

func (sp *snapshotPlugin) OnSessionOpen(ssn *framework.Session) {
	sp.session = ssn
	log.InfraLogger.V(3).Info("Snapshot plugin registering get-snapshot")
	ssn.AddHttpHandler("/get-snapshot", sp.serveSnapshot)
}

func (sp *snapshotPlugin) OnSessionClose(ssn *framework.Session) {}

func (sp *snapshotPlugin) serveSnapshot(writer http.ResponseWriter, request *http.Request) {
	snapshot, err := sp.session.Cache.Snapshot()
	if err != nil {
		http.Error(writer, err.Error(), http.StatusInternalServerError)
		return
	}
	snapshotAndConfig := Snapshot{
		ClusterInfo:     snapshot,
		Config:          sp.session.Config,
		SchedulerParams: &sp.session.SchedulerParams,
	}
	jsonBytes, err := json.Marshal(snapshotAndConfig)
	if err != nil {
		http.Error(writer, err.Error(), http.StatusInternalServerError)
		return
	}

	writer.Header().Set("Content-Disposition", "attachment; filename=snapshot.zip")
	writer.Header().Set("Content-Type", "application/zip")

	zipWriter := zip.NewWriter(writer)
	jsonWriter, err := zipWriter.Create(SnapshotFileName)
	if err != nil {
		http.Error(writer, err.Error(), http.StatusInternalServerError)
		return
	}

	_, err = io.Copy(jsonWriter, strings.NewReader(string(jsonBytes)))
	if err != nil {
		http.Error(writer, err.Error(), http.StatusInternalServerError)
		return
	}

	err = zipWriter.Close()
	if err != nil {
		http.Error(writer, err.Error(), http.StatusInternalServerError)
		return
	}
}

func New(arguments map[string]string) framework.Plugin {
	return &snapshotPlugin{}
}
