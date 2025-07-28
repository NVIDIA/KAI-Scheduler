// Copyright 2024 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"

	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/common_info"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/plugins/proportion/resource_division"
	rs "github.com/NVIDIA/KAI-scheduler/pkg/scheduler/plugins/proportion/resource_share"
	"knative.dev/pkg/ptr"
)

// Job represents a pending job with resource requirements
type Job struct {
	UID               string                `json:"uid"`
	ResourceRequest   rs.ResourceQuantities `json:"resourceRequest"`
	RemainingDuration int                   `json:"remainingDuration"`
}

// QueueJobs represents the queue configuration and its pending jobs
type QueueJobs struct {
	Queue rs.QueueOverrides `json:"queue"`
	Jobs  []Job             `json:"jobs"`
}

// SimulationRequest represents the input for the iterative simulation
type SimulationRequest struct {
	TotalResource rs.ResourceQuantities `json:"totalResource"`
	QueuesJobs    []QueueJobs           `json:"queuesJobs"`
	K             float64               `json:"k"`
	WindowSize    *int32                `json:"windowSize"`
	RoundLimit    *int32                `json:"roundLimit"`
}

// JobAllocation represents the allocation status of a job
type JobAllocation struct {
	JobUID    string                `json:"jobUid"`
	Resources rs.ResourceQuantities `json:"resources"`
	QueueUID  string                `json:"queueUid"`
	Iteration int                   `json:"iteration"`
}

// IterationResult represents the result of a single iteration
type IterationResult struct {
	FairShare     map[string]rs.ResourceQuantities `json:"fairShare"`
	AllocatedJobs []JobAllocation                  `json:"allocatedJobs"`
	RemainingJobs map[string][]Job                 `json:"remainingJobs"`
	TotalUsage    rs.ResourceQuantities            `json:"totalUsage"`
}

// SimulationResponse represents the complete simulation results
type SimulationResponse struct {
	Iterations []IterationResult `json:"iterations"`
}

type server struct {
	enableCors bool
}

func (s *server) enableCorsHeaders(w *http.ResponseWriter) {
	(*w).Header().Set("Access-Control-Allow-Origin", "*")
	(*w).Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
	(*w).Header().Set("Access-Control-Allow-Headers", "Content-Type")
}

func (s *server) simulateHandler(w http.ResponseWriter, r *http.Request) {
	if s.enableCors {
		s.enableCorsHeaders(&w)
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}
	}

	if r.Method != "POST" {
		http.Error(w, "Only POST allowed", http.StatusMethodNotAllowed)
		return
	}

	var req SimulationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	result := runIterativeSimulation(req)

	w.Header().Set("Content-Type", "application/json")
	err := json.NewEncoder(w).Encode(result)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to encode response: %v", err), http.StatusInternalServerError)
		return
	}
}

func runIterativeSimulation(req SimulationRequest) SimulationResponse {

	if req.WindowSize == nil {
		req.WindowSize = ptr.Int32(10)
	}
	if req.RoundLimit == nil {
		req.RoundLimit = ptr.Int32(100)
	}

	response := SimulationResponse{
		Iterations: make([]IterationResult, 0),
	}

	// Initialize remaining jobs map
	remainingJobs := make(map[string][]Job)
	for _, qj := range req.QueuesJobs {
		remainingJobs[string(qj.Queue.UID)] = qj.Jobs
	}

	totalUsage := make(rs.ResourceQuantities)
	totalUsage["GPU"] = req.TotalResource["GPU"] * float64(*req.WindowSize)
	totalUsage["CPU"] = req.TotalResource["CPU"] * float64(*req.WindowSize)
	totalUsage["Memory"] = req.TotalResource["Memory"] * float64(*req.WindowSize)

	iteration := 0
	for hasRemainingJobs(remainingJobs) && iteration < int(*req.RoundLimit) {
		iteration++

		// Convert queues for simulation
		queues := make([]rs.QueueOverrides, 0)
		for _, qj := range req.QueuesJobs {
			queues = append(queues, qj.Queue)
		}

		window := SliceLast(response.Iterations, int(*req.WindowSize))
		populateAbsoluteUsage(window, queues)

		// Run fairshare simulation
		queueAttributes := SimulateSetResourcesShare(req.TotalResource, totalUsage, req.K, queues)

		// Process allocations for this iteration
		iterResult := processIteration(iteration, response.Iterations[len(response.Iterations)-1], queueAttributes, remainingJobs, totalUsage)
		response.Iterations = append(response.Iterations, iterResult)
	}

	return response
}

func SliceLast(slice []IterationResult, n int) []IterationResult {
	if len(slice) < n {
		return slice
	}
	return slice[len(slice)-n:]
}

func processIteration(clusterResource rs.ResourceQuantities, previousIteration IterationResult, queueAttributes map[common_info.QueueID]*rs.QueueAttributes, remainingJobs map[string][]Job, currentUsage rs.ResourceQuantities) IterationResult {
	result := IterationResult{
		FairShare:     make(map[string]rs.ResourceQuantities),
		AllocatedJobs: make([]JobAllocation, 0),
		RemainingJobs: make(map[string][]Job),
		TotalUsage:    make(rs.ResourceQuantities),
	}

	return result
}

func populateAbsoluteUsage(window []IterationResult, queues []rs.QueueOverrides) {
	absoluteUsage := make(map[string]rs.ResourceQuantities)
	for _, iteration := range window {
		for _, job := range iteration.AllocatedJobs {
			absoluteUsage[string(job.QueueUID)].Add(job.Resources)
		}
	}

	for _, queue := range queues {
		usage, ok := absoluteUsage[string(queue.UID)]
		if !ok {
			continue
		}
		queue.ResourceShare.GPU.AbsoluteUsage = ptr.Float64(usage["GPU"])
		queue.ResourceShare.CPU.AbsoluteUsage = ptr.Float64(usage["CPU"])
		queue.ResourceShare.Memory.AbsoluteUsage = ptr.Float64(usage["Memory"])
	}
}

func canAllocateJob(request, available rs.ResourceQuantities) bool {
	for resource, amount := range request {
		if available[resource] < amount {
			return false
		}
	}
	return true
}

func hasRemainingJobs(jobs map[string][]Job) bool {
	for _, queueJobs := range jobs {
		if len(queueJobs) > 0 {
			return true
		}
	}
	return false
}

func SimulateSetResourcesShare(totalResource rs.ResourceQuantities, totalUsage map[rs.ResourceName]float64, k float64, queueOverrides []rs.QueueOverrides) map[common_info.QueueID]*rs.QueueAttributes {
	queues := make(map[common_info.QueueID]*rs.QueueAttributes)
	for _, qo := range queueOverrides {
		qa := qo.ToQueueAttributes()
		queues[qa.UID] = qa
	}
	resource_division.SetResourcesShare(totalResource, totalUsage, k, queues)
	return queues
}

func main() {
	var port = flag.Int("port", 8081, "Port to listen on")
	var enableCors = flag.Bool("enable-cors", false, "Enable CORS headers for cross-origin requests")
	flag.Parse()

	s := &server{
		enableCors: *enableCors,
	}

	http.HandleFunc("/simulate", s.simulateHandler)
	log.Printf("Starting iterative simulator server on port %d (CORS enabled: %v)...", *port, *enableCors)
	err := http.ListenAndServe(fmt.Sprintf(":%d", *port), nil)
	if err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
