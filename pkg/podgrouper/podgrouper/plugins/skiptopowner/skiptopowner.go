// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package skiptopowner

import (
	"context"
	"fmt"
	"maps"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/NVIDIA/KAI-scheduler/pkg/podgrouper/podgroup"
	"github.com/NVIDIA/KAI-scheduler/pkg/podgrouper/podgrouper/plugins/constants"
	"github.com/NVIDIA/KAI-scheduler/pkg/podgrouper/podgrouper/plugins/defaultgrouper"
	"github.com/NVIDIA/KAI-scheduler/pkg/podgrouper/podgrouper/plugins/grouper"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

var schedulingMetadataToCopy = []string{
	constants.PriorityLabelKey,
	constants.PreemptibilityLabelKey,
	constants.TopologyKey,
	constants.TopologyRequiredPlacementKey,
	constants.TopologyPreferredPlacementKey,
}

var logger = log.FromContext(context.Background()).WithName("podgrouper").WithName("skiptopowner")

type skipTopOwnerGrouper struct {
	client        client.Client
	defaultPlugin *defaultgrouper.DefaultGrouper
	customPlugins map[metav1.GroupVersionKind]grouper.Grouper
}

func NewSkipTopOwnerGrouper(client client.Client, defaultGrouper *defaultgrouper.DefaultGrouper,
	customPlugins map[metav1.GroupVersionKind]grouper.Grouper) *skipTopOwnerGrouper {
	return &skipTopOwnerGrouper{
		client:        client,
		defaultPlugin: defaultGrouper,
		customPlugins: customPlugins,
	}
}

func (sk *skipTopOwnerGrouper) Name() string {
	return "SkipTopOwner Grouper"
}

func (sk *skipTopOwnerGrouper) GetPodGroupMetadata(
	skippedOwner *unstructured.Unstructured, pod *v1.Pod, otherOwners ...*metav1.PartialObjectMetadata,
) (*podgroup.Metadata, error) {
	var lastOwnerPartial *metav1.PartialObjectMetadata
	if len(otherOwners) <= 1 {
		lastOwnerPartial = &metav1.PartialObjectMetadata{
			TypeMeta:   pod.TypeMeta,
			ObjectMeta: pod.ObjectMeta,
		}
	} else {
		lastOwnerPartial = otherOwners[len(otherOwners)-2]
	}

	lastOwner, err := sk.getObjectInstance(lastOwnerPartial)
	if err != nil {
		return nil, fmt.Errorf("failed to get last owner: %w", err)
	}

	sk.propagateLabelsDownChain(skippedOwner, otherOwners...)

	if lastOwner.GetLabels() == nil {
		lastOwner.SetLabels(skippedOwner.GetLabels())
	} else {
		maps.Copy(lastOwner.GetLabels(), skippedOwner.GetLabels())
	}
	return sk.getSupportedTypePGMetadata(lastOwner, pod, otherOwners[:len(otherOwners)-1]...)
}

func (sk *skipTopOwnerGrouper) getSupportedTypePGMetadata(
	lastOwner *unstructured.Unstructured, pod *v1.Pod, otherOwners ...*metav1.PartialObjectMetadata,
) (*podgroup.Metadata, error) {
	ownerKind := metav1.GroupVersionKind(lastOwner.GroupVersionKind())
	if grouper, found := sk.customPlugins[ownerKind]; found {
		return grouper.GetPodGroupMetadata(lastOwner, pod, otherOwners...)
	}
	return sk.defaultPlugin.GetPodGroupMetadata(lastOwner, pod, otherOwners...)
}

func (sk *skipTopOwnerGrouper) getObjectInstance(objectRef *metav1.PartialObjectMetadata) (*unstructured.Unstructured, error) {
	topOwnerInstance := &unstructured.Unstructured{}
	topOwnerInstance.SetGroupVersionKind(objectRef.GroupVersionKind())
	key := types.NamespacedName{
		Namespace: objectRef.GetNamespace(),
		Name:      objectRef.GetName(),
	}
	err := sk.client.Get(context.Background(), key, topOwnerInstance)
	return topOwnerInstance, err
}

// propagateLabelsDownChain propagates scheduling metadata from parent owners to children
// this merges the metadata, copying over if there are no conflicts
// It persists the changes to the cluster by patching the actual resources
func (sk *skipTopOwnerGrouper) propagateLabelsDownChain(
	skippedOwner *unstructured.Unstructured, otherOwners ...*metav1.PartialObjectMetadata) {

	for _, owner := range otherOwners {
		logger.V(1).Info("Owner", "namespace", owner.GetNamespace(), "name", owner.GetName(), "kind", owner.GroupVersionKind().Kind)
	}
	labels := skippedOwner.GetLabels()
	if labels == nil {
		return
	}
	// Propagate to all owners except the last one (which is the actual owner we'll use for grouping)
	if len(otherOwners) <= 1 {
		return
	}
	for _, ownerPartial := range otherOwners[:len(otherOwners)-1] {
		// Fetch the full resource object to update it
		owner, err := sk.getObjectInstance(ownerPartial)
		if err != nil {
			logger.V(1).Error(err, "Failed to get owner instance for label propagation", "owner", ownerPartial.GetName())
			continue
		}

		ownerLabels := owner.GetLabels()
		ownerAnnotations := owner.GetAnnotations()
		needsUpdate := false
		var newLabels map[string]string
		var newAnnotations map[string]string

		for _, metadata := range schedulingMetadataToCopy {
			// Skip if owner already has this metadata
			if ownerLabels != nil && ownerAnnotations != nil {
				if _, exists := ownerLabels[metadata]; exists {
					logger.V(1).Info("Skipping propagation of metadata", "metadata", metadata, "owner", owner.GroupVersionKind().Kind)
					continue
				}
				if _, exists := ownerAnnotations[metadata]; exists {
					logger.V(1).Info("Skipping propagation of metadata", "metadata", metadata, "owner", owner.GroupVersionKind().Kind)
					continue
				}
			}
			// Propagate this metadata if it exists in skippedOwner
			if val, found := labels[metadata]; found && val != "" {
				logger.V(1).Info("Propagating metadata", "val", val, "owner", owner.GroupVersionKind().Kind)
				if newLabels == nil {
					if ownerLabels != nil {
						newLabels = maps.Clone(ownerLabels)
					} else {
						newLabels = make(map[string]string)
					}
				}
				newLabels[metadata] = val
				needsUpdate = true
			}
			if val, found := ownerAnnotations[metadata]; found && val != "" {
				logger.V(1).Info("Propagating annotation", "val", val, "owner", owner.GroupVersionKind().Kind)
				if newAnnotations == nil {
					if ownerAnnotations != nil {
						newAnnotations = maps.Clone(ownerAnnotations)
					} else {
						newAnnotations = make(map[string]string)
					}
				}
				newAnnotations[metadata] = val
				needsUpdate = true
			}
		}

		// Patch the resource if we need to update labels
		if needsUpdate {
			originalOwner := owner.DeepCopy()
			owner.SetLabels(newLabels)
			owner.SetAnnotations(newAnnotations)
			err = sk.client.Patch(context.Background(), owner, client.MergeFrom(originalOwner))
			if err != nil {
				logger.V(1).Error(err, "Failed to patch owner with propagated labels", "owner", owner.GetName())
			} else {
				logger.V(1).Info("Successfully propagated labels to owner", "owner", owner.GetName())
			}
		}
	}
}
