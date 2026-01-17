package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	enginev2 "github.com/NVIDIA/KAI-scheduler/pkg/apis/scheduling/v2"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type CreateQueueRequest struct {
	Name        string  `json:"name"`
	DisplayName string  `json:"displayName"`
	ParentQueue string  `json:"parentQueue"`
	Priority    *int    `json:"priority"`
	GPUQuota    float64 `json:"gpuQuota"`
}

type CreateQueueResponse struct {
	Name    string `json:"name"`
	Created bool   `json:"created"`
}

func (s *server) handleQueue(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req CreateQueueRequest
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf("invalid JSON: %v", err), http.StatusBadRequest)
		return
	}

	name := strings.TrimSpace(req.Name)
	if name == "" {
		http.Error(w, "name is required", http.StatusBadRequest)
		return
	}

	// Build the Queue object
	queue := &enginev2.Queue{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: enginev2.QueueSpec{},
	}

	// Set optional fields
	if displayName := strings.TrimSpace(req.DisplayName); displayName != "" {
		queue.Spec.DisplayName = displayName
	}

	if parentQueue := strings.TrimSpace(req.ParentQueue); parentQueue != "" {
		queue.Spec.ParentQueue = parentQueue
	}

	if req.Priority != nil {
		queue.Spec.Priority = req.Priority
	}

	if req.GPUQuota > 0 {
		queue.Spec.Resources = &enginev2.QueueResources{
			GPU: enginev2.QueueResource{
				Quota: req.GPUQuota,
			},
		}
	}

	// Create the queue
	_, err := s.kai.SchedulingV2().Queues("").Create(r.Context(), queue, metav1.CreateOptions{})
	if err != nil {
		if apierrors.IsAlreadyExists(err) {
			http.Error(w, fmt.Sprintf("queue %q already exists", name), http.StatusConflict)
			return
		}
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(CreateQueueResponse{Name: name, Created: true})
}
