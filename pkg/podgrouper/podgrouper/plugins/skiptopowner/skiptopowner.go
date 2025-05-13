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
	"github.com/NVIDIA/KAI-scheduler/pkg/podgrouper/podgrouper/plugins/defaultgrouper"
	"github.com/NVIDIA/KAI-scheduler/pkg/podgrouper/podgrouper/plugins/grouper"
)

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
