package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"

	kaiv1alpha1 "github.com/NVIDIA/KAI-scheduler/pkg/apis/kai/v1alpha1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

type CreateTopologyRequest struct {
	Name        string           `json:"name"`
	Levels      []string         `json:"levels"`
	Assignments []NodeAssignment `json:"assignments"`
}

type NodeAssignment struct {
	Node   string   `json:"node"`
	Values []string `json:"values"` // ordered values matching Levels
}

type CreateTopologyResponse struct {
	TopologyName string   `json:"topologyName"`
	Created      bool     `json:"created"`
	PatchedNodes int      `json:"patchedNodes"`
	Errors       []string `json:"errors,omitempty"`
}

func (s *server) handleTopology(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req CreateTopologyRequest
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

	levels := make([]string, 0, len(req.Levels))
	seen := map[string]struct{}{}
	for _, l := range req.Levels {
		l = strings.TrimSpace(l)
		if l == "" {
			continue
		}
		if _, ok := seen[l]; ok {
			http.Error(w, fmt.Sprintf("levels must be unique (duplicate %q)", l), http.StatusBadRequest)
			return
		}
		seen[l] = struct{}{}
		levels = append(levels, l)
	}
	if len(levels) == 0 {
		http.Error(w, "levels must have at least 1 entry", http.StatusBadRequest)
		return
	}
	if len(levels) > 16 {
		http.Error(w, "levels must have at most 16 entries", http.StatusBadRequest)
		return
	}
	if i := indexOf(levels, "kubernetes.io/hostname"); i >= 0 && i != len(levels)-1 {
		http.Error(w, "kubernetes.io/hostname can only be used at the lowest level", http.StatusBadRequest)
		return
	}

	levelsSpec := make([]kaiv1alpha1.TopologyLevel, 0, len(levels))
	for _, l := range levels {
		levelsSpec = append(levelsSpec, kaiv1alpha1.TopologyLevel{NodeLabel: l})
	}

	topo := &kaiv1alpha1.Topology{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec:       kaiv1alpha1.TopologySpec{Levels: levelsSpec},
	}

	resp := CreateTopologyResponse{TopologyName: name, Created: false, PatchedNodes: 0, Errors: nil}

	_, err := s.kai.KaiV1alpha1().Topologies().Create(r.Context(), topo, metav1.CreateOptions{})
	if err != nil {
		if apierrors.IsAlreadyExists(err) {
			// Spec is immutable; don't attempt to update.
			http.Error(w, fmt.Sprintf("topology %q already exists (levels are immutable); delete it first if you need different levels", name), http.StatusConflict)
			return
		}
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	resp.Created = true

	// Patch node labels to match the given values per level.
	for _, a := range req.Assignments {
		nodeName := strings.TrimSpace(a.Node)
		if nodeName == "" {
			continue
		}

		vals := a.Values
		if len(vals) != len(levels) {
			resp.Errors = append(resp.Errors, fmt.Sprintf("%s: expected %d values (one per level), got %d", nodeName, len(levels), len(vals)))
			continue
		}

		labels := make(map[string]string, len(levels))
		for i, key := range levels {
			v := strings.TrimSpace(vals[i])
			if key == "kubernetes.io/hostname" && v == "" {
				v = nodeName
			}
			if v == "" {
				resp.Errors = append(resp.Errors, fmt.Sprintf("%s: empty value for level %q", nodeName, key))
				labels = nil
				break
			}
			labels[key] = v
		}
		if labels == nil {
			continue
		}

		if err := patchNodeLabels(r.Context(), s, nodeName, labels); err != nil {
			resp.Errors = append(resp.Errors, err.Error())
			continue
		}
		resp.PatchedNodes++
	}

	if resp.Errors != nil {
		sort.Strings(resp.Errors)
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

func patchNodeLabels(ctx context.Context, s *server, nodeName string, labels map[string]string) error {
	payload := map[string]interface{}{"metadata": map[string]interface{}{"labels": labels}}
	b, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("%s: marshal patch: %v", nodeName, err)
	}

	_, err = s.kube.CoreV1().Nodes().Patch(ctx, nodeName, types.MergePatchType, b, metav1.PatchOptions{})
	if err != nil {
		return fmt.Errorf("%s: patch node labels: %v", nodeName, err)
	}
	return nil
}

func indexOf(xs []string, s string) int {
	for i := range xs {
		if xs[i] == s {
			return i
		}
	}
	return -1
}
