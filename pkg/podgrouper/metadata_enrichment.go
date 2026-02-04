// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package controllers

import (
	"github.com/go-logr/logr"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"

	"github.com/NVIDIA/KAI-scheduler/pkg/podgrouper/podgroup"
	pluginconstants "github.com/NVIDIA/KAI-scheduler/pkg/podgrouper/podgrouper/plugins/constants"
)

func enrichMetadata(metadata *podgroup.Metadata, pod *v1.Pod, topOwner *unstructured.Unstructured, configs Configs, logger logr.Logger) {
	if len(configs.NodePoolLabelKey) > 0 {
		addNodePoolLabel(metadata, pod, configs.NodePoolLabelKey)
	}
	handleSubgroupCreationRequest(topOwner, metadata, logger)
	handlePodSubgroupAssignmentRequest(pod, metadata)
}

func addNodePoolLabel(metadata *podgroup.Metadata, pod *v1.Pod, nodePoolKey string) {
	if metadata.Labels == nil {
		metadata.Labels = map[string]string{}
	}

	if _, found := metadata.Labels[nodePoolKey]; found {
		return
	}

	if pod.Labels == nil {
		pod.Labels = map[string]string{}
	}

	if labelValue, found := pod.Labels[nodePoolKey]; found {
		metadata.Labels[nodePoolKey] = labelValue
	}
}

func handleSubgroupCreationRequest(topOwner *unstructured.Unstructured, metadata *podgroup.Metadata, logger logr.Logger) {
	if topOwner == nil {
		return
	}

	annotations := topOwner.GetAnnotations()
	if annotations == nil {
		return
	}

	subgroupName := annotations[pluginconstants.CreateSubgroupAnnotationKey]
	if subgroupName == "" || subgroupName == "default" { // "default" is reserved
		return
	}

	// Skip subgroup creation for workloads that may create multiple PodGroups.
	// For these workloads, it's ambiguous which PodGroup the auxiliary subgroup should belong to.
	if isMultiPodGroupWorkload(topOwner) {
		logger.Info("Skipping create-subgroup annotation: workload type may create multiple PodGroups",
			"kind", topOwner.GetKind(),
			"name", topOwner.GetName(),
			"namespace", topOwner.GetNamespace(),
			"requestedSubgroup", subgroupName)
		return
	}

	ensureSubgroupExists(metadata, subgroupName)
}

// GuyContinue
func isMultiPodGroupWorkload(topOwner *unstructured.Unstructured) bool {
	kind := topOwner.GetKind()

	switch kind {
	case "Job":
		// Deployments create one PodGroup per pod (not gang scheduled)
		return true
	case "JobSet":
		// JobSet creates multiple PodGroups when using InOrder startup policy (default)
		order, _, _ := unstructured.NestedString(topOwner.Object, "spec", "startupPolicy", "startupPolicyOrder")
		return order == "" || order == "InOrder"
	}
	return false
}

func handlePodSubgroupAssignmentRequest(pod *v1.Pod, metadata *podgroup.Metadata) {
	if pod.Annotations == nil {
		return
	}

	requestedSubgroup := pod.Annotations[pluginconstants.RequestedSubgroupAnnotationKey]
	if requestedSubgroup == "" {
		return
	}

	assignPodToSubgroup(metadata, pod, requestedSubgroup)
}

func ensureSubgroupExists(metadata *podgroup.Metadata, subgroupName string) {
	subgroupExists := false
	for _, sg := range metadata.SubGroups {
		if sg.Name == subgroupName {
			subgroupExists = true
			break
		}
	}

	if subgroupExists {
		return
	}

	if len(metadata.SubGroups) == 0 {
		originalMinAvailable := metadata.MinAvailable
		defaultSubGroup := &podgroup.SubGroupMetadata{
			Name:         "default",
			MinAvailable: originalMinAvailable,
		}
		metadata.SubGroups = append(metadata.SubGroups, defaultSubGroup)
	}

	requestedSubGroup := &podgroup.SubGroupMetadata{
		Name:         subgroupName,
		MinAvailable: 1,
	}
	metadata.SubGroups = append(metadata.SubGroups, requestedSubGroup)

	// Note: Incrementing MinAvailable here is primarily for observability and does not affect scheduling behavior.
	metadata.MinAvailable = metadata.MinAvailable + 1
}

func assignPodToSubgroup(metadata *podgroup.Metadata, pod *v1.Pod, subgroupName string) {
	for _, sg := range metadata.SubGroups {
		if sg.Name == subgroupName {
			podRef := types.NamespacedName{Namespace: pod.Namespace, Name: pod.Name}
			sg.PodsReferences = append(sg.PodsReferences, &podRef)
			break
		}
	}
}
