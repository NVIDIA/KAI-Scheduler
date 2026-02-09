// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package pytorch

import (
	"fmt"
	"strings"

	pytorchv1 "github.com/kubeflow/training-operator/pkg/apis/kubeflow.org/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/NVIDIA/KAI-scheduler/pkg/podgrouper/podgroup"
	"github.com/NVIDIA/KAI-scheduler/pkg/podgrouper/podgrouper/plugins/kubeflow"
)

const (
	replicaSpecName  = "pytorchReplicaSpecs"
	replicaTypeLabel = pytorchv1.ReplicaTypeLabel

	replicaTypeMaster = pytorchv1.PyTorchJobReplicaTypeMaster
	replicaTypeWorker = pytorchv1.PyTorchJobReplicaTypeWorker
)

type PyTorchGrouper struct {
	*kubeflow.KubeflowDistributedGrouper
}

func NewPyTorchGrouper(kubeflowGrouper *kubeflow.KubeflowDistributedGrouper) *PyTorchGrouper {
	return &PyTorchGrouper{
		KubeflowDistributedGrouper: kubeflowGrouper,
	}
}

func (ptg *PyTorchGrouper) Name() string {
	return "PyTorchJob Grouper"
}

func (ptg *PyTorchGrouper) GetPodGroupMetadata(
	topOwner *unstructured.Unstructured, pod *v1.Pod, _ ...*metav1.PartialObjectMetadata,
) (*podgroup.Metadata, error) {
	podGroupMetadata, err := ptg.KubeflowDistributedGrouper.GetPodGroupMetadata(topOwner, pod, replicaSpecName, []string{})
	if err != nil {
		return nil, err
	}

	minReplicas, err := getMinReplicas(topOwner)
	if err == nil {
		podGroupMetadata.MinAvailable = int32(minReplicas)
	}

	minAvailable, err := getMinAvailable(topOwner)
	if err == nil {
		podGroupMetadata.MinAvailable = int32(minAvailable)
	}

	subGroups, err := ptg.buildSubGroups(topOwner, pod, podGroupMetadata.MinAvailable)
	if err != nil {
		return nil, err
	}
	podGroupMetadata.SubGroups = subGroups

	return podGroupMetadata, nil
}

func (ptg *PyTorchGrouper) buildSubGroups(
	topOwner *unstructured.Unstructured, pod *v1.Pod, totalMinAvailable int32,
) ([]*podgroup.SubGroupMetadata, error) {
	replicaSpecs, found, err := unstructured.NestedMap(topOwner.Object, "spec", "pytorchReplicaSpecs")
	if err != nil {
		return nil, fmt.Errorf("failed to get pytorchReplicaSpecs from PyTorchJob %s/%s. Err: %w", topOwner.GetNamespace(), topOwner.GetName(), err)
	}
	if !found {
		return nil, fmt.Errorf("pytorchReplicaSpecs not found in PyTorchJob %s/%s", topOwner.GetNamespace(), topOwner.GetName())
	}

	masterReplicas, found, err := unstructured.NestedInt64(replicaSpecs, string(replicaTypeMaster), "replicas")
	if err != nil {
		return nil, fmt.Errorf("failed to get replicas from pytorchReplicaSpecs[%s] in PyTorchJob %s/%s. Err: %w", string(replicaTypeMaster), topOwner.GetNamespace(), topOwner.GetName(), err)
	}
	if !found {
		masterReplicas = 0
	}

	var subGroups []*podgroup.SubGroupMetadata

	masterSubGroup := buildMasterSubGroup(replicaSpecs, pod, int32(masterReplicas))
	if masterSubGroup != nil {
		subGroups = append(subGroups, masterSubGroup)
	}

	workerMinAvailable := max(0, totalMinAvailable-int32(masterReplicas))
	workerSubGroup := buildWorkerSubGroup(replicaSpecs, pod, workerMinAvailable)
	if workerSubGroup != nil {
		subGroups = append(subGroups, workerSubGroup)
	}

	return subGroups, nil
}

func buildMasterSubGroup(replicaSpecs map[string]interface{}, pod *v1.Pod, masterReplicas int32) *podgroup.SubGroupMetadata {
	if _, exists := replicaSpecs[string(replicaTypeMaster)]; !exists {
		return nil
	}
	if masterReplicas == 0 {
		return nil
	}

	var podReferences []string
	if pod.Labels[replicaTypeLabel] == strings.ToLower(string(replicaTypeMaster)) {
		podReferences = append(podReferences, pod.Name)
	}

	return &podgroup.SubGroupMetadata{
		Name:           string(replicaTypeMaster),
		MinAvailable:   masterReplicas,
		PodsReferences: podReferences,
	}
}

func buildWorkerSubGroup(replicaSpecs map[string]interface{}, pod *v1.Pod, workerMinAvailable int32) *podgroup.SubGroupMetadata {
	if _, exists := replicaSpecs[string(replicaTypeWorker)]; !exists {
		return nil
	}

	var podReferences []string
	if pod.Labels[replicaTypeLabel] == strings.ToLower(string(replicaTypeWorker)) {
		podReferences = append(podReferences, pod.Name)
	}

	return &podgroup.SubGroupMetadata{
		Name:           string(replicaTypeWorker),
		MinAvailable:   workerMinAvailable,
		PodsReferences: podReferences,
	}
}

func getMinReplicas(topOwner *unstructured.Unstructured) (int64, error) {
	minReplicas, found, err := unstructured.NestedInt64(topOwner.Object, "spec", "elasticPolicy", "minReplicas")
	if err != nil {
		return 0, err
	}
	if !found {
		return 0, fmt.Errorf("minReplicas not found in PyTorchJob %s/%s", topOwner.GetNamespace(), topOwner.GetName())
	}
	return minReplicas, nil
}

func getMinAvailable(topOwner *unstructured.Unstructured) (int64, error) {
	minReplicas, found, err := unstructured.NestedInt64(topOwner.Object, "spec", "runPolicy", "schedulingPolicy", "minAvailable")
	if err != nil {
		return 0, err
	}
	if !found {
		return 0, fmt.Errorf("minAvailable not found in PyTorchJob %s/%s", topOwner.GetNamespace(), topOwner.GetName())
	}
	return minReplicas, nil
}
