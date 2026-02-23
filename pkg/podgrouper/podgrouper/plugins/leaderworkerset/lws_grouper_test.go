// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package leader_worker_set

import (
	"testing"

	"github.com/NVIDIA/KAI-scheduler/pkg/podgrouper/podgroup"
	"github.com/NVIDIA/KAI-scheduler/pkg/podgrouper/podgrouper/plugins/defaultgrouper"
	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func baseOwner(name string, startupPolicy string, replicas int64) *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"kind":       "LeaderWorkerSet",
			"apiVersion": "leaderworkerset.x-k8s.io/v1",
			"metadata": map[string]interface{}{
				"name":      name,
				"namespace": "default",
				"uid":       name + "-uid",
			},
			"spec": map[string]interface{}{
				"startupPolicy": startupPolicy,
				"leaderWorkerTemplate": map[string]interface{}{
					"size": replicas,
				},
			},
		},
	}
}

func TestGetPodGroupMetadata_LeaderCreated(t *testing.T) {
	owner := baseOwner("lws-test", "LeaderCreated", 3)

	pod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "lws-test-0-1",
			Namespace: "default",
			Labels:    map[string]string{},
		},
	}

	lwsGrouper := NewLwsGrouper(defaultgrouper.NewDefaultGrouper("", "", fake.NewFakeClient()))
	podGroupMetadata, err := lwsGrouper.GetPodGroupMetadata(owner, pod)

	assert.Nil(t, err)
	assert.Equal(t, int32(3), podGroupMetadata.MinAvailable)
	assert.Equal(t, "LeaderWorkerSet", podGroupMetadata.Owner.Kind)
	assert.Equal(t, "leaderworkerset.x-k8s.io/v1", podGroupMetadata.Owner.APIVersion)
	assert.Equal(t, "lws-test", podGroupMetadata.Owner.Name)
	assert.Equal(t, "lws-test-uid", string(podGroupMetadata.Owner.UID))

	assert.Equal(t, 2, len(podGroupMetadata.SubGroups))
	leaderSubGroup := findSubGroupByName(podGroupMetadata.SubGroups, "leader")
	assert.NotNil(t, leaderSubGroup)
	assert.Equal(t, int32(1), leaderSubGroup.MinAvailable)
	assert.Equal(t, 0, len(leaderSubGroup.PodsReferences))
	workersSubGroup := findSubGroupByName(podGroupMetadata.SubGroups, "workers")
	assert.NotNil(t, workersSubGroup)
	assert.Equal(t, int32(2), workersSubGroup.MinAvailable)
	assert.Equal(t, 1, len(workersSubGroup.PodsReferences))
	assert.Equal(t, "lws-test-0-1", workersSubGroup.PodsReferences[0])
}

func TestGetPodGroupMetadata_LeaderReady_LeaderPod(t *testing.T) {
	owner := baseOwner("lws-ready", "LeaderReady", 5)

	pod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "lws-ready-0-0",
			Namespace: "default",
			Labels: map[string]string{
				"leaderworkerset.sigs.k8s.io/worker-index": "0",
			},
		},
		Spec: v1.PodSpec{NodeName: ""},
	}

	lwsGrouper := NewLwsGrouper(defaultgrouper.NewDefaultGrouper("", "", fake.NewFakeClient()))
	podGroupMetadata, err := lwsGrouper.GetPodGroupMetadata(owner, pod)

	assert.Nil(t, err)
	assert.Equal(t, int32(1), podGroupMetadata.MinAvailable)
	assert.Equal(t, 1, len(podGroupMetadata.SubGroups))
	leaderSubGroup := findSubGroupByName(podGroupMetadata.SubGroups, "leader")
	assert.NotNil(t, leaderSubGroup)
	assert.Equal(t, int32(1), leaderSubGroup.MinAvailable)
	assert.Equal(t, 1, len(leaderSubGroup.PodsReferences))
	assert.Equal(t, "lws-ready-0-0", leaderSubGroup.PodsReferences[0])
}

func TestGetPodGroupMetadata_LeaderReady_WorkerPod(t *testing.T) {
	owner := baseOwner("lws-ready", "LeaderReady", 5)

	pod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "lws-ready-0-2",
			Namespace: "default",
			Annotations: map[string]string{
				"leaderworkerset.sigs.k8s.io/size": "5",
			},
			Labels: map[string]string{
				"leaderworkerset.sigs.k8s.io/group-index":  "0",
				"leaderworkerset.sigs.k8s.io/worker-index": "2",
			},
		},
		Spec: v1.PodSpec{NodeName: "worker-node"},
	}

	lwsGrouper := NewLwsGrouper(defaultgrouper.NewDefaultGrouper("", "", fake.NewFakeClient()))
	podGroupMetadata, err := lwsGrouper.GetPodGroupMetadata(owner, pod)

	assert.Nil(t, err)
	assert.Equal(t, int32(5), podGroupMetadata.MinAvailable)
	assert.Equal(t, 2, len(podGroupMetadata.SubGroups))
	workersSubGroup := findSubGroupByName(podGroupMetadata.SubGroups, "workers")
	assert.NotNil(t, workersSubGroup)
	assert.Equal(t, int32(4), workersSubGroup.MinAvailable)
	assert.Equal(t, 1, len(workersSubGroup.PodsReferences))
	assert.Equal(t, "lws-ready-0-2", workersSubGroup.PodsReferences[0])
}

func TestGetPodGroupMetadata_GroupIndex_Label(t *testing.T) {
	owner := baseOwner("lws-grouped", "LeaderCreated", 2)

	pod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Labels: map[string]string{
				"leaderworkerset.sigs.k8s.io/group-index": "1",
			},
		},
	}

	lwsGrouper := NewLwsGrouper(defaultgrouper.NewDefaultGrouper("", "", fake.NewFakeClient()))
	podGroupMetadata, err := lwsGrouper.GetPodGroupMetadata(owner, pod)

	assert.Nil(t, err)
	assert.Contains(t, podGroupMetadata.Name, "-group-1")
}

func TestGetPodGroupMetadata_SubGroups_LeaderPod(t *testing.T) {
	owner := baseOwner("lws-subgroups", "LeaderCreated", 3)
	pod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "lws-subgroups-0-0",
			Namespace: "default",
			Labels: map[string]string{
				"leaderworkerset.sigs.k8s.io/worker-index": "0",
			},
		},
	}
	lwsGrouper := NewLwsGrouper(defaultgrouper.NewDefaultGrouper("", "", fake.NewFakeClient()))
	metadata, err := lwsGrouper.GetPodGroupMetadata(owner, pod)
	assert.Nil(t, err)

	assert.Equal(t, 2, len(metadata.SubGroups))
	leaderSubGroup := findSubGroupByName(metadata.SubGroups, "leader")
	assert.NotNil(t, leaderSubGroup)
	assert.Equal(t, int32(1), leaderSubGroup.MinAvailable)
	assert.Equal(t, 1, len(leaderSubGroup.PodsReferences))
	assert.Equal(t, "lws-subgroups-0-0", leaderSubGroup.PodsReferences[0])

	workersSubGroup := findSubGroupByName(metadata.SubGroups, "workers")
	assert.NotNil(t, workersSubGroup)
	assert.Equal(t, int32(2), workersSubGroup.MinAvailable)
	assert.Equal(t, 0, len(workersSubGroup.PodsReferences))
}

func TestGetPodGroupMetadata_SubGroups_WorkerPod(t *testing.T) {
	owner := baseOwner("lws-subgroups", "LeaderCreated", 3)
	pod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "lws-subgroups-0-1",
			Namespace: "default",
			Labels: map[string]string{
				"leaderworkerset.sigs.k8s.io/worker-index": "1",
			},
		},
	}
	lwsGrouper := NewLwsGrouper(defaultgrouper.NewDefaultGrouper("", "", fake.NewFakeClient()))
	metadata, err := lwsGrouper.GetPodGroupMetadata(owner, pod)
	assert.Nil(t, err)

	assert.Equal(t, 2, len(metadata.SubGroups))
	leaderSubGroup := findSubGroupByName(metadata.SubGroups, "leader")
	assert.NotNil(t, leaderSubGroup)
	assert.Equal(t, int32(1), leaderSubGroup.MinAvailable)
	assert.Equal(t, 0, len(leaderSubGroup.PodsReferences))

	workersSubGroup := findSubGroupByName(metadata.SubGroups, "workers")
	assert.NotNil(t, workersSubGroup)
	assert.Equal(t, int32(2), workersSubGroup.MinAvailable)
	assert.Equal(t, 1, len(workersSubGroup.PodsReferences))
	assert.Equal(t, "lws-subgroups-0-1", workersSubGroup.PodsReferences[0])
}

func TestGetPodGroupMetadata_SubGroups_OnlyLeader(t *testing.T) {
	owner := baseOwner("lws-single", "LeaderCreated", 1)
	pod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "lws-single-0-0",
			Namespace: "default",
			Labels: map[string]string{
				"leaderworkerset.sigs.k8s.io/worker-index": "0",
			},
		},
	}
	lwsGrouper := NewLwsGrouper(defaultgrouper.NewDefaultGrouper("", "", fake.NewFakeClient()))
	metadata, err := lwsGrouper.GetPodGroupMetadata(owner, pod)
	assert.Nil(t, err)

	assert.Equal(t, 1, len(metadata.SubGroups))
	leaderSubGroup := findSubGroupByName(metadata.SubGroups, "leader")
	assert.NotNil(t, leaderSubGroup)
	assert.Equal(t, int32(1), leaderSubGroup.MinAvailable)
	assert.Equal(t, 1, len(leaderSubGroup.PodsReferences))
	assert.Equal(t, "lws-single-0-0", leaderSubGroup.PodsReferences[0])
}

func findSubGroupByName(subGroups []*podgroup.SubGroupMetadata, name string) *podgroup.SubGroupMetadata {
	for _, sg := range subGroups {
		if sg.Name == name {
			return sg
		}
	}
	return nil
}
