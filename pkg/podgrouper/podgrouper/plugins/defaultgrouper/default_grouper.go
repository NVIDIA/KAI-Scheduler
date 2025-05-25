// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package defaultgrouper

import (
	"context"
	"fmt"

	"golang.org/x/exp/maps"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/log"

	commonconsts "github.com/NVIDIA/KAI-scheduler/pkg/common/constants"

	"github.com/NVIDIA/KAI-scheduler/pkg/podgrouper/podgroup"
	"github.com/NVIDIA/KAI-scheduler/pkg/podgrouper/podgrouper/plugins/constants"
	"github.com/NVIDIA/KAI-scheduler/pkg/podgrouper/topowner"
)

var (
	logger = log.FromContext(context.Background())
)

type DefaultGrouper struct {
	queueLabelKey    string
	nodePoolLabelKey string
}

func NewDefaultGrouper(queueLabelKey, nodePoolLabelKey string) *DefaultGrouper {
	return &DefaultGrouper{
		queueLabelKey:    queueLabelKey,
		nodePoolLabelKey: nodePoolLabelKey,
	}
}

func (dg *DefaultGrouper) Name() string {
	return "Default Grouper"
}

func (dg *DefaultGrouper) GetPodGroupMetadata(topOwner *unstructured.Unstructured, pod *v1.Pod, _ ...*metav1.PartialObjectMetadata) (*podgroup.Metadata, error) {
	podGroupMetadata := podgroup.Metadata{
		Owner: metav1.OwnerReference{
			APIVersion: topOwner.GetAPIVersion(),
			Kind:       topOwner.GetKind(),
			Name:       topOwner.GetName(),
			UID:        topOwner.GetUID(),
		},
		Namespace:         pod.GetNamespace(),
		Name:              dg.CalcPodGroupName(topOwner),
		Annotations:       dg.CalcPodGroupAnnotations(topOwner, pod),
		Labels:            dg.CalcPodGroupLabels(topOwner, pod),
		Queue:             dg.CalcPodGroupQueue(topOwner, pod),
		PriorityClassName: dg.CalcPodGroupPriorityClass(topOwner, pod, constants.TrainPriorityClass),
		MinAvailable:      1,
	}

	return &podGroupMetadata, nil
}

func (dg *DefaultGrouper) CalcPodGroupName(topOwner *unstructured.Unstructured) string {
	return fmt.Sprintf("%s-%s-%s", constants.PodGroupNamePrefix, topOwner.GetName(), topOwner.GetUID())
}

func (dg *DefaultGrouper) CalcPodGroupAnnotations(topOwner *unstructured.Unstructured, pod *v1.Pod) map[string]string {
	// Inherit all the annotations of the top owner
	pgAnnotations := make(map[string]string, len(topOwner.GetAnnotations())+2)

	if value, exists := pod.GetAnnotations()[constants.UserLabelKey]; exists {
		pgAnnotations[constants.UserLabelKey] = value
	}
	pgAnnotations[constants.JobIdKey] = string(topOwner.GetUID())

	topOwnerMetadata := topowner.GetTopOwnerMetadata(topOwner)
	marshalledMetadata, err := topOwnerMetadata.MarshalYAML()
	if err != nil {
		logger.V(1).Error(err, "Unable to marshal top owner metadata", "metadata", topOwnerMetadata)
	} else {
		pgAnnotations[commonconsts.TopOwnerMetadataKey] = marshalledMetadata
	}

	maps.Copy(pgAnnotations, topOwner.GetAnnotations())

	return pgAnnotations
}

func (dg *DefaultGrouper) CalcPodGroupLabels(topOwner *unstructured.Unstructured, pod *v1.Pod) map[string]string {
	// Inherit all the labels of the top owner
	pgLabels := make(map[string]string, len(topOwner.GetLabels()))
	maps.Copy(pgLabels, topOwner.GetLabels())

	// Get podGroup user from the pod label
	if _, exists := pgLabels[constants.UserLabelKey]; !exists {
		if value, exists := pod.GetLabels()[constants.UserLabelKey]; exists {
			pgLabels[constants.UserLabelKey] = value
		}
	}

	return pgLabels
}

func (dg *DefaultGrouper) CalcPodGroupQueue(topOwner *unstructured.Unstructured, pod *v1.Pod) string {
	if queue, found := topOwner.GetLabels()[dg.queueLabelKey]; found {
		return queue
	} else if queue, found = pod.GetLabels()[dg.queueLabelKey]; found {
		return queue
	}

	queue := dg.calculateQueueName(topOwner, pod)
	if queue != "" {
		return queue
	}

	return constants.DefaultQueueName
}

func (dg *DefaultGrouper) calculateQueueName(topOwner *unstructured.Unstructured, pod *v1.Pod) string {
	project := ""
	if projectLabel, found := topOwner.GetLabels()[constants.ProjectLabelKey]; found {
		project = projectLabel
	} else if projectLabel, found := pod.GetLabels()[constants.ProjectLabelKey]; found {
		project = projectLabel
	}

	if project == "" {
		return ""
	}

	if nodePool, found := pod.GetLabels()[dg.nodePoolLabelKey]; found {
		return fmt.Sprintf("%s-%s", project, nodePool)
	}

	return project
}

func (dg *DefaultGrouper) CalcPodGroupPriorityClass(topOwner *unstructured.Unstructured, pod *v1.Pod,
	defaultPriorityClassForJob string) string {
	if priorityClassName, found := topOwner.GetLabels()[constants.PriorityLabelKey]; found {
		return priorityClassName
	} else if priorityClassName, found = pod.GetLabels()[constants.PriorityLabelKey]; found {
		return priorityClassName
	} else if len(pod.Spec.PriorityClassName) != 0 {
		return pod.Spec.PriorityClassName
	} else {
		return defaultPriorityClassForJob
	}
}
