// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package dynamo

import (
	"context"
	"fmt"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/NVIDIA/KAI-scheduler/pkg/podgrouper/podgroup"
	"github.com/NVIDIA/KAI-scheduler/pkg/podgrouper/podgrouper/plugins/constants"
	"github.com/NVIDIA/KAI-scheduler/pkg/podgrouper/podgrouper/plugins/defaultgrouper"
	leader_worker_set "github.com/NVIDIA/KAI-scheduler/pkg/podgrouper/podgrouper/plugins/leaderworkerset"
)

const (
	labelKeyPodGangName    = "grove.io/podgang"
	crdTypePodGang         = "PodGang"
	crdTypeLeaderWorkerSet = "LeaderWorkerSet"
	crdTypePodClique       = "PodClique"
	crdTypePodCliqueSet    = "PodCliqueSet"
)

type DynamoGrouper struct {
	client client.Client
	*defaultgrouper.DefaultGrouper
}

func NewDynamoGrouper(client client.Client, defaultGrouper *defaultgrouper.DefaultGrouper) *DynamoGrouper {
	return &DynamoGrouper{
		client:         client,
		DefaultGrouper: defaultGrouper,
	}
}

func (gg *DynamoGrouper) Name() string {
	return "Dynamo Grouper"
}

// This plugin will act as a wrapper around the Grove's CRDs and will parse and pass metadate from DynamoGrpahDeployment to Grove's native PodGang
// alternatively, if LWS is used, this plugin will act as a wrapper around the LWS and will parse and pass metadata from DynamoGrpahDeployment to LWS

// +kubebuilder:rbac:groups=grove.io,resources=podgangsets,verbs=get;list;watch
// +kubebuilder:rbac:groups=grove.io,resources=podgangsets/finalizers,verbs=patch;update;create
// +kubebuilder:rbac:groups=grove.io,resources=podcliquesets,verbs=get;list;watch
// +kubebuilder:rbac:groups=grove.io,resources=podcliquesets/finalizers,verbs=patch;update;create
// +kubebuilder:rbac:groups=grove.io,resources=podcliques,verbs=get;list;watch
// +kubebuilder:rbac:groups=grove.io,resources=podcliques/finalizers,verbs=patch;update;create
// +kubebuilder:rbac:groups=grove.io,resources=podcliquescalinggroups,verbs=get;list;watch
// +kubebuilder:rbac:groups=grove.io,resources=podcliquescalinggroups/finalizers,verbs=patch;update;create
// +kubebuilder:rbac:groups=nvidia.com,resources=dynamographdeployments,verbs=get;list;watch
// +kubebuilder:rbac:groups=nvidia.com,resources=dynamographdeployments/finalizers,verbs=patch;update;create
// +kubebuilder:rbac:groups=scheduler.grove.io,resources=podgangs,verbs=get;list;watch
// +kubebuilder:rbac:groups=scheduler.grove.io,resources=podgangs/finalizers,verbs=patch;update;create
// +kubebuilder:rbac:groups=leaderworkerset.x-k8s.io,resources=leaderworkersets,verbs=get;list;watch
// +kubebuilder:rbac:groups=leaderworkerset.x-k8s.io,resources=leaderworkersets/finalizers,verbs=patch;update;create

// Metadata in Dynamo is stored in the DynamoGraphDeployment object.
func (dg *DynamoGrouper) GetPodGroupMetadata(
	dynamoGraphDeployment *unstructured.Unstructured, pod *v1.Pod, otherOwners ...*metav1.PartialObjectMetadata,
) (*podgroup.Metadata, error) {
	// Find PodGang or LeaderWorkerSet that has DynamoGraphDeployment as its owner
	// The chain is: Pod -> ... -> PodGang/LWS -> DynamoGraphDeployment (top owner)
	ownedCRD, crdType, err := dg.findOwnedCRD(dynamoGraphDeployment, otherOwners)
	if err != nil {
		return nil, fmt.Errorf("failed to find owned CRD for DynamoGraphDeployment %s/%s: %w",
			dynamoGraphDeployment.GetNamespace(), dynamoGraphDeployment.GetName(), err)
	}

	// Transfer labels and annotations from DynamoGraphDeployment to PodGang/LWS
	// This allows the groupers to read metadata from the CRD object
	modifiedCRD := dg.transferMetadataToCRD(ownedCRD, dynamoGraphDeployment)

	// Update the CRD in the client so groupers that fetch from client will get the updated metadata
	// Note: This is a best-effort update - if it fails, we continue with the modified CRD
	_ = dg.client.Update(context.Background(), modifiedCRD)

	// Call the appropriate grouper based on CRD type
	// Note: For PodCliqueSet, we use DefaultGrouper since GroveGrouper expects PodGang
	// The metadata has already been transferred to PodCliqueSet, so DefaultGrouper will read it from there
	var metadata *podgroup.Metadata
	switch crdType {
	case crdTypePodCliqueSet:
		// PodCliqueSet is the Grove intermediate resource
		// Use DefaultGrouper which will read metadata from the PodCliqueSet labels/annotations we just transferred
		metadata, err = dg.DefaultGrouper.GetPodGroupMetadata(modifiedCRD, pod, otherOwners...)
		if err != nil {
			return nil, fmt.Errorf("failed to get DefaultGrouper metadata for PodCliqueSet %s/%s: %w",
				ownedCRD.GetNamespace(), ownedCRD.GetName(), err)
		}
	case crdTypeLeaderWorkerSet:
		lwsGrouper := leader_worker_set.NewLwsGrouper(dg.DefaultGrouper)
		metadata, err = lwsGrouper.GetPodGroupMetadata(modifiedCRD, pod, otherOwners...)
		if err != nil {
			return nil, fmt.Errorf("failed to get LwsGrouper metadata for LeaderWorkerSet %s/%s: %w",
				ownedCRD.GetNamespace(), ownedCRD.GetName(), err)
		}
	default:
		return nil, fmt.Errorf("unknown CRD type: %s", crdType)
	}

	// Preserve existing graphGroups logic for SubGroups
	var minAvailable int32
	pgSlice, found, err := unstructured.NestedSlice(dynamoGraphDeployment.Object, "spec", "graphGroups")
	if err != nil {
		return nil, fmt.Errorf("failed to get spec.graphGroups from DynamoGraphDeployment %s/%s. Err: %w",
			dynamoGraphDeployment.GetNamespace(), dynamoGraphDeployment.GetName(), err)
	}
	if found {
		// Clear existing subgroups if we're parsing graphGroups from DynamoGraphDeployment
		metadata.SubGroups = []*podgroup.SubGroupMetadata{}
		for pgIndex, v := range pgSlice {
			pgr, ok := v.(map[string]interface{})
			if !ok {
				return nil, fmt.Errorf("invalid structure of spec.graphGroups[%v] in DynamoGraphDeployment %s/%s",
					pgIndex, dynamoGraphDeployment.GetNamespace(), dynamoGraphDeployment.GetName())
			}
			subGroup, err := parseGroveSubGroup(pgr, pgIndex, pod.Namespace, dynamoGraphDeployment.GetName())
			if err != nil {
				return nil, fmt.Errorf("failed to parse spec.graphGroups[%d] from DynamoGraphDeployment %s/%s. Err: %w",
					pgIndex, dynamoGraphDeployment.GetNamespace(), dynamoGraphDeployment.GetName(), err)
			}
			metadata.SubGroups = append(metadata.SubGroups, subGroup)
			minAvailable += subGroup.MinAvailable
		}
		metadata.MinAvailable = minAvailable
	}

	return metadata, nil
}

func parseGroveSubGroup(
	pg map[string]interface{}, pgIndex int, namespace, podGangName string,
) (*podgroup.SubGroupMetadata, error) {
	// Name
	name, found, err := unstructured.NestedString(pg, "name")
	if err != nil {
		return nil, fmt.Errorf("failed to parse 'name' field. Err: %v", err)
	}
	if !found {
		return nil, fmt.Errorf("missing required 'name' field")
	}

	// MinReplicas
	minAvailable, found, err := unstructured.NestedInt64(pg, "minReplicas")
	if err != nil {
		return nil, fmt.Errorf("failed to parse 'minReplicas' field. Err: %v", err)
	}
	if !found {
		return nil, fmt.Errorf("missing required 'minReplicas' field")
	}
	if minAvailable <= 0 {
		return nil, fmt.Errorf("invalid 'minReplicas' field. Must be greater than 0")
	}

	// PodReferences
	podReferences, found, err := unstructured.NestedSlice(pg, "podReferences")
	if err != nil {
		return nil, fmt.Errorf("failed to parse 'podReferences' field. Err: %w", err)
	}
	if !found {
		return nil, fmt.Errorf("missing required 'podReferences' field")
	}
	var pods []*types.NamespacedName
	for podIndex, podRef := range podReferences {
		reference, ok := podRef.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("invalid spec.podgroup[%d].podReferences[%d] in PodGang %s/%s",
				pgIndex, podIndex, namespace, podGangName)
		}
		namespacedName, err := parsePodReference(reference)
		if err != nil {
			return nil, fmt.Errorf("failed to parse spec.podgroups[%d].podreferences[%d] from PodGang %s/%s. Err: %w",
				pgIndex, podIndex, namespace, podGangName, err)
		}
		pods = append(pods, namespacedName)
	}

	return &podgroup.SubGroupMetadata{
		Name:           name,
		MinAvailable:   int32(minAvailable),
		PodsReferences: pods,
	}, nil
}

// findOwnedCRD finds PodCliqueSet (for Grove) or LeaderWorkerSet that has DynamoGraphDeployment as its owner
// The ownership tree is: DynamoGraphDeployment -> PodCliqueSet -> PodGang
// First tries to find it in the owner chain, then searches for resources owned by DynamoGraphDeployment
func (dg *DynamoGrouper) findOwnedCRD(dynamoGraphDeployment *unstructured.Unstructured, otherOwners []*metav1.PartialObjectMetadata) (*unstructured.Unstructured, string, error) {
	var podCliqueSetOwner *metav1.PartialObjectMetadata
	var lwsOwner *metav1.PartialObjectMetadata

	// First, try to find PodCliqueSet or LeaderWorkerSet in the owner chain
	if len(otherOwners) > 0 {
		for i := range otherOwners {
			owner := otherOwners[i]
			// Check for PodCliqueSet (grove.io/v1alpha1) - Grove intermediate resource
			if owner.Kind == crdTypePodCliqueSet && owner.APIVersion == "grove.io/v1alpha1" {
				if podCliqueSetOwner != nil {
					return nil, "", fmt.Errorf("found multiple PodCliqueSet owners in owner chain for DynamoGraphDeployment %s/%s",
						dynamoGraphDeployment.GetNamespace(), dynamoGraphDeployment.GetName())
				}
				podCliqueSetOwner = owner
			}
			// Check for LeaderWorkerSet (leaderworkerset.x-k8s.io/v1)
			if owner.Kind == crdTypeLeaderWorkerSet && owner.APIVersion == "leaderworkerset.x-k8s.io/v1" {
				if lwsOwner != nil {
					return nil, "", fmt.Errorf("found multiple LeaderWorkerSet owners in owner chain for DynamoGraphDeployment %s/%s",
						dynamoGraphDeployment.GetNamespace(), dynamoGraphDeployment.GetName())
				}
				lwsOwner = owner
			}
		}
	}

	// If found in owner chain, fetch and verify
	if podCliqueSetOwner != nil {
		if lwsOwner != nil {
			return nil, "", fmt.Errorf("found both PodCliqueSet and LeaderWorkerSet in owner chain for DynamoGraphDeployment %s/%s",
				dynamoGraphDeployment.GetNamespace(), dynamoGraphDeployment.GetName())
		}
		return dg.fetchAndVerifyPodCliqueSet(podCliqueSetOwner, dynamoGraphDeployment)
	}

	if lwsOwner != nil {
		return dg.fetchAndVerifyLeaderWorkerSet(lwsOwner, dynamoGraphDeployment)
	}

	// If not found in owner chain, search for PodCliqueSet/LWS that have DynamoGraphDeployment as owner
	return dg.searchForOwnedCRD(dynamoGraphDeployment)
}

// fetchAndVerifyPodCliqueSet fetches PodCliqueSet and verifies it has DynamoGraphDeployment as its owner
func (dg *DynamoGrouper) fetchAndVerifyPodCliqueSet(podCliqueSetOwner *metav1.PartialObjectMetadata, dynamoGraphDeployment *unstructured.Unstructured) (*unstructured.Unstructured, string, error) {
	podCliqueSet := &unstructured.Unstructured{}
	podCliqueSet.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "grove.io",
		Kind:    crdTypePodCliqueSet,
		Version: "v1alpha1",
	})
	err := dg.client.Get(context.Background(), client.ObjectKey{
		Namespace: podCliqueSetOwner.GetNamespace(),
		Name:      podCliqueSetOwner.GetName(),
	}, podCliqueSet)
	if err != nil {
		return nil, "", fmt.Errorf("failed to get PodCliqueSet %s/%s: %w",
			podCliqueSetOwner.GetNamespace(), podCliqueSetOwner.GetName(), err)
	}
	// Verify PodCliqueSet has DynamoGraphDeployment as its owner
	if !dg.hasOwnerReference(podCliqueSet, dynamoGraphDeployment) {
		return nil, "", fmt.Errorf("PodCliqueSet %s/%s does not have DynamoGraphDeployment %s/%s as its owner",
			podCliqueSetOwner.GetNamespace(), podCliqueSetOwner.GetName(),
			dynamoGraphDeployment.GetNamespace(), dynamoGraphDeployment.GetName())
	}
	return podCliqueSet, crdTypePodCliqueSet, nil
}

// fetchAndVerifyLeaderWorkerSet fetches LeaderWorkerSet and verifies it has DynamoGraphDeployment as its owner
func (dg *DynamoGrouper) fetchAndVerifyLeaderWorkerSet(lwsOwner *metav1.PartialObjectMetadata, dynamoGraphDeployment *unstructured.Unstructured) (*unstructured.Unstructured, string, error) {
	lws := &unstructured.Unstructured{}
	lws.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "leaderworkerset.x-k8s.io",
		Kind:    crdTypeLeaderWorkerSet,
		Version: "v1",
	})
	err := dg.client.Get(context.Background(), client.ObjectKey{
		Namespace: lwsOwner.GetNamespace(),
		Name:      lwsOwner.GetName(),
	}, lws)
	if err != nil {
		return nil, "", fmt.Errorf("failed to get LeaderWorkerSet %s/%s: %w",
			lwsOwner.GetNamespace(), lwsOwner.GetName(), err)
	}
	// Verify LeaderWorkerSet has DynamoGraphDeployment as its owner
	if !dg.hasOwnerReference(lws, dynamoGraphDeployment) {
		return nil, "", fmt.Errorf("LeaderWorkerSet %s/%s does not have DynamoGraphDeployment %s/%s as its owner",
			lwsOwner.GetNamespace(), lwsOwner.GetName(),
			dynamoGraphDeployment.GetNamespace(), dynamoGraphDeployment.GetName())
	}
	return lws, crdTypeLeaderWorkerSet, nil
}

// searchForOwnedCRD searches for PodCliqueSet or LeaderWorkerSet resources that have DynamoGraphDeployment as their owner
func (dg *DynamoGrouper) searchForOwnedCRD(dynamoGraphDeployment *unstructured.Unstructured) (*unstructured.Unstructured, string, error) {
	namespace := dynamoGraphDeployment.GetNamespace()
	dynamoName := dynamoGraphDeployment.GetName()

	var foundPodCliqueSet *unstructured.Unstructured
	var foundLWS *unstructured.Unstructured

	// Search for PodCliqueSet resources in the namespace
	podCliqueSetList := &unstructured.UnstructuredList{}
	podCliqueSetList.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "grove.io",
		Kind:    "PodCliqueSetList",
		Version: "v1alpha1",
	})
	err := dg.client.List(context.Background(), podCliqueSetList, client.InNamespace(namespace))
	if err == nil {
		for i := range podCliqueSetList.Items {
			podCliqueSet := &podCliqueSetList.Items[i]
			if dg.hasOwnerReference(podCliqueSet, dynamoGraphDeployment) {
				if foundPodCliqueSet != nil {
					return nil, "", fmt.Errorf("found multiple PodCliqueSet resources owned by DynamoGraphDeployment %s/%s",
						namespace, dynamoName)
				}
				foundPodCliqueSet = podCliqueSet
			}
		}
	}

	// Search for LeaderWorkerSet resources in the namespace
	lwsList := &unstructured.UnstructuredList{}
	lwsList.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "leaderworkerset.x-k8s.io",
		Kind:    "LeaderWorkerSetList",
		Version: "v1",
	})
	err = dg.client.List(context.Background(), lwsList, client.InNamespace(namespace))
	if err == nil {
		for i := range lwsList.Items {
			lws := &lwsList.Items[i]
			if dg.hasOwnerReference(lws, dynamoGraphDeployment) {
				if foundLWS != nil {
					return nil, "", fmt.Errorf("found multiple LeaderWorkerSet resources owned by DynamoGraphDeployment %s/%s",
						namespace, dynamoName)
				}
				foundLWS = lws
			}
		}
	}

	// Check if both are found
	if foundPodCliqueSet != nil && foundLWS != nil {
		return nil, "", fmt.Errorf("found both PodCliqueSet %s/%s and LeaderWorkerSet %s/%s owned by DynamoGraphDeployment %s/%s",
			foundPodCliqueSet.GetNamespace(), foundPodCliqueSet.GetName(),
			foundLWS.GetNamespace(), foundLWS.GetName(),
			namespace, dynamoName)
	}

	if foundPodCliqueSet != nil {
		return foundPodCliqueSet, crdTypePodCliqueSet, nil
	}

	if foundLWS != nil {
		return foundLWS, crdTypeLeaderWorkerSet, nil
	}

	return nil, "", fmt.Errorf("no PodCliqueSet or LeaderWorkerSet found with DynamoGraphDeployment %s/%s as owner",
		namespace, dynamoName)
}

// hasOwnerReference checks if the resource has the specified owner as one of its owner references
func (dg *DynamoGrouper) hasOwnerReference(resource *unstructured.Unstructured, owner *unstructured.Unstructured) bool {
	ownerRefs := resource.GetOwnerReferences()
	for i := range ownerRefs {
		ref := &ownerRefs[i]
		if ref.Kind == owner.GetKind() &&
			ref.APIVersion == owner.GetAPIVersion() &&
			ref.Name == owner.GetName() &&
			ref.UID == owner.GetUID() {
			return true
		}
	}
	return false
}

// transferMetadataToCRD transfers labels, annotations, and spec fields from DynamoGraphDeployment to the CRD (PodCliqueSet/LWS)
// This creates a deep copy of the CRD and adds the metadata so the grouper can read it from the CRD object
func (dg *DynamoGrouper) transferMetadataToCRD(crd *unstructured.Unstructured, dynamoGraphDeployment *unstructured.Unstructured) *unstructured.Unstructured {
	// Create a deep copy of the CRD
	modifiedCRD := crd.DeepCopy()

	// Get labels and annotations from DynamoGraphDeployment
	dynamoLabels := dynamoGraphDeployment.GetLabels()
	dynamoAnnotations := dynamoGraphDeployment.GetAnnotations()

	// Get existing labels and annotations from CRD
	crdLabels := modifiedCRD.GetLabels()
	if crdLabels == nil {
		crdLabels = make(map[string]string)
	}
	crdAnnotations := modifiedCRD.GetAnnotations()
	if crdAnnotations == nil {
		crdAnnotations = make(map[string]string)
	}

	// Transfer priorityClassName label (for DefaultGrouper)
	if priorityClassName, found := dynamoLabels[constants.PriorityLabelKey]; found {
		crdLabels[constants.PriorityLabelKey] = priorityClassName
		// Also set spec.priorityClassName for GroveGrouper compatibility
		if err := unstructured.SetNestedField(modifiedCRD.Object, priorityClassName, "spec", "priorityClassName"); err != nil {
			// If spec doesn't exist, create it
			spec, _, _ := unstructured.NestedMap(modifiedCRD.Object, "spec")
			if spec == nil {
				spec = make(map[string]interface{})
			}
			spec["priorityClassName"] = priorityClassName
			unstructured.SetNestedMap(modifiedCRD.Object, spec, "spec")
		}
	}

	// Transfer preemptibility label
	if preemptibility, found := dynamoLabels[constants.PreemptibilityLabelKey]; found {
		crdLabels[constants.PreemptibilityLabelKey] = preemptibility
	}

	// Transfer topology annotations
	if topology, found := dynamoAnnotations["kai.scheduler/topology"]; found {
		crdAnnotations["kai.scheduler/topology"] = topology
		// Also set grove.io/topology-name annotation for Grove compatibility
		crdAnnotations["grove.io/topology-name"] = topology
	}
	if preferredLevel, found := dynamoAnnotations["kai.scheduler/topology-preferred-placement"]; found {
		crdAnnotations["kai.scheduler/topology-preferred-placement"] = preferredLevel
	}
	if requiredLevel, found := dynamoAnnotations["kai.scheduler/topology-required-placement"]; found {
		crdAnnotations["kai.scheduler/topology-required-placement"] = requiredLevel
	}

	// Set the modified labels and annotations
	modifiedCRD.SetLabels(crdLabels)
	modifiedCRD.SetAnnotations(crdAnnotations)

	return modifiedCRD
}

func parsePodReference(podRef map[string]interface{}) (*types.NamespacedName, error) {
	podNamespace, found, err := unstructured.NestedString(podRef, "namespace")
	if err != nil {
		return nil, fmt.Errorf("failed to parse 'namespace' field. Err: %v", err)
	}
	if !found {
		return nil, fmt.Errorf("missing required 'namespace' field")
	}

	podName, found, err := unstructured.NestedString(podRef, "name")
	if err != nil {
		return nil, fmt.Errorf("failed to parse 'name' field. Err: %v", err)
	}
	if !found {
		return nil, fmt.Errorf("missing required 'name' field")
	}

	return &types.NamespacedName{Namespace: podNamespace, Name: podName}, nil
}
