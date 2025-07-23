// Copyright 2023 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"net/http"

	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/common_info"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/plugins/proportion/resource_division"
	rs "github.com/NVIDIA/KAI-scheduler/pkg/scheduler/plugins/proportion/resource_share"
)

type SimulateRequest struct {
	TotalResource rs.ResourceQuantities              `json:"totalResource"`
	Queues        []resource_division.QueueOverrides `json:"queues"`
}

type QueueFairShare struct {
	GPU    float64 `json:"gpu"`
	CPU    float64 `json:"cpu"`
	Memory float64 `json:"memory"`
}

func simulateHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Only POST allowed", http.StatusMethodNotAllowed)
		return
	}

	var req SimulateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	queues := SimulateSetResourcesShare(req.TotalResource, req.Queues)

	resp := make(map[string]QueueFairShare)
	for id, qa := range queues {
		resp[string(id)] = QueueFairShare{
			GPU:    qa.GPU.FairShare,
			CPU:    qa.CPU.FairShare,
			Memory: qa.Memory.FairShare,
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func main() {
	var port = flag.Int("port", 8080, "Port to listen on")
	flag.Parse()
	http.HandleFunc("/simulate", simulateHandler)
	http.ListenAndServe(fmt.Sprintf(":%d", *port), nil)
}

func SimulateSetResourcesShare(totalResource rs.ResourceQuantities, queueOverrides []resource_division.QueueOverrides) map[common_info.QueueID]*rs.QueueAttributes {
	queues := make(map[common_info.QueueID]*rs.QueueAttributes)
	for _, qo := range queueOverrides {
		qa := qo.ToQueueAttributes()
		queues[qa.UID] = qa
	}
	resource_division.SetResourcesShare(totalResource, queues)
	return queues
}
