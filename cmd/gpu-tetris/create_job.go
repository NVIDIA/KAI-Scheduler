package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	batchv1 "k8s.io/api/batch/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type CreateJobRequest struct {
	Namespace   string `json:"namespace"`
	Name        string `json:"name"`
	Queue       string `json:"queue"`
	Mode        string `json:"mode"` // "whole" | "fraction" | "memory"
	Parallelism int    `json:"parallelism"`
	Completions int    `json:"completions"`

	GPUCount          int     `json:"gpuCount"`
	GPUFraction       float64 `json:"gpuFraction"`
	FractionNumDevice int     `json:"fractionNumDevices"`
	GPUMemoryMiB      int     `json:"gpuMemoryMiB"`

	Image string `json:"image"`
}

type CreateJobResponse struct {
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
}

func (s *server) handleJob(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		s.handleCreateJob(w, r)
	case http.MethodDelete:
		s.handleDeleteTetrisJobs(w, r)
	default:
		w.Header().Set("Allow", strings.Join([]string{http.MethodPost, http.MethodDelete}, ", "))
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *server) handleCreateJob(w http.ResponseWriter, r *http.Request) {
	var req CreateJobRequest
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf("invalid JSON: %v", err), http.StatusBadRequest)
		return
	}

	ns := strings.TrimSpace(req.Namespace)
	if ns == "" {
		ns = s.cfg.defaultNS
	}

	name := strings.TrimSpace(req.Name)
	if name == "" {
		name = fmt.Sprintf("gpu-tetris-%d", time.Now().Unix())
	}

	queue := strings.TrimSpace(req.Queue)
	if queue == "" {
		queue = s.cfg.defaultQueue
	}

	image := strings.TrimSpace(req.Image)
	if image == "" {
		image = s.cfg.defaultImage
	}

	mode := strings.TrimSpace(req.Mode)
	if mode == "" {
		mode = "whole"
	}

	parallelism := int32(req.Parallelism)
	if parallelism <= 0 {
		parallelism = 1
	}

	completions := int32(req.Completions)
	if completions <= 0 {
		completions = 1
	}

	labels := map[string]string{
		"kai.scheduler/queue": queue,
		tetrisCreatedLabelKey: tetrisCreatedLabelValue,
		"job-name":            name,
	}

	podAnnotations := map[string]string{}

	container := v1.Container{
		Name:    "workload",
		Image:   image,
		Command: []string{"sleep"},
		Args:    []string{"60"},
	}

	switch mode {
	case "whole":
		gpuCount := req.GPUCount
		if gpuCount <= 0 {
			gpuCount = 1
		}
		q := resource.NewQuantity(int64(gpuCount), resource.DecimalSI)
		container.Resources.Limits = v1.ResourceList{
			v1.ResourceName("nvidia.com/gpu"): *q,
		}
		container.Resources.Requests = v1.ResourceList{
			v1.ResourceName("nvidia.com/gpu"): *q,
		}

	case "fraction":
		frac := req.GPUFraction
		if frac <= 0 || frac >= 1 {
			http.Error(w, "gpuFraction must be >0 and <1", http.StatusBadRequest)
			return
		}
		podAnnotations["gpu-fraction"] = strconv.FormatFloat(frac, 'g', 3, 64)
		if req.FractionNumDevice > 1 {
			podAnnotations["gpu-fraction-num-devices"] = strconv.Itoa(req.FractionNumDevice)
		}

	case "memory":
		mem := req.GPUMemoryMiB
		if mem <= 0 {
			http.Error(w, "gpuMemoryMiB must be >0", http.StatusBadRequest)
			return
		}
		podAnnotations["gpu-memory"] = strconv.Itoa(mem)

	default:
		http.Error(w, "mode must be one of: whole, fraction, memory", http.StatusBadRequest)
		return
	}

	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: ns,
			Labels: map[string]string{
				tetrisCreatedLabelKey: tetrisCreatedLabelValue,
			},
		},
		Spec: batchv1.JobSpec{
			Parallelism: &parallelism,
			Completions: &completions,
			Template: v1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels:      labels,
					Annotations: podAnnotations,
				},
				Spec: v1.PodSpec{
					SchedulerName: s.cfg.schedulerName,
					RestartPolicy: v1.RestartPolicyNever,
					Containers:    []v1.Container{container},
				},
			},
		},
	}

	created, err := s.kube.BatchV1().Jobs(ns).Create(r.Context(), job, metav1.CreateOptions{})
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(CreateJobResponse{Namespace: created.Namespace, Name: created.Name})
}

type DeleteTetrisJobsResponse struct {
	Deleted int      `json:"deleted"`
	Errors  []string `json:"errors,omitempty"`
}

func (s *server) handleDeleteTetrisJobs(w http.ResponseWriter, r *http.Request) {
	selector := fmt.Sprintf("%s=%s", tetrisCreatedLabelKey, tetrisCreatedLabelValue)
	list, err := s.kube.BatchV1().Jobs("").List(r.Context(), metav1.ListOptions{LabelSelector: selector})
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}

	policy := metav1.DeletePropagationForeground
	deleteOpts := metav1.DeleteOptions{PropagationPolicy: &policy}

	resp := DeleteTetrisJobsResponse{Deleted: 0, Errors: nil}
	for i := range list.Items {
		j := &list.Items[i]
		err := s.kube.BatchV1().Jobs(j.Namespace).Delete(r.Context(), j.Name, deleteOpts)
		if err != nil {
			resp.Errors = append(resp.Errors, fmt.Sprintf("%s/%s: %v", j.Namespace, j.Name, err))
			continue
		}
		resp.Deleted++
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}
