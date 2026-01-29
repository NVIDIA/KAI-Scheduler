// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package pytorch

import (
	"testing"

	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/NVIDIA/KAI-scheduler/pkg/podgrouper/podgroup"
	"github.com/NVIDIA/KAI-scheduler/pkg/podgrouper/podgrouper/plugins/defaultgrouper"
	"github.com/NVIDIA/KAI-scheduler/pkg/podgrouper/podgrouper/plugins/kubeflow"
)

const (
	minReplicasNum    = 66
	minAvailableNum   = 33
	workerReplicasNum = 4
	masterReplicasNum = 2
	queueLabelKey     = "kai.scheduler/queue"
	nodePoolLabelKey  = "kai.scheduler/node-pool"
)

func newTestPyTorchGrouper() *PyTorchGrouper {
	defaultGrouper := defaultgrouper.NewDefaultGrouper(queueLabelKey, nodePoolLabelKey, fake.NewFakeClient())
	kubeFlowGrouper := kubeflow.NewKubeflowDistributedGrouper(defaultGrouper)
	return NewPyTorchGrouper(kubeFlowGrouper)
}

func TestGetPodGroupMetadata_OnlyReplicas(t *testing.T) {
	pytorchJob := getBasicPytorchJob()
	pod := &v1.Pod{}
	grouper := newTestPyTorchGrouper()
	metadata, err := grouper.GetPodGroupMetadata(pytorchJob, pod)
	assert.Nil(t, err, "Got error when getting pytorch pod group metadata")
	assert.EqualValues(t, workerReplicasNum+masterReplicasNum, metadata.MinAvailable)
}

func TestGetPodGroupMetadata_OnlyMinReplicas(t *testing.T) {
	pytorchJob := getBasicPytorchJob()
	err := unstructured.SetNestedField(pytorchJob.Object, int64(minReplicasNum), "spec", "elasticPolicy", "minReplicas")
	assert.Nil(t, err, "Got error when setting minReplicas for pytorch job")

	pod := &v1.Pod{}
	grouper := newTestPyTorchGrouper()
	metadata, err := grouper.GetPodGroupMetadata(pytorchJob, pod)
	assert.Nil(t, err, "Got error when getting pytorch pod group metadata")
	assert.EqualValues(t, minReplicasNum, metadata.MinAvailable)
}

func TestGetPodGroupMetadata_OnlyMinAvailable(t *testing.T) {
	pytorchJob := getBasicPytorchJob()

	err := unstructured.SetNestedField(pytorchJob.Object, int64(minAvailableNum), "spec", "runPolicy", "schedulingPolicy", "minAvailable")
	assert.Nil(t, err, "Got error when setting minAvailable for pytorch job")

	pod := &v1.Pod{}
	grouper := newTestPyTorchGrouper()
	metadata, err := grouper.GetPodGroupMetadata(pytorchJob, pod)

	assert.Nil(t, err, "Got error when getting pytorch pod group metadata")
	assert.EqualValues(t, minAvailableNum, metadata.MinAvailable)
}

// TestGetPodGroupMetadata_MinAvailableAndMinReplicas tests the case when both minAvailable and minReplicas are set -
// minAvailable should be used as the value for MinAvailable in the metadata.
func TestGetPodGroupMetadata_MinAvailableAndMinReplicas(t *testing.T) {
	pytorchJob := getBasicPytorchJob()

	err := unstructured.SetNestedField(pytorchJob.Object, int64(minReplicasNum), "spec", "elasticPolicy", "minReplicas")
	assert.Nil(t, err, "Got error when setting minReplicas for pytorch job")

	err = unstructured.SetNestedField(pytorchJob.Object, int64(minAvailableNum), "spec", "runPolicy", "schedulingPolicy", "minAvailable")
	assert.Nil(t, err, "Got error when setting minAvailable for pytorch job")

	pod := &v1.Pod{}
	grouper := newTestPyTorchGrouper()
	metadata, err := grouper.GetPodGroupMetadata(pytorchJob, pod)
	assert.Nil(t, err, "Got error when getting pytorch pod group metadata")
	assert.EqualValues(t, minAvailableNum, metadata.MinAvailable)
}

// TestGetPodGroupMetadata_OnlyMasterReplicas tests a PyTorch job with only Master replicas and no Worker replicas
func TestGetPodGroupMetadata_OnlyMasterReplicas(t *testing.T) {
	pytorchJob := getPytorchJobWithOnlyMaster()
	pod := &v1.Pod{}
	grouper := newTestPyTorchGrouper()
	metadata, err := grouper.GetPodGroupMetadata(pytorchJob, pod)
	assert.Nil(t, err, "Got error when getting pytorch pod group metadata")
	assert.EqualValues(t, masterReplicasNum, metadata.MinAvailable)
}

// TestGetPodGroupMetadata_OnlyWorkerReplicas tests a PyTorch job with only Worker replicas and no Master replicas
func TestGetPodGroupMetadata_OnlyWorkerReplicas(t *testing.T) {
	pytorchJob := getPytorchJobWithOnlyWorker()
	pod := &v1.Pod{}
	grouper := newTestPyTorchGrouper()
	metadata, err := grouper.GetPodGroupMetadata(pytorchJob, pod)
	assert.Nil(t, err, "Got error when getting pytorch pod group metadata")
	assert.EqualValues(t, workerReplicasNum, metadata.MinAvailable)
}

// TestGetPodGroupMetadata_OnlyMasterWithMinReplicas tests a PyTorch job with only Master replicas
// and a specified minReplicas value
func TestGetPodGroupMetadata_OnlyMasterWithMinReplicas(t *testing.T) {
	pytorchJob := getPytorchJobWithOnlyMaster()
	err := unstructured.SetNestedField(pytorchJob.Object, int64(minReplicasNum), "spec", "elasticPolicy", "minReplicas")
	assert.Nil(t, err, "Got error when setting minReplicas for pytorch job")

	pod := &v1.Pod{}
	grouper := newTestPyTorchGrouper()
	metadata, err := grouper.GetPodGroupMetadata(pytorchJob, pod)
	assert.Nil(t, err, "Got error when getting pytorch pod group metadata")
	assert.EqualValues(t, minReplicasNum, metadata.MinAvailable)
}

// TestGetPodGroupMetadata_OnlyWorkerWithMinReplicas tests a PyTorch job with only Worker replicas
// and a specified minReplicas value
func TestGetPodGroupMetadata_OnlyWorkerWithMinReplicas(t *testing.T) {
	pytorchJob := getPytorchJobWithOnlyWorker()
	err := unstructured.SetNestedField(pytorchJob.Object, int64(minReplicasNum), "spec", "elasticPolicy", "minReplicas")
	assert.Nil(t, err, "Got error when setting minReplicas for pytorch job")

	pod := &v1.Pod{}
	grouper := newTestPyTorchGrouper()
	metadata, err := grouper.GetPodGroupMetadata(pytorchJob, pod)
	assert.Nil(t, err, "Got error when getting pytorch pod group metadata")
	assert.EqualValues(t, minReplicasNum, metadata.MinAvailable)
}

// TestGetPodGroupMetadata_OnlyMasterWithMinAvailable tests a PyTorch job with only Master replicas
// and a specified minAvailable value
func TestGetPodGroupMetadata_OnlyMasterWithMinAvailable(t *testing.T) {
	pytorchJob := getPytorchJobWithOnlyMaster()
	err := unstructured.SetNestedField(pytorchJob.Object, int64(minAvailableNum), "spec", "runPolicy", "schedulingPolicy", "minAvailable")
	assert.Nil(t, err, "Got error when setting minAvailable for pytorch job")

	pod := &v1.Pod{}
	grouper := newTestPyTorchGrouper()
	metadata, err := grouper.GetPodGroupMetadata(pytorchJob, pod)
	assert.Nil(t, err, "Got error when getting pytorch pod group metadata")
	assert.EqualValues(t, minAvailableNum, metadata.MinAvailable)
}

// TestGetPodGroupMetadata_OnlyWorkerWithMinAvailable tests a PyTorch job with only Worker replicas
// and a specified minAvailable value
func TestGetPodGroupMetadata_OnlyWorkerWithMinAvailable(t *testing.T) {
	pytorchJob := getPytorchJobWithOnlyWorker()
	err := unstructured.SetNestedField(pytorchJob.Object, int64(minAvailableNum), "spec", "runPolicy", "schedulingPolicy", "minAvailable")
	assert.Nil(t, err, "Got error when setting minAvailable for pytorch job")

	pod := &v1.Pod{}
	grouper := newTestPyTorchGrouper()
	metadata, err := grouper.GetPodGroupMetadata(pytorchJob, pod)
	assert.Nil(t, err, "Got error when getting pytorch pod group metadata")
	assert.EqualValues(t, minAvailableNum, metadata.MinAvailable)
}

func getBasicPytorchJob() *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"kind":       "PyTorchJob",
			"apiVersion": "kubeflow.org/v1",
			"metadata": map[string]interface{}{
				"name":      "test_job",
				"namespace": "test_namespace",
				"uid":       "1",
			},
			"spec": map[string]interface{}{
				"pytorchReplicaSpecs": map[string]interface{}{
					"Master": map[string]interface{}{
						"replicas": int64(masterReplicasNum),
					},
					"Worker": map[string]interface{}{
						"replicas": int64(workerReplicasNum),
					},
				},
			},
		},
	}
}

// getPytorchJobWithOnlyMaster returns a PyTorch job configuration with only Master replicas
func getPytorchJobWithOnlyMaster() *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"kind":       "PyTorchJob",
			"apiVersion": "kubeflow.org/v1",
			"metadata": map[string]interface{}{
				"name":      "test_job_master_only",
				"namespace": "test_namespace",
				"uid":       "2",
			},
			"spec": map[string]interface{}{
				"pytorchReplicaSpecs": map[string]interface{}{
					"Master": map[string]interface{}{
						"replicas": int64(masterReplicasNum),
					},
				},
			},
		},
	}
}

// getPytorchJobWithOnlyWorker returns a PyTorch job configuration with only Worker replicas
func getPytorchJobWithOnlyWorker() *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"kind":       "PyTorchJob",
			"apiVersion": "kubeflow.org/v1",
			"metadata": map[string]interface{}{
				"name":      "test_job_worker_only",
				"namespace": "test_namespace",
				"uid":       "3",
			},
			"spec": map[string]interface{}{
				"pytorchReplicaSpecs": map[string]interface{}{
					"Worker": map[string]interface{}{
						"replicas": int64(workerReplicasNum),
					},
				},
			},
		},
	}
}

func TestGetPodGroupMetadata_SubGroups_MasterAndWorker(t *testing.T) {
	pytorchJob := getBasicPytorchJob()
	pod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod-master-0",
			Namespace: "test_namespace",
			Labels: map[string]string{
				ReplicaTypeLabel: "master",
			},
		},
	}
	grouper := newTestPyTorchGrouper()
	metadata, err := grouper.GetPodGroupMetadata(pytorchJob, pod)
	assert.Nil(t, err)

	assert.Equal(t, 2, len(metadata.SubGroups))

	masterSubGroup := findSubGroupByName(metadata.SubGroups, string(ReplicaTypeMaster))
	assert.NotNil(t, masterSubGroup)
	assert.Equal(t, 1, len(masterSubGroup.PodsReferences))
	assert.Equal(t, "test-pod-master-0", masterSubGroup.PodsReferences[0].Name)
	assert.Equal(t, "test_namespace", masterSubGroup.PodsReferences[0].Namespace)

	workerSubGroup := findSubGroupByName(metadata.SubGroups, string(ReplicaTypeWorker))
	assert.NotNil(t, workerSubGroup)
	assert.Equal(t, 0, len(workerSubGroup.PodsReferences))
}

func TestGetPodGroupMetadata_SubGroups_WorkerPod(t *testing.T) {
	pytorchJob := getBasicPytorchJob()
	pod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod-worker-0",
			Namespace: "test_namespace",
			Labels: map[string]string{
				ReplicaTypeLabel: "worker",
			},
		},
	}
	grouper := newTestPyTorchGrouper()
	metadata, err := grouper.GetPodGroupMetadata(pytorchJob, pod)
	assert.Nil(t, err)

	assert.Equal(t, 2, len(metadata.SubGroups))

	masterSubGroup := findSubGroupByName(metadata.SubGroups, string(ReplicaTypeMaster))
	assert.NotNil(t, masterSubGroup)
	assert.Equal(t, 0, len(masterSubGroup.PodsReferences))

	workerSubGroup := findSubGroupByName(metadata.SubGroups, string(ReplicaTypeWorker))
	assert.NotNil(t, workerSubGroup)
	assert.Equal(t, 1, len(workerSubGroup.PodsReferences))
	assert.Equal(t, "test-pod-worker-0", workerSubGroup.PodsReferences[0].Name)
}

func TestGetPodGroupMetadata_SubGroups_OnlyMaster(t *testing.T) {
	pytorchJob := getPytorchJobWithOnlyMaster()
	pod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod-master-0",
			Namespace: "test_namespace",
			Labels: map[string]string{
				ReplicaTypeLabel: "master",
			},
		},
	}
	grouper := newTestPyTorchGrouper()
	metadata, err := grouper.GetPodGroupMetadata(pytorchJob, pod)
	assert.Nil(t, err)

	assert.Equal(t, 1, len(metadata.SubGroups))

	masterSubGroup := findSubGroupByName(metadata.SubGroups, string(ReplicaTypeMaster))
	assert.NotNil(t, masterSubGroup)
	assert.Equal(t, 1, len(masterSubGroup.PodsReferences))
	assert.Equal(t, "test-pod-master-0", masterSubGroup.PodsReferences[0].Name)
}

func TestGetPodGroupMetadata_SubGroups_OnlyWorker(t *testing.T) {
	pytorchJob := getPytorchJobWithOnlyWorker()
	pod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod-worker-0",
			Namespace: "test_namespace",
			Labels: map[string]string{
				ReplicaTypeLabel: "worker",
			},
		},
	}
	grouper := newTestPyTorchGrouper()
	metadata, err := grouper.GetPodGroupMetadata(pytorchJob, pod)
	assert.Nil(t, err)

	assert.Equal(t, 1, len(metadata.SubGroups))

	workerSubGroup := findSubGroupByName(metadata.SubGroups, string(ReplicaTypeWorker))
	assert.NotNil(t, workerSubGroup)
	assert.Equal(t, 1, len(workerSubGroup.PodsReferences))
	assert.Equal(t, "test-pod-worker-0", workerSubGroup.PodsReferences[0].Name)
}

func TestGetPodGroupMetadata_SubGroups_PodWithoutReplicaTypeLabel(t *testing.T) {
	pytorchJob := getBasicPytorchJob()
	pod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod-0",
			Namespace: "test_namespace",
		},
	}
	grouper := newTestPyTorchGrouper()
	metadata, err := grouper.GetPodGroupMetadata(pytorchJob, pod)
	assert.Nil(t, err)

	assert.Equal(t, 2, len(metadata.SubGroups))
	for _, subGroup := range metadata.SubGroups {
		assert.Equal(t, 0, len(subGroup.PodsReferences))
	}
}

func findSubGroupByName(subGroups []*podgroup.SubGroupMetadata, name string) *podgroup.SubGroupMetadata {
	for _, sg := range subGroups {
		if sg.Name == name {
			return sg
		}
	}
	return nil
}
