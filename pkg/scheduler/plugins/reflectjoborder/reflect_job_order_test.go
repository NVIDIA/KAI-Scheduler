// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package reflectjoborder

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/common_info"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/podgroup_info"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/queue_info"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/conf"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/framework"
)

func TestJobOrderPlugin_OnSessionOpen(t *testing.T) {
	ssn := &framework.Session{
		ClusterInfo: &api.ClusterInfo{PodGroupInfos: map[common_info.PodGroupID]*podgroup_info.PodGroupInfo{
			"pg1": {UID: "pg1", Priority: 5, Queue: "q1"},
			"pg2": {UID: "pg2", Priority: 2, Queue: "q2"},
		}},
		Config: &conf.SchedulerConfiguration{
			QueueDepthPerAction: map[string]int{"Allocate": 10},
		},
	}
	plugin := &JobOrderPlugin{}
	plugin.OnSessionOpen(ssn)

	if plugin.ReflectJobOrder == nil {
		t.Fatalf("ReflectJobOrder should be initialized")
	}
}

// Test serveJobs returns correct JSON and status when ReflectJobOrder is set
func TestServeJobs_ReflectJobOrderReady(t *testing.T) {
	plugin := &JobOrderPlugin{
		ReflectJobOrder: &ReflectJobOrder{
			GlobalOrder: []JobOrder{{ID: "pg1", Priority: 10}},
			QueueOrder:  map[common_info.QueueID][]JobOrder{"q1": {{ID: "pg1", Priority: 10}}},
		},
	}
	req := httptest.NewRequest(http.MethodGet, "/get-job-order", nil)
	rr := httptest.NewRecorder()
	plugin.serveJobs(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("Expected HTTP 200 OK, got %d", rr.Code)
	}
	var resp ReflectJobOrder
	if err := json.NewDecoder(bytes.NewReader(rr.Body.Bytes())).Decode(&resp); err != nil {
		t.Fatalf("Failed to decode serveJobs response: %v", err)
	}
	if len(resp.GlobalOrder) != 1 || resp.GlobalOrder[0].Priority != 10 {
		t.Errorf("Unexpected response json: %+v", resp)
	}
	if len(resp.QueueOrder) != 1 {
		t.Errorf("Expected 1 queue, got %d", len(resp.QueueOrder))
	}
}

// Test serveJobs returns 503 if ReflectJobOrder is nil
func TestServeJobs_ReflectJobOrderNotReady(t *testing.T) {
	plugin := &JobOrderPlugin{ReflectJobOrder: nil}
	req := httptest.NewRequest(http.MethodGet, "/get-job-order", nil)
	rr := httptest.NewRecorder()
	plugin.serveJobs(rr, req)
	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected HTTP 503, got %d", rr.Code)
	}
	if !bytes.Contains(rr.Body.Bytes(), []byte("Job order data not ready")) {
		t.Errorf("Expected error message in body, got: %s", rr.Body.String())
	}
}

// Test serveJobs handles encoding error gracefully
type brokenWriter struct{ http.ResponseWriter }

func (b *brokenWriter) Write(_ []byte) (int, error) { return 0, errEncode }

var errEncode = &encodeError{"forced encode error"}

type encodeError struct{ msg string }

func (e *encodeError) Error() string { return e.msg }

func TestServeJobs_EncodeError(t *testing.T) {
	plugin := &JobOrderPlugin{ReflectJobOrder: &ReflectJobOrder{}}
	req := httptest.NewRequest(http.MethodGet, "/get-job-order", nil)
	rr := httptest.NewRecorder()
	bw := &brokenWriter{rr}
	plugin.serveJobs(bw, req)
	// Should write 500 error
	if rr.Code != http.StatusInternalServerError {
		t.Errorf("Expected HTTP 500, got %d", rr.Code)
	}
}

func newTestSession() *framework.Session {
	return &framework.Session{
		ClusterInfo: &api.ClusterInfo{
			PodGroupInfos: map[common_info.PodGroupID]*podgroup_info.PodGroupInfo{
				"pg1": {UID: "pg1", Priority: 5, Queue: "q1"},
				"pg2": {UID: "pg2", Priority: 2, Queue: "q2"},
			},
			Queues: map[common_info.QueueID]*queue_info.QueueInfo{
				"q1": {UID: "q1", Name: "q1"},
				"q2": {UID: "q2", Name: "q2"},
			},
			QueueResourceUsage: *queue_info.NewClusterUsage(),
		},
		Config: &conf.SchedulerConfiguration{
			QueueDepthPerAction: map[string]int{"Allocate": 10},
		},
	}
}

func TestNewBuilder_CacheHit(t *testing.T) {
	ssn := newTestSession()

	builder := NewBuilder()
	plugin1 := builder(nil).(*JobOrderPlugin)
	plugin1.OnSessionOpen(ssn)

	if plugin1.ReflectJobOrder == nil {
		t.Fatal("first OnSessionOpen should produce a ReflectJobOrder")
	}
	firstResult := plugin1.ReflectJobOrder

	plugin2 := builder(nil).(*JobOrderPlugin)
	plugin2.OnSessionOpen(ssn)

	if plugin2.ReflectJobOrder != firstResult {
		t.Error("second OnSessionOpen should reuse the cached ReflectJobOrder pointer")
	}
}

func TestNewBuilder_CacheMissOnChange(t *testing.T) {
	ssn := newTestSession()

	builder := NewBuilder()
	plugin1 := builder(nil).(*JobOrderPlugin)
	plugin1.OnSessionOpen(ssn)
	firstResult := plugin1.ReflectJobOrder

	ssn.ClusterInfo.PodGroupInfos["pg3"] = &podgroup_info.PodGroupInfo{
		UID: "pg3", Priority: 1, Queue: "q1",
	}

	plugin2 := builder(nil).(*JobOrderPlugin)
	plugin2.OnSessionOpen(ssn)

	if plugin2.ReflectJobOrder == firstResult {
		t.Error("OnSessionOpen after data change should recompute, not reuse cached pointer")
	}
}

func TestNilCache_NoErrors(t *testing.T) {
	ssn := newTestSession()

	plugin := &JobOrderPlugin{}
	plugin.OnSessionOpen(ssn)

	if plugin.ReflectJobOrder == nil {
		t.Fatal("OnSessionOpen without cache should still produce a ReflectJobOrder")
	}
}

func TestComputeFingerprint_Deterministic(t *testing.T) {
	ssn := newTestSession()

	fp1 := computeFingerprint(ssn)
	fp2 := computeFingerprint(ssn)

	if fp1 != fp2 {
		t.Errorf("computeFingerprint should be deterministic: got %d and %d", fp1, fp2)
	}
}

func TestComputeFingerprint_ChangesOnMutation(t *testing.T) {
	ssn := newTestSession()
	fp1 := computeFingerprint(ssn)

	ssn.ClusterInfo.PodGroupInfos["pg3"] = &podgroup_info.PodGroupInfo{
		UID: "pg3", Priority: 1, Queue: "q1",
	}
	fp2 := computeFingerprint(ssn)

	if fp1 == fp2 {
		t.Error("computeFingerprint should differ after adding a new PodGroupInfo")
	}
}
