package main

import (
	"context"
	"fmt"

	kaischedulerclientset "github.com/NVIDIA/KAI-scheduler/pkg/apis/client/clientset/versioned"
	kaiv1alpha1 "github.com/NVIDIA/KAI-scheduler/pkg/apis/kai/v1alpha1"
	schedulingv1alpha2 "github.com/NVIDIA/KAI-scheduler/pkg/apis/scheduling/v1alpha2"
	snapshotplugin "github.com/NVIDIA/KAI-scheduler/pkg/scheduler/plugins/snapshot"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

func LoadLiveSnapshot(ctx context.Context, kube kubernetes.Interface, kai kaischedulerclientset.Interface) (*snapshotplugin.Snapshot, error) {
	if kube == nil {
		return nil, fmt.Errorf("kube client is nil")
	}
	if kai == nil {
		return nil, fmt.Errorf("kai client is nil")
	}

	nodeList, err := kube.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("list nodes: %w", err)
	}
	nodes := make([]*corev1.Node, 0, len(nodeList.Items))
	for i := range nodeList.Items {
		nodes = append(nodes, &nodeList.Items[i])
	}

	topoList, err := kai.KaiV1alpha1().Topologies().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("list topologies: %w", err)
	}
	topologies := make([]*kaiv1alpha1.Topology, 0, len(topoList.Items))
	for i := range topoList.Items {
		topologies = append(topologies, &topoList.Items[i])
	}

	// BindRequests are namespaced; list them across all namespaces.
	nsList, err := kube.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("list namespaces: %w", err)
	}
	bindRequests := make([]*schedulingv1alpha2.BindRequest, 0)
	for i := range nsList.Items {
		ns := nsList.Items[i].Name
		brList, err := kai.SchedulingV1alpha2().BindRequests(ns).List(ctx, metav1.ListOptions{})
		if err != nil {
			// Best-effort: keep going if one namespace fails.
			continue
		}
		for j := range brList.Items {
			bindRequests = append(bindRequests, &brList.Items[j])
		}
	}

	raw := &snapshotplugin.RawKubernetesObjects{
		Pods:                   []*corev1.Pod{},
		Nodes:                  nodes,
		Queues:                 nil,
		PodGroups:              nil,
		BindRequests:           bindRequests,
		PriorityClasses:        nil,
		ConfigMaps:             nil,
		PersistentVolumeClaims: nil,
		CSIStorageCapacities:   nil,
		StorageClasses:         nil,
		CSIDrivers:             nil,
		ResourceClaims:         nil,
		ResourceSlices:         nil,
		DeviceClasses:          nil,
		Topologies:             topologies,
	}

	return &snapshotplugin.Snapshot{RawObjects: raw}, nil
}
