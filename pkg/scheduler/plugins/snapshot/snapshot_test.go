// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package snapshot

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"

	kubeaischedulerver "github.com/NVIDIA/KAI-scheduler/pkg/apis/client/clientset/versioned/fake"
	enginev2 "github.com/NVIDIA/KAI-scheduler/pkg/apis/scheduling/v2"
	enginev2alpha2 "github.com/NVIDIA/KAI-scheduler/pkg/apis/scheduling/v2alpha2"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/cache"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/conf"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/framework"
)

const (
	NodePoolLabelKey   = "test-key"
	NodePoolLabelValue = "test-value"
)

func TestSnapshotPlugin(t *testing.T) {
	fakeKubeClient := fake.NewSimpleClientset()
	fakeKubeAISchedulerClient := kubeaischedulerver.NewSimpleClientset()

	// Create test resources
	testPod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod",
			Namespace: "default",
		},
		Spec: v1.PodSpec{
			Containers: []v1.Container{
				{
					Name:  "test-container",
					Image: "test-image",
				},
			},
		},
	}

	testNodes := []*v1.Node{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test-node",
				Labels: map[string]string{
					NodePoolLabelKey: NodePoolLabelValue,
				},
			},
			Spec: v1.NodeSpec{
				Unschedulable: false,
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test-node-other-shard",
				Labels: map[string]string{
					NodePoolLabelKey: "other",
				},
			},
			Spec: v1.NodeSpec{
				Unschedulable: false,
			},
		},
	}

	testQueue := &enginev2.Queue{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-queue",
			Labels: map[string]string{
				NodePoolLabelKey: NodePoolLabelValue,
			},
		},
		Spec: enginev2.QueueSpec{
			DisplayName: "test-queue",
			Resources: &enginev2.QueueResources{
				GPU: enginev2.QueueResource{
					Quota:           1,
					OverQuotaWeight: 1,
					Limit:           1,
				},
				CPU: enginev2.QueueResource{
					Quota:           1,
					OverQuotaWeight: 1,
					Limit:           1,
				},
				Memory: enginev2.QueueResource{
					Quota:           1,
					OverQuotaWeight: 1,
					Limit:           1,
				},
			},
		},
	}

	testPodGroup := &enginev2alpha2.PodGroup{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-podgroup",
			Namespace: "default",
			Labels: map[string]string{
				NodePoolLabelKey: NodePoolLabelValue,
			},
		},
		Spec: enginev2alpha2.PodGroupSpec{
			MinMember: 1,
		},
	}

	// Create scheduler configuration
	schedulerConfig := &conf.SchedulerConfiguration{
		Actions: "allocate",
		Tiers: []conf.Tier{
			{
				Plugins: []conf.PluginOption{
					{
						Name: "predicates",
					},
				},
			},
		},
	}

	schedulerParams := conf.SchedulerParams{
		SchedulerName: "test-scheduler",
		PartitionParams: &conf.SchedulingNodePoolParams{
			NodePoolLabelKey:   NodePoolLabelKey,
			NodePoolLabelValue: NodePoolLabelValue,
		},
	}

	// Create cache
	schedulerCache := cache.New(&cache.SchedulerCacheParams{
		KubeClient:                  fakeKubeClient,
		KubeAISchedulerClient:       fakeKubeAISchedulerClient,
		SchedulerName:               schedulerParams.SchedulerName,
		NodePoolParams:              schedulerParams.PartitionParams,
		RestrictNodeScheduling:      false,
		DetailedFitErrors:           false,
		ScheduleCSIStorage:          false,
		FullHierarchyFairness:       true,
		NodeLevelScheduler:          false,
		NumOfStatusRecordingWorkers: 1,
	})

	// Start cache
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Create test resources in fake clients after cache is started and synced
	_, err := fakeKubeClient.CoreV1().Pods("default").Create(ctx, testPod, metav1.CreateOptions{})
	assert.NoError(t, err)

	for _, node := range testNodes {
		_, err = fakeKubeClient.CoreV1().Nodes().Create(ctx, node, metav1.CreateOptions{})
		assert.NoError(t, err)
	}

	_, err = fakeKubeAISchedulerClient.SchedulingV2().Queues("").Create(ctx, testQueue, metav1.CreateOptions{})
	assert.NoError(t, err)

	_, err = fakeKubeAISchedulerClient.SchedulingV2alpha2().PodGroups("default").Create(ctx, testPodGroup, metav1.CreateOptions{})
	assert.NoError(t, err)

	schedulerCache.Run(ctx.Done())
	schedulerCache.WaitForCacheSync(ctx.Done())

	// Create session
	session := &framework.Session{
		Config:          schedulerConfig,
		SchedulerParams: schedulerParams,
		Cache:           schedulerCache,
	}

	// Create snapshot plugin
	plugin := New(nil)
	plugin.OnSessionOpen(session)

	// Create test HTTP request and response writer
	req := httptest.NewRequest("GET", "/get-snapshot", nil)
	w := httptest.NewRecorder()

	// Call serveSnapshot
	plugin.(*snapshotPlugin).serveSnapshot(w, req)

	// Check response status
	assert.Equal(t, http.StatusOK, w.Code)

	// Parse response
	var snapshot Snapshot
	err = json.Unmarshal(w.Body.Bytes(), &snapshot)
	assert.NoError(t, err)

	// Verify configuration
	assert.Equal(t, schedulerConfig.Actions, snapshot.Config.Actions)
	assert.Equal(t, schedulerParams.SchedulerName, snapshot.SchedulerParams.SchedulerName)
	assert.Equal(t, schedulerParams.PartitionParams.NodePoolLabelKey, snapshot.SchedulerParams.PartitionParams.NodePoolLabelKey)
	assert.Equal(t, schedulerParams.PartitionParams.NodePoolLabelValue, snapshot.SchedulerParams.PartitionParams.NodePoolLabelValue)

	// Verify resources in snapshot
	assert.NotNil(t, snapshot.RawObjects)

	// Verify Pods
	assert.Len(t, snapshot.RawObjects.Pods, 1)
	assert.Equal(t, testPod.Name, snapshot.RawObjects.Pods[0].Name)
	assert.Equal(t, testPod.Namespace, snapshot.RawObjects.Pods[0].Namespace)

	// Verify Nodes
	assert.Len(t, snapshot.RawObjects.Nodes, 1)
	if len(snapshot.RawObjects.Nodes) > 0 {
		assert.Equal(t, testNodes[0].Name, snapshot.RawObjects.Nodes[0].Name)
	}

	// Verify Queues
	assert.Len(t, snapshot.RawObjects.Queues, 1)
	if len(snapshot.RawObjects.Queues) > 0 {
		assert.Equal(t, testQueue.Name, snapshot.RawObjects.Queues[0].Name)
	}

	// Verify PodGroups
	assert.Len(t, snapshot.RawObjects.PodGroups, 1)
	if len(snapshot.RawObjects.PodGroups) > 0 {
		assert.Equal(t, testPodGroup.Name, snapshot.RawObjects.PodGroups[0].Name)
		assert.Equal(t, testPodGroup.Namespace, snapshot.RawObjects.PodGroups[0].Namespace)
	}
}
