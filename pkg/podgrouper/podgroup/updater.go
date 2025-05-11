// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package podgroup

import (
	"reflect"

	enginev2alpha2 "github.com/NVIDIA/KAI-scheduler/pkg/apis/scheduling/v2alpha2"
)

const (
	pgWorkloadStatus             = "runai-calculated-status"
	pgRunningPodsAnnotationName  = "runai-running-pods"
	pgPendingPodsAnnotationName  = "runai-pending-pods"
	pgUsedNodes                  = "runai-used-nodes"
	pgRequestedGPUs              = "runai-podgroup-requested-gpus"
	pgRequestedGPUsMemory        = "runai-podgroup-requested-gpus-memory"
	pgTotalRequestedGPUs         = "runai-total-requested-gpus"
	pgTotalRequestedGPUsMemory   = "runai-total-requested-gpus-memory"
	pgCurrentRequestedGPUs       = "runai-current-requested-gpus"
	pgCurrentRequestedGPUsMemory = "runai-current-requested-gpus-memory"
	pgAllocatedGPUs              = "runai-current-allocated-gpus"
	pgAllocatedGPUsMemory        = "runai-current-allocated-gpus-memory"
	pgRequestedMigDevices        = "runai-podgroup-requested-mig-devices"
)

var notToUpdateAnnotations = map[string]bool{
	pgWorkloadStatus:             true,
	pgRunningPodsAnnotationName:  true,
	pgPendingPodsAnnotationName:  true,
	pgUsedNodes:                  true,
	pgRequestedGPUs:              true,
	pgRequestedGPUsMemory:        true,
	pgTotalRequestedGPUs:         true,
	pgTotalRequestedGPUsMemory:   true,
	pgCurrentRequestedGPUs:       true,
	pgCurrentRequestedGPUsMemory: true,
	pgAllocatedGPUs:              true,
	pgAllocatedGPUsMemory:        true,
	pgRequestedMigDevices:        true,
}

func podGroupsEqual(leftPodGroup, rightPodGroup *enginev2alpha2.PodGroup) bool {
	leftPgUpdatableAnnotations := removeBlocklistedKeys(leftPodGroup.Annotations)
	rightPgUpdatableAnnotations := removeBlocklistedKeys(rightPodGroup.Annotations)

	return reflect.DeepEqual(leftPodGroup.Spec, rightPodGroup.Spec) &&
		reflect.DeepEqual(leftPodGroup.OwnerReferences, rightPodGroup.OwnerReferences) &&
		reflect.DeepEqual(leftPodGroup.Labels, rightPodGroup.Labels) &&
		reflect.DeepEqual(leftPgUpdatableAnnotations, rightPgUpdatableAnnotations)
}

func updatePodGroup(oldPodGroup, newPodGroup *enginev2alpha2.PodGroup) {
	oldPodGroup.Annotations = mergeTwoMaps(oldPodGroup.Annotations, removeBlocklistedKeys(newPodGroup.Annotations))
	oldPodGroup.Labels = newPodGroup.Labels
	oldPodGroup.Spec = newPodGroup.Spec
	oldPodGroup.OwnerReferences = newPodGroup.OwnerReferences
}

func removeBlocklistedKeys(inputAnnotations map[string]string) map[string]string {
	annotationsToUpdate := map[string]string{}
	for annotationKey, annotationValue := range inputAnnotations {
		if !notToUpdateAnnotations[annotationKey] {
			annotationsToUpdate[annotationKey] = annotationValue
		}
	}
	return annotationsToUpdate
}

func mergeTwoMaps(org map[string]string, toMerge map[string]string) map[string]string {
	for annotationKey, annotationValue := range toMerge {
		org[annotationKey] = annotationValue
	}
	return org
}
