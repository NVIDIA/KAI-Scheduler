package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type CreatePodRequest struct {
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
	Queue     string `json:"queue"`
	Mode      string `json:"mode"` // "whole" | "fraction" | "memory"

	GPUCount          int     `json:"gpuCount"`
	GPUFraction       float64 `json:"gpuFraction"`
	FractionNumDevice int     `json:"fractionNumDevices"`
	GPUMemoryMiB      int     `json:"gpuMemoryMiB"`

	Image string `json:"image"`
}

type CreatePodResponse struct {
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
}

func (s *server) handleCreatePod(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req CreatePodRequest
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

	pod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: ns,
			Labels: map[string]string{
				"kai.scheduler/queue": queue,
			},
			Annotations: map[string]string{},
		},
		Spec: v1.PodSpec{
			SchedulerName: s.cfg.schedulerName,
			RestartPolicy: v1.RestartPolicyNever,
			Containers: []v1.Container{
				{
					Name:    "workload",
					Image:   image,
					Command: []string{"sleep"},
					Args:    []string{"3600"},
				},
			},
		},
	}

	switch mode {
	case "whole":
		gpuCount := req.GPUCount
		if gpuCount <= 0 {
			gpuCount = 1
		}
		q := resource.NewQuantity(int64(gpuCount), resource.DecimalSI)
		pod.Spec.Containers[0].Resources.Limits = v1.ResourceList{
			v1.ResourceName("nvidia.com/gpu"): *q,
		}
		pod.Spec.Containers[0].Resources.Requests = v1.ResourceList{
			v1.ResourceName("nvidia.com/gpu"): *q,
		}

	case "fraction":
		frac := req.GPUFraction
		if frac <= 0 || frac >= 1 {
			http.Error(w, "gpuFraction must be >0 and <1", http.StatusBadRequest)
			return
		}
		pod.ObjectMeta.Annotations["gpu-fraction"] = strconv.FormatFloat(frac, 'g', 3, 64)
		if req.FractionNumDevice > 1 {
			pod.ObjectMeta.Annotations["gpu-fraction-num-devices"] = strconv.Itoa(req.FractionNumDevice)
		}

	case "memory":
		mem := req.GPUMemoryMiB
		if mem <= 0 {
			http.Error(w, "gpuMemoryMiB must be >0", http.StatusBadRequest)
			return
		}
		pod.ObjectMeta.Annotations["gpu-memory"] = strconv.Itoa(mem)

	default:
		http.Error(w, "mode must be one of: whole, fraction, memory", http.StatusBadRequest)
		return
	}

	created, err := s.kube.CoreV1().Pods(ns).Create(r.Context(), pod, metav1.CreateOptions{})
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(CreatePodResponse{Namespace: created.Namespace, Name: created.Name})
}
