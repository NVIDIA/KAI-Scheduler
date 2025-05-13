// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package pytorch

import (
	"testing"

	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/NVIDIA/KAI-scheduler/pkg/podgrouper/podgrouper/plugins/defaultgrouper"
	"github.com/NVIDIA/KAI-scheduler/pkg/podgrouper/podgrouper/plugins/kubeflow"
)

const (
	minReplicasNum    = 66
	minAvailableNum   = 33
	workerReplicasNum = 4
	masterReplicasNum = 2
	queueLabelKey     = "kai.scheduler/queue"
)

func TestGetPodGroupMetadata_OnlyReplicas(t *testing.T) {
	pytorchJob := getBasicPytorchJob()
	pod := &v1.Pod{}
	defaultGrouper := defaultgrouper.NewDefaultGrouper(queueLabelKey)
	kubeFlowGrouper := kubeflow.NewKubeflowDistributedGrouper(defaultGrouper)
	grouper := NewPyTorchGrouper(kubeFlowGrouper)
	metadata, err := grouper.GetPodGroupMetadata(pytorchJob, pod)
	assert.Nil(t, err, "Got error when getting pytorch pod group metadata")
	assert.EqualValues(t, workerReplicasNum+masterReplicasNum, metadata.MinAvailable)
}

func TestGetPodGroupMetadata_OnlyMinReplicas(t *testing.T) {
	pytorchJob := getBasicPytorchJob()
	err := unstructured.SetNestedField(pytorchJob.Object, int64(minReplicasNum), "spec", "elasticPolicy", "minReplicas")
	assert.Nil(t, err, "Got error when setting minReplicas for pytorch job")

	pod := &v1.Pod{}
	defaultGrouper := defaultgrouper.NewDefaultGrouper(queueLabelKey)
	kubeFlowGrouper := kubeflow.NewKubeflowDistributedGrouper(defaultGrouper)
	grouper := NewPyTorchGrouper(kubeFlowGrouper)
	metadata, err := grouper.GetPodGroupMetadata(pytorchJob, pod)
	assert.Nil(t, err, "Got error when getting pytorch pod group metadata")
	assert.EqualValues(t, minReplicasNum, metadata.MinAvailable)
}

func TestGetPodGroupMetadata_OnlyMinAvailable(t *testing.T) {
	pytorchJob := getBasicPytorchJob()

	err := unstructured.SetNestedField(pytorchJob.Object, int64(minAvailableNum), "spec", "runPolicy", "schedulingPolicy", "minAvailable")
	assert.Nil(t, err, "Got error when setting minAvailable for pytorch job")

	pod := &v1.Pod{}
	defaultGrouper := defaultgrouper.NewDefaultGrouper(queueLabelKey)
	kubeFlowGrouper := kubeflow.NewKubeflowDistributedGrouper(defaultGrouper)
	grouper := NewPyTorchGrouper(kubeFlowGrouper)
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
	defaultGrouper := defaultgrouper.NewDefaultGrouper(queueLabelKey)
	kubeFlowGrouper := kubeflow.NewKubeflowDistributedGrouper(defaultGrouper)
	grouper := NewPyTorchGrouper(kubeFlowGrouper)
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
