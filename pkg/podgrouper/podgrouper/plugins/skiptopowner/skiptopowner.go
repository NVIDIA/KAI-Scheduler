// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package skiptopowner

import (
	"context"
	"fmt"
	"maps"

	"sigs.k8s.io/controller-runtime/pkg/log"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/NVIDIA/KAI-scheduler/pkg/podgrouper/podgroup"
	"github.com/NVIDIA/KAI-scheduler/pkg/podgrouper/podgrouper/plugins/defaultgrouper"
	"github.com/NVIDIA/KAI-scheduler/pkg/podgrouper/podgrouper/plugins/grouper"
)

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

	err = sk.propagateLabelsDownChain(skippedOwner, otherOwners...)
	if err != nil {
		return nil, fmt.Errorf("failed to propagate labels down chain: %w", err)
	}

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
// The changes are made in memory only and are not persisted to the cluster
func (sk *skipTopOwnerGrouper) propagateLabelsDownChain(
	skippedOwner *unstructured.Unstructured, otherOwners ...*metav1.PartialObjectMetadata) error {

	for _, owner := range otherOwners {
		logger.V(1).Info("Owner", "namespace", owner.GetNamespace(), "name", owner.GetName(), "kind", owner.GroupVersionKind().Kind)
	}
	labels := skippedOwner.GetLabels()
	annotations := skippedOwner.GetAnnotations()
	if len(otherOwners) <= 1 {
		return nil
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

		for key, val := range labels {
			// Skip if owner already has this label
			if ownerLabels != nil {
				if _, exists := ownerLabels[key]; exists {
					continue
				}
			}
			// Propagate this label if it has a value
			if val != "" {
				if newLabels == nil {
					if ownerLabels != nil {
						newLabels = maps.Clone(ownerLabels)
					} else {
						newLabels = make(map[string]string)
					}
				}
				newLabels[key] = val
				needsUpdate = true
			}
		}

		for key, val := range annotations {
			// Skip if owner already has this annotation
			if ownerAnnotations != nil {
				if _, exists := ownerAnnotations[key]; exists {
					continue
				}
			}
			// Propagate this annotation if it has a value
			if val != "" {
				if newAnnotations == nil {
					if ownerAnnotations != nil {
						newAnnotations = maps.Clone(ownerAnnotations)
					} else {
						newAnnotations = make(map[string]string)
					}
				}
				newAnnotations[key] = val
				needsUpdate = true
			}
		}

		// Patch the resource if we need to update labels
		if needsUpdate {
			owner.SetLabels(newLabels)
			owner.SetAnnotations(newAnnotations)
		}
	}
	return nil
}
