// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package controllers

import (
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"

	"github.com/NVIDIA/KAI-scheduler/pkg/podgrouper/podgroup"
	pluginconstants "github.com/NVIDIA/KAI-scheduler/pkg/podgrouper/podgrouper/plugins/constants"
)

// enrichMetadata enriches the PodGroup metadata with node pool labels and user-requested subgroups.
func enrichMetadata(metadata *podgroup.Metadata, pod *v1.Pod, topOwner *unstructured.Unstructured, configs Configs) {
	if len(configs.NodePoolLabelKey) > 0 {
		addNodePoolLabel(metadata, pod, configs.NodePoolLabelKey)
	}

	handleRequestedSubgroups(metadata, pod, topOwner)
}

func handleRequestedSubgroups(metadata *podgroup.Metadata, pod *v1.Pod, topOwner *unstructured.Unstructured) {
	if topOwner != nil {
		if createSubgroupName, found := getCreateSubgroupAnnotation(topOwner); found {
			ensureRequestedSubgroupExists(metadata, createSubgroupName)
		}
	}

	if requestedSubgroup, found := getRequestedSubgroupAnnotation(pod); found {
		assignPodToSubgroup(metadata, pod, requestedSubgroup)
	}
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

func getCreateSubgroupAnnotation(topOwner *unstructured.Unstructured) (string, bool) {
	annotations := topOwner.GetAnnotations()
	if annotations == nil {
		return "", false
	}

	createSubgroupName, found := annotations[pluginconstants.CreateSubgroupAnnotationKey]
	if !found || createSubgroupName == "" {
		return "", false
	}

	// Validate: "default" is reserved
	if createSubgroupName == "default" {
		return "", false
	}

	return createSubgroupName, true
}

func ensureRequestedSubgroupExists(metadata *podgroup.Metadata, subgroupName string) {
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

func getRequestedSubgroupAnnotation(pod *v1.Pod) (string, bool) {
	if pod.Annotations == nil {
		return "", false
	}

	requestedSubgroup, found := pod.Annotations[pluginconstants.RequestedSubgroupAnnotationKey]
	if !found || requestedSubgroup == "" {
		return "", false
	}

	return requestedSubgroup, true
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
