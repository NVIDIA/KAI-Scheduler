// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package ray

import (
	"context"
	"fmt"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/NVIDIA/KAI-scheduler/pkg/podgrouper/podgroup"
	"github.com/NVIDIA/KAI-scheduler/pkg/podgrouper/podgrouper/plugins/defaultgrouper"
)

const (
	rayClusterKind = "RayCluster"

	rayPriorityClassName = "ray.io/priority-class-name"

	headSubGroupName = "head"
)

type RayGrouper struct {
	client client.Client
	*defaultgrouper.DefaultGrouper
}

func NewRayGrouper(client client.Client, defaultGrouper *defaultgrouper.DefaultGrouper) *RayGrouper {
	return &RayGrouper{
		client:         client,
		DefaultGrouper: defaultGrouper,
	}
}

// +kubebuilder:rbac:groups=ray.io,resources=rayclusters;rayjobs;rayservices,verbs=get;list;watch
// +kubebuilder:rbac:groups=ray.io,resources=rayclusters/finalizers;rayjobs/finalizers;rayservices/finalizers,verbs=patch;update;create

func (rg *RayGrouper) getPodGroupMetadataWithClusterNamePath(
	topOwner *unstructured.Unstructured, pod *v1.Pod, clusterNamePaths [][]string,
) (
	*podgroup.Metadata, error,
) {
	var rayClusterObj *unstructured.Unstructured
	var err error
	rayClusterObj, err = rg.extractRayClusterObject(topOwner, clusterNamePaths)
	if err != nil {
		return nil, err
	}

	podGroupMetadata, err := rg.getPodGroupMetadataInternal(topOwner, rayClusterObj, pod)
	if err != nil {
		return nil, err
	}

	// Ray has it's own way to specify the workload priority class
	if rayPriorityClassName, labelFound := topOwner.GetLabels()[rayPriorityClassName]; labelFound {
		podGroupMetadata.PriorityClassName = rayPriorityClassName
	}

	return podGroupMetadata, nil
}

func (rg *RayGrouper) getPodGroupMetadataInternal(
	topOwner *unstructured.Unstructured, rayClusterObj *unstructured.Unstructured, pod *v1.Pod) (
	*podgroup.Metadata, error,
) {
	podGroupMetadata, err := rg.DefaultGrouper.GetPodGroupMetadata(topOwner, pod)
	if err != nil {
		return nil, err
	}

	minReplicas, subGroups, err := calcJobNumOfPodsAndSubGroups(rayClusterObj)
	if err != nil {
		return nil, err
	}
	podGroupMetadata.MinAvailable = minReplicas
	podGroupMetadata.SubGroups = subGroups

	return podGroupMetadata, nil
}

func (rg *RayGrouper) extractRayClusterObject(
	topOwner *unstructured.Unstructured, rayClusterNamePaths [][]string) (
	rayClusterObj *unstructured.Unstructured, err error,
) {
	if len(rayClusterNamePaths) == 0 {
		// If no rayClusterNamePaths are provided, use the topOwner as the rayClusterObj
		return topOwner, nil
	}

	var found bool = false
	var rayClusterName string
	for pathIndex := 0; (pathIndex < len(rayClusterNamePaths)) && !found; pathIndex++ {
		rayClusterName, found, err = unstructured.NestedString(topOwner.Object, rayClusterNamePaths[pathIndex]...)
		if err != nil {
			return nil, err
		}
	}
	if !found || rayClusterName == "" {
		return nil, fmt.Errorf("failed to extract rayClusterName for %s/%s of kind %s."+
			" Please make sure that the ray-operator is up and a rayCluster child-object has been created",
			topOwner.GetNamespace(), topOwner.GetName(), topOwner.GetKind())
	}

	rayClusterObj = &unstructured.Unstructured{}
	rayClusterObj.SetAPIVersion(topOwner.GetAPIVersion())
	rayClusterObj.SetKind(rayClusterKind)
	key := types.NamespacedName{
		Namespace: topOwner.GetNamespace(),
		Name:      rayClusterName,
	}
	err = rg.client.Get(context.Background(), key, rayClusterObj)
	if err != nil {
		return nil, err
	}

	return rayClusterObj, nil
}

// These calculations are based on the way KubeRay creates volcano pod-groups
// https://github.com/ray-project/kuberay/blob/dbcc686eabefecc3b939cd5c6e7a051f2473ad34/ray-operator/controllers/ray/batchscheduler/volcano/volcano_scheduler.go#L106
// https://github.com/ray-project/kuberay/blob/dbcc686eabefecc3b939cd5c6e7a051f2473ad34/ray-operator/controllers/ray/batchscheduler/volcano/volcano_scheduler.go#L51
func calcJobNumOfPodsAndSubGroups(topOwner *unstructured.Unstructured) (int32, []*podgroup.SubGroupMetadata, error) {
	headMinReplicas := int32(calcMinHeadReplicas(topOwner))
	minReplicas := headMinReplicas

	subGroups := []*podgroup.SubGroupMetadata{
		{
			Name:         headSubGroupName,
			MinAvailable: headMinReplicas,
		},
	}

	workerGroupSpecs, workerSpecFound, err := unstructured.NestedSlice(topOwner.Object,
		"spec", "workerGroupSpecs")
	if err != nil {
		return 0, nil, err
	}
	if !workerSpecFound || len(workerGroupSpecs) == 0 {
		return minReplicas, subGroups, nil
	}

	for groupIndex, groupSpec := range workerGroupSpecs {
		groupMinReplicas, groupDesiredReplicas, err := getReplicaCountersForWorkerGroup(groupSpec, groupIndex)
		if err != nil {
			return 0, nil, err
		}
		if groupMinReplicas == 0 && groupDesiredReplicas == 0 {
			continue // This type of worker doesn't contribute for minReplicas
		}

		numOfHosts, err := getGroupNumOfHosts(groupSpec)
		if err != nil {
			return 0, nil, err
		}

		var workerGroupMinReplicas int32
		if groupMinReplicas > 0 {
			// if minReplicas is set, use it to calculate the min number for workload scheduling
			workerGroupMinReplicas = int32(groupMinReplicas * numOfHosts)
		} else {
			// if minReplicas is not set, use the desiredReplicas field to calculate the min number for workload scheduling
			workerGroupMinReplicas = int32(groupDesiredReplicas * numOfHosts)
		}
		minReplicas += workerGroupMinReplicas

		// Get worker group name or generate one
		workerGroupName := getWorkerGroupName(groupSpec, groupIndex)
		subGroups = append(subGroups, &podgroup.SubGroupMetadata{
			Name:         workerGroupName,
			MinAvailable: workerGroupMinReplicas,
		})
	}

	return minReplicas, subGroups, nil
}

func getWorkerGroupName(groupSpec interface{}, groupIndex int) string {
	groupName, found, err := unstructured.NestedString(groupSpec.(map[string]interface{}), "groupName")
	if err != nil || !found || groupName == "" {
		return fmt.Sprintf("worker-group-%d", groupIndex)
	}
	return groupName
}

func getGroupNumOfHosts(groupSpec interface{}) (int64, error) {
	numOfHosts, found, err := unstructured.NestedInt64(groupSpec.(map[string]interface{}), "numOfHosts")
	if err != nil {
		return 0, err
	}
	if !found {
		numOfHosts = 1
	}
	return numOfHosts, nil
}

func getReplicaCountersForWorkerGroup(groupSpec interface{}, groupIndex int) (
	minReplicas int64, desiredReplicas int64, err error) {
	var found bool
	workerGroupSpec := groupSpec.(map[string]interface{})

	suspendedWorkerGroup, found, err := unstructured.NestedBool(workerGroupSpec, "suspended")
	if err != nil {
		return 0, 0, err
	}
	if found && suspendedWorkerGroup {
		return 0, 0, nil
	}

	desiredReplicas, found, err = unstructured.NestedInt64(workerGroupSpec, "replicas")
	if err != nil {
		return 0, 0, err
	}
	if !found {
		desiredReplicas = 0
	}

	minReplicas, found, err = unstructured.NestedInt64(workerGroupSpec, "minReplicas")
	if err != nil {
		return 0, 0, err
	}
	if !found {
		minReplicas = 0
	}
	if minReplicas > desiredReplicas {
		return 0, 0, fmt.Errorf(
			"ray-cluster replicas for a workerGroupsSpec can't be less than minReplicas. "+
				"Given %v replicas and %v minReplicas. Please fix the replicas field for group number %v",
			desiredReplicas, minReplicas, groupIndex)
	}
	return minReplicas, desiredReplicas, nil
}

func calcMinHeadReplicas(topOwner *unstructured.Unstructured) int64 {
	minHeadReplicas := int64(1) // default value 1

	launcherReplicas, replicasFound, replicasErr := unstructured.NestedInt64(topOwner.Object,
		"spec", "headGroupSpec", "replicas")
	if replicasErr == nil && replicasFound && launcherReplicas > 0 {
		minHeadReplicas = launcherReplicas
	}

	launcherMinReplicas, minReplicasFound, err := unstructured.NestedInt64(topOwner.Object,
		"spec", "headGroupSpec", "minReplicas")
	if err == nil && minReplicasFound && launcherMinReplicas > 0 {
		minHeadReplicas = launcherMinReplicas
	}

	return minHeadReplicas
}
