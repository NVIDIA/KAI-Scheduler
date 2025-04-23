// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package node_info

import (
	"fmt"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	. "go.uber.org/mock/gomock"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	commonconstants "github.com/NVIDIA/KAI-scheduler/pkg/common/constants"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/common_info"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/pod_affinity"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/pod_info"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/pod_status"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/resource_info"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/storagecapacity_info"
)

const MibToMbScale = 1.048576

func nodeInfoEqual(l, r *NodeInfo) bool {
	l.PodAffinityInfo = nil
	r.PodAffinityInfo = nil
	return reflect.DeepEqual(l, r)
}

type AddRemovePodsTest struct {
	name     string
	node     *v1.Node
	pods     []*v1.Pod
	rmPods   []*v1.Pod
	expected *NodeInfo
}

type podCreationOptions struct {
	GPUs      float64
	releasing bool
	gpuGroup  string
}

func RunAddRemovePodsTests(t *testing.T, tests []AddRemovePodsTest) {
	for i, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			controller := NewController(t)
			nodePodAffinityInfo := pod_affinity.NewMockNodePodAffinityInfo(controller)
			nodePodAffinityInfo.EXPECT().AddPod(Any()).Times(len(test.pods))
			nodePodAffinityInfo.EXPECT().RemovePod(Any()).Times(len(test.rmPods))

			ni := NewNodeInfo(test.node, nodePodAffinityInfo)

			for _, pod := range test.pods {
				_ = ni.AddTask(pod_info.NewTaskInfo(pod))
			}

			for _, pod := range test.rmPods {
				pi := pod_info.NewTaskInfo(pod)
				_ = ni.RemoveTask(pi)
			}

			t.Log(test.expected.PodInfos)
			if !nodeInfoEqual(ni, test.expected) {
				t.Errorf("node info %d: \n expected %v, \n got %v \n",
					i, test.expected, ni)
			}
		})
	}
}

func TestNodeInfo_AddPod(t *testing.T) {
	node1 := common_info.BuildNode("n1", common_info.BuildResourceList("8000m", "10G"))
	podAnnotations := map[string]string{
		pod_info.ReceivedResourceTypeAnnotationName: string(pod_info.ReceivedTypeRegular),
		commonconstants.PodGroupAnnotationForPod:    common_info.FakePogGroupId,
	}
	pod1 := common_info.BuildPod("c1", "p1", "n1", v1.PodRunning,
		common_info.BuildResourceList("1000m", "1G"), []metav1.OwnerReference{},
		make(map[string]string), podAnnotations)
	pod2 := common_info.BuildPod("c1", "p2", "n1", v1.PodRunning,
		common_info.BuildResourceList("2000m", "2G"), []metav1.OwnerReference{},
		make(map[string]string), podAnnotations)

	node1ExpectedNodeInfo := &NodeInfo{
		Name:        "n1",
		Node:        node1,
		Idle:        common_info.BuildResource("5000m", "7G"),
		Used:        common_info.BuildResource("3000m", "3G"),
		Releasing:   resource_info.EmptyResource(),
		Allocatable: common_info.BuildResource("8000m", "10G"),
		PodInfos: map[common_info.PodID]*pod_info.PodInfo{
			"c1/p1": pod_info.NewTaskInfo(pod1),
			"c1/p2": pod_info.NewTaskInfo(pod2),
		},
		LegacyMIGTasks:              map[common_info.PodID]string{},
		MemoryOfEveryGpuOnNode:      DefaultGpuMemory,
		GpuSharingNodeInfo:          *newGpuSharingNodeInfo(),
		AccessibleStorageCapacities: map[common_info.StorageClassID][]*storagecapacity_info.StorageCapacityInfo{},
	}
	for _, podInfo := range node1ExpectedNodeInfo.PodInfos {
		node1ExpectedNodeInfo.setAcceptedResources(podInfo)
	}

	tests := []AddRemovePodsTest{
		{
			name:     "add 2 running non-owner pod",
			node:     node1,
			pods:     []*v1.Pod{pod1, pod2},
			expected: node1ExpectedNodeInfo,
		},
	}

	RunAddRemovePodsTests(t, tests)
}

func TestNodeInfo_RemovePod(t *testing.T) {
	node1 := common_info.BuildNode("n1", common_info.BuildResourceList("8000m", "10G"))

	podAnnotations := map[string]string{
		pod_info.ReceivedResourceTypeAnnotationName: string(pod_info.ReceivedTypeRegular),
		commonconstants.PodGroupAnnotationForPod:    common_info.FakePogGroupId,
	}

	pod1 := common_info.BuildPod("c1", "p1", "n1", v1.PodRunning, common_info.BuildResourceList("1000m", "1G"),
		[]metav1.OwnerReference{}, make(map[string]string), podAnnotations)
	pod2 := common_info.BuildPod("c1", "p2", "n1", v1.PodRunning, common_info.BuildResourceList("2000m", "2G"),
		[]metav1.OwnerReference{}, make(map[string]string), podAnnotations)
	pod3 := common_info.BuildPod("c1", "p3", "n1", v1.PodRunning, common_info.BuildResourceList("3000m", "3G"),
		[]metav1.OwnerReference{}, make(map[string]string), podAnnotations)
	pod1PodInfo := pod_info.NewTaskInfo(pod1)
	pod3PodInfo := pod_info.NewTaskInfo(pod3)

	node1ExpectedNodeInfo := &NodeInfo{
		Name:        "n1",
		Node:        node1,
		Idle:        common_info.BuildResource("4000m", "6G"),
		Used:        common_info.BuildResource("4000m", "4G"),
		Releasing:   resource_info.EmptyResource(),
		Allocatable: common_info.BuildResource("8000m", "10G"),
		PodInfos: map[common_info.PodID]*pod_info.PodInfo{
			"c1/p1": pod1PodInfo,
			"c1/p3": pod3PodInfo,
		},
		LegacyMIGTasks:              map[common_info.PodID]string{},
		MemoryOfEveryGpuOnNode:      DefaultGpuMemory,
		GpuSharingNodeInfo:          *newGpuSharingNodeInfo(),
		AccessibleStorageCapacities: map[common_info.StorageClassID][]*storagecapacity_info.StorageCapacityInfo{},
	}
	node1ExpectedNodeInfo.setAcceptedResources(pod1PodInfo)
	node1ExpectedNodeInfo.setAcceptedResources(pod3PodInfo)

	tests := []AddRemovePodsTest{
		{
			name:     "add 3 running non-owner pod, remove 1 running non-owner pod",
			node:     node1,
			pods:     []*v1.Pod{pod1, pod2, pod3},
			rmPods:   []*v1.Pod{pod2},
			expected: node1ExpectedNodeInfo,
		},
	}

	RunAddRemovePodsTests(t, tests)
}

func TestAddRemovePods(t *testing.T) {
	type podInfoMetadata struct {
		pod       *v1.Pod
		status    pod_status.PodStatus
		gpuGroups []string
	}
	type addRemovePodsTestData struct {
		name                string
		node                *v1.Node
		podsInfoMetadata    []podInfoMetadata
		addedPodsNodeInfo   *NodeInfo
		removedPodsNodeInfo *NodeInfo
	}

	tests := []addRemovePodsTestData{
		{
			name: "releasing pod",
			node: common_info.BuildNode("n1",
				common_info.BuildResourceListWithGPU("8000m", "10G", "1")),
			podsInfoMetadata: []podInfoMetadata{
				{
					pod: common_info.BuildPod("c1", "p1", "n1", v1.PodRunning,
						common_info.BuildResourceListWithGPU("1000m", "1G", "0.5"), []metav1.OwnerReference{},
						make(map[string]string), map[string]string{
							pod_info.ReceivedResourceTypeAnnotationName: string(pod_info.ReceivedTypeRegular),
							commonconstants.PodGroupAnnotationForPod:    common_info.FakePogGroupId,
						}),
					status:    pod_status.Releasing,
					gpuGroups: []string{"1"},
				},
			},
			addedPodsNodeInfo: &NodeInfo{
				Name:                   "n1",
				Idle:                   common_info.BuildResourceWithGpu("7000m", "9G", "0"),
				Used:                   common_info.BuildResourceWithGpu("1000m", "1G", "0"),
				Releasing:              common_info.BuildResourceWithGpu("1000m", "1G", "1"),
				Allocatable:            common_info.BuildResourceWithGpu("8000m", "10G", "1"),
				PodInfos:               map[common_info.PodID]*pod_info.PodInfo{},
				LegacyMIGTasks:         map[common_info.PodID]string{},
				MemoryOfEveryGpuOnNode: DefaultGpuMemory,
				GpuSharingNodeInfo: func() GpuSharingNodeInfo {
					sharingMaps := *newGpuSharingNodeInfo()
					sharingMaps.ReleasingSharedGPUs["1"] = true
					sharingMaps.ReleasingSharedGPUsMemory["1"] = 50
					sharingMaps.UsedSharedGPUsMemory["1"] = 50
					sharingMaps.AllocatedSharedGPUsMemory["1"] = 50
					return sharingMaps
				}(),
				AccessibleStorageCapacities: map[common_info.StorageClassID][]*storagecapacity_info.StorageCapacityInfo{},
			},
			removedPodsNodeInfo: &NodeInfo{
				Name:                   "n1",
				Idle:                   common_info.BuildResourceWithGpu("8000m", "10G", "1"),
				Used:                   resource_info.EmptyResource(),
				Releasing:              resource_info.EmptyResource(),
				Allocatable:            common_info.BuildResourceWithGpu("8000m", "10G", "1"),
				PodInfos:               map[common_info.PodID]*pod_info.PodInfo{},
				LegacyMIGTasks:         map[common_info.PodID]string{},
				MemoryOfEveryGpuOnNode: DefaultGpuMemory,
				GpuSharingNodeInfo: func() GpuSharingNodeInfo {
					sharingMaps := *newGpuSharingNodeInfo()
					sharingMaps.ReleasingSharedGPUsMemory["1"] = 0
					sharingMaps.UsedSharedGPUsMemory["1"] = 0
					sharingMaps.AllocatedSharedGPUsMemory["1"] = 0
					return sharingMaps
				}(),
				AccessibleStorageCapacities: map[common_info.StorageClassID][]*storagecapacity_info.StorageCapacityInfo{},
			},
		},
		{
			name: "pipelined pod - different gpus",
			node: common_info.BuildNode("n1",
				common_info.BuildResourceListWithGPU("8000m", "10G", "1")),
			podsInfoMetadata: []podInfoMetadata{
				{
					pod: common_info.BuildPod("c1", "p1", "n1", v1.PodRunning,
						common_info.BuildResourceListWithGPU("1000m", "1G", "0.5"), []metav1.OwnerReference{},
						make(map[string]string), map[string]string{
							pod_info.ReceivedResourceTypeAnnotationName: string(pod_info.ReceivedTypeRegular),
							commonconstants.PodGroupAnnotationForPod:    common_info.FakePogGroupId,
						}),
					status:    pod_status.Releasing,
					gpuGroups: []string{"1"},
				},
				{
					pod: common_info.BuildPod("c1", "p2", "n1", v1.PodRunning,
						common_info.BuildResourceListWithGPU("500m", "1G", "0.5"), []metav1.OwnerReference{},
						make(map[string]string), map[string]string{
							pod_info.ReceivedResourceTypeAnnotationName: string(pod_info.ReceivedTypeRegular),
							commonconstants.PodGroupAnnotationForPod:    common_info.FakePogGroupId,
						}),
					status:    pod_status.Pipelined,
					gpuGroups: []string{"2"},
				},
			},
			addedPodsNodeInfo: &NodeInfo{
				Name:                   "n1",
				Idle:                   common_info.BuildResourceWithGpu("7000m", "9G", "0"),
				Used:                   common_info.BuildResourceWithGpu("1500m", "2G", "0"),
				Releasing:              common_info.BuildResourceWithGpu("500m", "0G", "0"),
				Allocatable:            common_info.BuildResourceWithGpu("8000m", "10G", "1"),
				PodInfos:               map[common_info.PodID]*pod_info.PodInfo{},
				LegacyMIGTasks:         map[common_info.PodID]string{},
				MemoryOfEveryGpuOnNode: DefaultGpuMemory,
				GpuSharingNodeInfo: func() GpuSharingNodeInfo {
					sharingMaps := *newGpuSharingNodeInfo()
					sharingMaps.ReleasingSharedGPUs["1"] = true
					sharingMaps.ReleasingSharedGPUsMemory["1"] = 50
					sharingMaps.ReleasingSharedGPUsMemory["2"] = -50
					sharingMaps.UsedSharedGPUsMemory["1"] = 50
					sharingMaps.UsedSharedGPUsMemory["2"] = 50
					sharingMaps.AllocatedSharedGPUsMemory["1"] = 50

					return sharingMaps
				}(),
				AccessibleStorageCapacities: map[common_info.StorageClassID][]*storagecapacity_info.StorageCapacityInfo{},
			},
			removedPodsNodeInfo: &NodeInfo{
				Name:                   "n1",
				Idle:                   common_info.BuildResourceWithGpu("8000m", "10G", "1"),
				Used:                   resource_info.EmptyResource(),
				Releasing:              resource_info.EmptyResource(),
				Allocatable:            common_info.BuildResourceWithGpu("8000m", "10G", "1"),
				PodInfos:               map[common_info.PodID]*pod_info.PodInfo{},
				LegacyMIGTasks:         map[common_info.PodID]string{},
				MemoryOfEveryGpuOnNode: DefaultGpuMemory,
				GpuSharingNodeInfo: func() GpuSharingNodeInfo {
					sharingMaps := *newGpuSharingNodeInfo()
					sharingMaps.ReleasingSharedGPUsMemory["1"] = 0
					sharingMaps.ReleasingSharedGPUsMemory["2"] = 0
					sharingMaps.UsedSharedGPUsMemory["1"] = 0
					sharingMaps.UsedSharedGPUsMemory["2"] = 0
					sharingMaps.AllocatedSharedGPUsMemory["1"] = 0
					return sharingMaps
				}(),
				AccessibleStorageCapacities: map[common_info.StorageClassID][]*storagecapacity_info.StorageCapacityInfo{},
			},
		},
		{
			name: "pipelined pod - same gpus",
			node: common_info.BuildNode("n1",
				common_info.BuildResourceListWithGPU("8000m", "10G", "1")),
			podsInfoMetadata: []podInfoMetadata{
				{
					pod: common_info.BuildPod("c1", "p1", "n1", v1.PodRunning,
						common_info.BuildResourceListWithGPU("1000m", "1G", "0.5"), []metav1.OwnerReference{},
						make(map[string]string), map[string]string{
							pod_info.ReceivedResourceTypeAnnotationName: string(pod_info.ReceivedTypeRegular),
							commonconstants.PodGroupAnnotationForPod:    common_info.FakePogGroupId,
						}),
					status:    pod_status.Releasing,
					gpuGroups: []string{"1"},
				},
				{
					pod: common_info.BuildPod("c1", "p2", "n1", v1.PodRunning,
						common_info.BuildResourceListWithGPU("1000m", "1G", "0.2"), []metav1.OwnerReference{},
						make(map[string]string), map[string]string{
							pod_info.ReceivedResourceTypeAnnotationName: string(pod_info.ReceivedTypeRegular),
							commonconstants.PodGroupAnnotationForPod:    common_info.FakePogGroupId,
						}),
					status:    pod_status.Running,
					gpuGroups: []string{"1"},
				},
				{
					pod: common_info.BuildPod("c1", "p3", "n1", v1.PodRunning,
						common_info.BuildResourceListWithGPU("500m", "1G", "0.5"), []metav1.OwnerReference{},
						make(map[string]string), map[string]string{
							pod_info.ReceivedResourceTypeAnnotationName: string(pod_info.ReceivedTypeRegular),
							commonconstants.PodGroupAnnotationForPod:    common_info.FakePogGroupId,
						}),
					status:    pod_status.Pipelined,
					gpuGroups: []string{"1"},
				},
			},
			addedPodsNodeInfo: &NodeInfo{
				Name:                   "n1",
				Idle:                   common_info.BuildResourceWithGpu("6000m", "8G", "0"),
				Used:                   common_info.BuildResourceWithGpu("2500m", "3G", "0"),
				Releasing:              common_info.BuildResourceWithGpu("500m", "0G", "0"),
				Allocatable:            common_info.BuildResourceWithGpu("8000m", "10G", "1"),
				PodInfos:               map[common_info.PodID]*pod_info.PodInfo{},
				LegacyMIGTasks:         map[common_info.PodID]string{},
				MemoryOfEveryGpuOnNode: DefaultGpuMemory,
				GpuSharingNodeInfo: func() GpuSharingNodeInfo {
					sharingMaps := *newGpuSharingNodeInfo()
					sharingMaps.ReleasingSharedGPUsMemory["1"] = 0
					sharingMaps.UsedSharedGPUsMemory["1"] = 120
					sharingMaps.AllocatedSharedGPUsMemory["1"] = 70
					return sharingMaps
				}(),
				AccessibleStorageCapacities: map[common_info.StorageClassID][]*storagecapacity_info.StorageCapacityInfo{},
			},
			removedPodsNodeInfo: &NodeInfo{
				Name:                   "n1",
				Idle:                   common_info.BuildResourceWithGpu("8000m", "10G", "1"),
				Used:                   resource_info.EmptyResource(),
				Releasing:              resource_info.EmptyResource(),
				Allocatable:            common_info.BuildResourceWithGpu("8000m", "10G", "1"),
				PodInfos:               map[common_info.PodID]*pod_info.PodInfo{},
				LegacyMIGTasks:         map[common_info.PodID]string{},
				MemoryOfEveryGpuOnNode: DefaultGpuMemory,
				GpuSharingNodeInfo: func() GpuSharingNodeInfo {
					sharingMaps := *newGpuSharingNodeInfo()
					sharingMaps.ReleasingSharedGPUsMemory["1"] = 0
					sharingMaps.UsedSharedGPUsMemory["1"] = 0
					sharingMaps.AllocatedSharedGPUsMemory["1"] = 0
					return sharingMaps
				}(),
				AccessibleStorageCapacities: map[common_info.StorageClassID][]*storagecapacity_info.StorageCapacityInfo{},
			},
		},
	}

	for i, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Logf("Running test %d: %s", i, test.name)

			test.addedPodsNodeInfo.Node = test.node
			test.removedPodsNodeInfo.Node = test.node

			controller := NewController(t)
			nodePodAffinityInfoAdded := pod_affinity.NewMockNodePodAffinityInfo(controller)
			nodePodAffinityInfoAdded.EXPECT().AddPod(Any()).Times(len(test.podsInfoMetadata))

			ni := NewNodeInfo(test.node, nodePodAffinityInfoAdded)
			var podsInfo []*pod_info.PodInfo
			for _, podInfoMetaData := range test.podsInfoMetadata {
				pi := pod_info.NewTaskInfo(podInfoMetaData.pod)
				pi.Status = podInfoMetaData.status
				pi.GPUGroups = podInfoMetaData.gpuGroups
				podsInfo = append(podsInfo, pi)
			}

			for _, podInfo := range podsInfo {
				_ = ni.AddTask(podInfo)
				podInfoKey := common_info.PodID(fmt.Sprintf("%s/%s", podInfo.Namespace, podInfo.Name))
				test.addedPodsNodeInfo.PodInfos[podInfoKey] = podInfo
			}
			if !nodeInfoEqual(ni, test.addedPodsNodeInfo) {
				t.Errorf("pods added info: \n expected %v, \n got %v \n",
					test.addedPodsNodeInfo, ni)
			}

			nodePodAffinityInfoRemoved := pod_affinity.NewMockNodePodAffinityInfo(controller)
			nodePodAffinityInfoRemoved.EXPECT().RemovePod(Any()).Times(len(test.podsInfoMetadata))
			ni.PodAffinityInfo = nodePodAffinityInfoRemoved

			// This is an important line - the remove code might not work if it won't remove in reverse order
			// Because the statement discard in reverse order to operations addition, it should be fine.
			slices.Reverse(podsInfo)

			for _, podInfo := range podsInfo {
				_ = ni.RemoveTask(podInfo)
			}
			if !nodeInfoEqual(ni, test.removedPodsNodeInfo) {
				t.Errorf("pods removed info: \n expected %v, \n got %v \n",
					test.removedPodsNodeInfo, ni)
			}
		})
	}
}

func TestConvertMibToMb(t *testing.T) {
	mibSize := int64(100)
	mbSize := convertMibToMb(mibSize)
	mbSizeManualConversion := int64(float64(mibSize) * MibToMbScale)
	assert.Equal(t, mbSize, mbSizeManualConversion)
}

type allocatableTestData struct {
	node                    *v1.Node
	podsResources           []v1.ResourceList
	podResourcesToAllocate  v1.ResourceList
	podAnnotations          map[string]string
	expected                bool
	expectedMessageContains []string
}
type allocatableTestFunction func(ni *NodeInfo, task *pod_info.PodInfo) (bool, error)

func TestIsTaskAllocatable(t *testing.T) {
	nodeCapacityDifferent := common_info.BuildNode("n2", common_info.BuildResourceList("1000m", "1G"))
	nodeCapacityDifferent.Status.Capacity = common_info.BuildResourceList("2000m", "2G")

	tests := map[string]allocatableTestData{
		"add pod with not enough cpu and memory": {
			node:                    common_info.BuildNode("n1", common_info.BuildResourceList("2000m", "2G")),
			podsResources:           []v1.ResourceList{common_info.BuildResourceList("1000m", "1G")},
			podResourcesToAllocate:  common_info.BuildResourceList("2000m", "2G"),
			expected:                false,
			expectedMessageContains: []string{"CPU cores", "memory"},
		},
		"add pod with not enough cpu - 1 millicpu": {
			node:                    common_info.BuildNode("n1", common_info.BuildResourceList("2000m", "2G")),
			podsResources:           []v1.ResourceList{common_info.BuildResourceList("1000m", "1G")},
			podResourcesToAllocate:  common_info.BuildResourceList("1001m", "1G"),
			expected:                false,
			expectedMessageContains: []string{"CPU cores"},
		},
		"add pod with not enough memory - 1 Kb": {
			node:                    common_info.BuildNode("n1", common_info.BuildResourceList("2000m", "2G")),
			podsResources:           []v1.ResourceList{common_info.BuildResourceList("1000m", "2G")},
			podResourcesToAllocate:  common_info.BuildResourceList("1000m", "1Ki"),
			expected:                false,
			expectedMessageContains: []string{"memory"},
		},
		"add pod with enough cpu and memory": {
			node:                    common_info.BuildNode("n1", common_info.BuildResourceList("2000m", "2G")),
			podsResources:           []v1.ResourceList{common_info.BuildResourceList("1000m", "1G")},
			podResourcesToAllocate:  common_info.BuildResourceList("1000m", "1G"),
			expected:                true,
			expectedMessageContains: []string{},
		},
		"task in capacity but not in available": {
			node:                    nodeCapacityDifferent,
			podsResources:           []v1.ResourceList{common_info.BuildResourceList("1000m", "1G")},
			podResourcesToAllocate:  common_info.BuildResourceList("1000m", "1G"),
			expected:                false,
			expectedMessageContains: []string{"CPU cores", "memory"},
		},
		"missing gpu": {
			node:                    common_info.BuildNode("n1", common_info.BuildResourceList("2000m", "2G")),
			podsResources:           []v1.ResourceList{},
			podResourcesToAllocate:  common_info.BuildResourceListWithGPU("1000m", "1G", "1"),
			expected:                false,
			expectedMessageContains: []string{"GPU"},
		},
		"missing gpu - requesting a fraction": {
			node:          common_info.BuildNode("n1", common_info.BuildResourceList("2000m", "2G")),
			podsResources: []v1.ResourceList{},
			// the build pod function will convert it to annotation similarly to the admission controller
			podResourcesToAllocate:  common_info.BuildResourceListWithGPU("1000m", "1G", "500m"),
			expected:                false,
			expectedMessageContains: []string{"GPU"},
		},
		"already used gpu so missing gpu": {
			node:                    common_info.BuildNode("n1", common_info.BuildResourceListWithGPU("2000m", "2G", "1")),
			podsResources:           []v1.ResourceList{common_info.BuildResourceListWithGPU("1000m", "1G", "1")},
			podResourcesToAllocate:  common_info.BuildResourceListWithGPU("1000m", "1G", "1"),
			expected:                false,
			expectedMessageContains: []string{"GPU"},
		},
		"enough cpu memory and gpu": {
			node:                    common_info.BuildNode("n1", common_info.BuildResourceListWithGPU("2000m", "2G", "2")),
			podsResources:           []v1.ResourceList{common_info.BuildResourceListWithGPU("1000m", "1G", "1")},
			podResourcesToAllocate:  common_info.BuildResourceListWithGPU("1000m", "1G", "1"),
			expected:                true,
			expectedMessageContains: []string{},
		},
	}

	for testName, testData := range tests {
		t.Run(testName, func(t *testing.T) {
			runAllocatableTest(
				t, testData, testName,
				func(ni *NodeInfo, task *pod_info.PodInfo) (bool, error) {
					return ni.IsTaskAllocatable(task), ni.FittingError(task, false)
				},
			)
		})
	}
}

func TestIsTaskAllocatableOnReleasingOrIdle(t *testing.T) {
	singleMigNode := common_info.BuildNode("single-mig", common_info.BuildResourceListWithGPU("2000m", "2G", "8"))
	singleMigNode.Labels[commonconstants.MigEnabledLabel] = "true"

	mixedMigNode := common_info.BuildNode("mixed-mig", common_info.BuildResourceListWithGPU("2000m", "2G", "8"))
	mixedMigNode.Labels[commonconstants.MigEnabledLabel] = "true"
	mixedMigNode.Labels[commonconstants.GpuCountLabel] = "8"
	mixedMigNode.Labels[GpuMemoryLabel] = "40"

	tests := map[string]allocatableTestData{
		"cpu only job on single mig node with enough resources": {
			node:                   singleMigNode,
			podsResources:          []v1.ResourceList{common_info.BuildResourceList("200m", "500m")},
			podResourcesToAllocate: common_info.BuildResourceList("200m", "500m"),
			expected:               true,
		},
		"cpu only job on mixed mig node with enough resources": {
			node:                   mixedMigNode,
			podsResources:          []v1.ResourceList{common_info.BuildResourceList("200m", "500m")},
			podResourcesToAllocate: common_info.BuildResourceList("200m", "500m"),
			expected:               true,
		},
	}

	for testName, testData := range tests {
		t.Run(testName, func(t *testing.T) {
			runAllocatableTest(
				t, testData, testName,
				func(ni *NodeInfo, task *pod_info.PodInfo) (bool, error) {
					return ni.IsTaskAllocatableOnReleasingOrIdle(task), nil
				},
			)
		})
	}
}

func runAllocatableTest(
	t *testing.T, testData allocatableTestData, testName string,
	testedFunction allocatableTestFunction,
) {
	controller := NewController(t)
	nodePodAffinityInfo := pod_affinity.NewMockNodePodAffinityInfo(controller)
	nodePodAffinityInfo.EXPECT().AddPod(Any()).Times(len(testData.podsResources))

	ni := NewNodeInfo(testData.node, nodePodAffinityInfo)
	for ind, podResouces := range testData.podsResources {
		pod := common_info.BuildPod(
			fmt.Sprintf("p%d", ind), "p1", "n1", v1.PodRunning, podResouces,
			[]metav1.OwnerReference{}, make(map[string]string), map[string]string{})
		addJobAnnotation(pod)
		pi := pod_info.NewTaskInfo(pod)
		if err := ni.AddTask(pi); err != nil {
			t.Errorf("%s: failed to add pod %v, index: %d", testName, pi, ind)
		}
	}
	pod := common_info.BuildPod(
		"podToAllocate", "p1", "n1", v1.PodRunning, testData.podResourcesToAllocate,
		[]metav1.OwnerReference{}, make(map[string]string), testData.podAnnotations)
	addJobAnnotation(pod)
	task := pod_info.NewTaskInfo(pod)
	allocatable, fitErr := testedFunction(ni, task)
	if allocatable != testData.expected {
		t.Errorf("%s: is pod allocatable: expected %v, got %v", testName, testData.expected, allocatable)
	}
	if fitErr != nil {
		for _, expectedMessage := range testData.expectedMessageContains {
			if !strings.Contains(fitErr.Error(), expectedMessage) {
				t.Errorf("%s: expected error message to contain %s, got %s", testName, expectedMessage, fitErr.Error())
			}
		}
	}
}

func TestGpuOperatorHasMemoryError_MibInput(t *testing.T) {
	testNode := common_info.BuildNode("n1", common_info.BuildResourceList("8000m", "10G"))
	testNode.Labels[GpuMemoryLabel] = "4096"
	gpuMemoryInMb, ok := getNodeGpuMemory(testNode)
	assert.Equal(t, true, ok)
	assert.Equal(t, int64(4200), gpuMemoryInMb)
}

func TestGpuOperatorHasMemoryError_Bytes(t *testing.T) {
	testNode := common_info.BuildNode("n1", common_info.BuildResourceList("8000m", "10G"))
	testNode.Labels[GpuMemoryLabel] = "4295000001"
	gpuMemoryInMb, ok := getNodeGpuMemory(testNode)
	assert.Equal(t, true, ok)
	assert.Equal(t, int64(4200), gpuMemoryInMb)
}

func addJobAnnotation(pod *v1.Pod) {
	if pod.Annotations == nil {
		pod.Annotations = make(map[string]string)
	}
	pod.Annotations[commonconstants.PodGroupAnnotationForPod] = pod.Name
}

func TestNodeInfo_isTaskAllocatableOnNonAllocatedResources(t *testing.T) {
	type fields struct {
		Name                   string
		Node                   *v1.Node
		Releasing              *resource_info.Resource
		Idle                   *resource_info.Resource
		Used                   *resource_info.Resource
		Allocatable            *resource_info.Resource
		PodInfos               map[common_info.PodID]*pod_info.PodInfo
		MaxTaskNum             int
		MemoryOfEveryGpuOnNode int64
		GpuMemorySynced        bool
		PodAffinityInfo        pod_affinity.NodePodAffinityInfo
		GpuSharingNodeInfo     GpuSharingNodeInfo
	}
	type args struct {
		task                      *pod_info.PodInfo
		nodeNonAllocatedResources *resource_info.Resource
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   bool
	}{
		{
			"invalid fraction from memory request",
			fields{
				MemoryOfEveryGpuOnNode: 1000,
				GpuMemorySynced:        true,
				Idle:                   resource_info.NewResource(0, 0, 2),
				Used:                   resource_info.EmptyResource(),
				Releasing:              resource_info.EmptyResource(),
				Allocatable:            resource_info.NewResource(0, 0, 2),
			},
			args{
				task: pod_info.NewTaskInfo(
					&v1.Pod{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "p1",
							Namespace: "n1",
							Annotations: map[string]string{
								commonconstants.PodGroupAnnotationForPod: "pg1",
								pod_info.GpuMemoryAnnotationName:         "1500",
							},
						},
						Spec: v1.PodSpec{
							Containers: []v1.Container{
								{
									Name: "c1",
								},
							},
						},
					},
				),
				nodeNonAllocatedResources: resource_info.NewResource(0, 0, 2),
			},
			false,
		},
		{
			"valid fraction from memory request",
			fields{
				MemoryOfEveryGpuOnNode: 1000,
				GpuMemorySynced:        true,
				Idle:                   resource_info.NewResource(0, 0, 2),
				Used:                   resource_info.EmptyResource(),
				Releasing:              resource_info.EmptyResource(),
				Allocatable:            resource_info.NewResource(0, 0, 2),
			},
			args{
				task: pod_info.NewTaskInfo(
					&v1.Pod{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "p1",
							Namespace: "n1",
							Annotations: map[string]string{
								commonconstants.PodGroupAnnotationForPod: "pg1",
								pod_info.GpuMemoryAnnotationName:         "1000",
							},
						},
						Spec: v1.PodSpec{
							Containers: []v1.Container{
								{
									Name: "c1",
								},
							},
						},
					},
				),
				nodeNonAllocatedResources: resource_info.NewResource(0, 0, 2),
			},
			true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ni := &NodeInfo{
				Name:                   tt.fields.Name,
				Node:                   tt.fields.Node,
				Releasing:              tt.fields.Releasing,
				Idle:                   tt.fields.Idle,
				Used:                   tt.fields.Used,
				Allocatable:            tt.fields.Allocatable,
				PodInfos:               tt.fields.PodInfos,
				MaxTaskNum:             tt.fields.MaxTaskNum,
				MemoryOfEveryGpuOnNode: tt.fields.MemoryOfEveryGpuOnNode,
				GpuMemorySynced:        tt.fields.GpuMemorySynced,
				PodAffinityInfo:        tt.fields.PodAffinityInfo,
				GpuSharingNodeInfo:     tt.fields.GpuSharingNodeInfo,
			}
			assert.Equalf(t, tt.want,
				ni.isTaskAllocatableOnNonAllocatedResources(tt.args.task, tt.args.nodeNonAllocatedResources),
				"isTaskAllocatableOnNonAllocatedResources(%v, %v)", tt.args.task, tt.args.nodeNonAllocatedResources)
		})
	}
}

func TestNodeInfo_GetSumOfIdleGPUs(t *testing.T) {
	tests := []struct {
		name           string
		capacity       map[v1.ResourceName]resource.Quantity
		allocatable    map[v1.ResourceName]resource.Quantity
		tasks          []*pod_info.PodInfo
		expectedGPUs   float64
		expectedMemory int64
	}{
		{
			name:           "no tasks",
			capacity:       common_info.BuildResourceListWithGPU("8000m", "10G", "2"),
			allocatable:    common_info.BuildResourceListWithGPU("8000m", "10G", "2"),
			tasks:          []*pod_info.PodInfo{},
			expectedGPUs:   2,
			expectedMemory: 200,
		},
		{
			name:        "whole GPU tasks",
			capacity:    common_info.BuildResourceListWithGPU("8000m", "10G", "8"),
			allocatable: common_info.BuildResourceListWithGPU("8000m", "10G", "8"),
			tasks: []*pod_info.PodInfo{
				createPod("team-a", "pod1", podCreationOptions{GPUs: 2}),
				createPod("team-a", "pod2", podCreationOptions{GPUs: 1}),
			},
			expectedGPUs:   5,
			expectedMemory: 500,
		},
		{
			name:        "fraction GPU tasks on different devices",
			capacity:    common_info.BuildResourceListWithGPU("8000m", "10G", "8"),
			allocatable: common_info.BuildResourceListWithGPU("8000m", "10G", "8"),
			tasks: []*pod_info.PodInfo{
				createPod("team-a", "pod1", podCreationOptions{GPUs: 0.5, gpuGroup: "group1"}),
				createPod("team-a", "pod2", podCreationOptions{GPUs: 0.1, gpuGroup: "group2"}),
			},
			expectedGPUs:   7.4,
			expectedMemory: 740,
		},
		{
			name:        "fraction GPU tasks on same device",
			capacity:    common_info.BuildResourceListWithGPU("8000m", "10G", "8"),
			allocatable: common_info.BuildResourceListWithGPU("8000m", "10G", "8"),
			tasks: []*pod_info.PodInfo{
				createPod("team-a", "pod1", podCreationOptions{GPUs: 0.5, gpuGroup: "group1"}),
				createPod("team-a", "pod2", podCreationOptions{GPUs: 0.1, gpuGroup: "group1"}),
			},
			expectedGPUs:   7.4,
			expectedMemory: 740,
		},
		{
			name:        "mixed task types",
			capacity:    common_info.BuildResourceListWithGPU("8000m", "10G", "8"),
			allocatable: common_info.BuildResourceListWithGPU("8000m", "10G", "8"),
			tasks: []*pod_info.PodInfo{
				createPod("team-a", "pod1", podCreationOptions{GPUs: 2}),
				createPod("team-a", "pod2", podCreationOptions{GPUs: 0.5, gpuGroup: "group1"}),
				createPod("team-a", "pod3", podCreationOptions{GPUs: 1}),
				createPod("team-a", "pod4", podCreationOptions{GPUs: 0.1, gpuGroup: "group2"}),
			},
			expectedGPUs:   4.4,
			expectedMemory: 440,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			node := &v1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "node1",
				},
				Status: v1.NodeStatus{
					Capacity:    tt.capacity,
					Allocatable: tt.allocatable,
				},
			}

			controller := NewController(t)
			nodePodAffinity := pod_affinity.NewMockNodePodAffinityInfo(controller)
			nodePodAffinity.EXPECT().AddPod(Any()).Times(len(tt.tasks))

			ni := NewNodeInfo(node, nodePodAffinity)
			for _, task := range tt.tasks {
				assert.Nil(t, ni.AddTask(task))
			}
			idleGPUs, idleMemory := ni.GetSumOfIdleGPUs()
			assert.Equalf(t, tt.expectedGPUs, idleGPUs,
				"test: %s, expected GPU %f, got %f", tt.name, tt.expectedGPUs, idleGPUs)
			assert.Equalf(t, tt.expectedMemory, idleMemory,
				"test: %s, expected GPU memory %d, got %d", tt.name, tt.expectedMemory, idleMemory)
		})
	}
}

func TestNodeInfo_GetSumOfReleasingGPUs(t *testing.T) {
	tests := []struct {
		name           string
		capacity       map[v1.ResourceName]resource.Quantity
		allocatable    map[v1.ResourceName]resource.Quantity
		tasks          []*pod_info.PodInfo
		expectedGPUs   float64
		expectedMemory int64
	}{
		{
			name:           "no tasks",
			capacity:       common_info.BuildResourceListWithGPU("8000m", "10G", "2"),
			allocatable:    common_info.BuildResourceListWithGPU("8000m", "10G", "2"),
			tasks:          []*pod_info.PodInfo{},
			expectedGPUs:   0,
			expectedMemory: 0,
		},
		{
			name:        "whole GPU tasks",
			capacity:    common_info.BuildResourceListWithGPU("8000m", "10G", "8"),
			allocatable: common_info.BuildResourceListWithGPU("8000m", "10G", "8"),
			tasks: []*pod_info.PodInfo{
				createPod("team-a", "pod1", podCreationOptions{GPUs: 2, releasing: true}),
				createPod("team-a", "pod2", podCreationOptions{GPUs: 1, releasing: true}),
			},
			expectedGPUs:   3,
			expectedMemory: 300,
		},
		{
			name:        "fraction GPU tasks on different devices",
			capacity:    common_info.BuildResourceListWithGPU("8000m", "10G", "8"),
			allocatable: common_info.BuildResourceListWithGPU("8000m", "10G", "8"),
			tasks: []*pod_info.PodInfo{
				createPod("team-a", "pod1",
					podCreationOptions{GPUs: 0.5, releasing: true, gpuGroup: "group1"}),
				createPod("team-a", "pod2",
					podCreationOptions{GPUs: 0.1, releasing: true, gpuGroup: "group2"}),
			},
			expectedGPUs:   2,
			expectedMemory: 200,
		},
		{
			name:        "fraction GPU tasks on same device",
			capacity:    common_info.BuildResourceListWithGPU("8000m", "10G", "8"),
			allocatable: common_info.BuildResourceListWithGPU("8000m", "10G", "8"),
			tasks: []*pod_info.PodInfo{
				createPod("team-a", "pod1",
					podCreationOptions{GPUs: 0.5, releasing: true, gpuGroup: "group1"}),
				createPod("team-a", "pod2",
					podCreationOptions{GPUs: 0.1, releasing: true, gpuGroup: "group1"}),
			},
			expectedGPUs:   1,
			expectedMemory: 100,
		},
		{
			name:        "mixed task types",
			capacity:    common_info.BuildResourceListWithGPU("8000m", "10G", "8"),
			allocatable: common_info.BuildResourceListWithGPU("8000m", "10G", "8"),
			tasks: []*pod_info.PodInfo{
				createPod("team-a", "pod1", podCreationOptions{GPUs: 2, releasing: true}),
				createPod("team-a", "pod2",
					podCreationOptions{GPUs: 0.5, releasing: true, gpuGroup: "group1"}),
				createPod("team-a", "pod3", podCreationOptions{GPUs: 1, releasing: true}),
			},
			expectedGPUs:   4,
			expectedMemory: 400,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			node := &v1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "node1",
				},
				Status: v1.NodeStatus{
					Capacity:    tt.capacity,
					Allocatable: tt.allocatable,
				},
			}

			controller := NewController(t)
			nodePodAffinity := pod_affinity.NewMockNodePodAffinityInfo(controller)
			nodePodAffinity.EXPECT().AddPod(Any()).Times(len(tt.tasks))

			ni := NewNodeInfo(node, nodePodAffinity)
			for _, task := range tt.tasks {
				assert.Nil(t, ni.AddTask(task))
			}
			idleGPUs, idleMemory := ni.GetSumOfReleasingGPUs()
			assert.Equalf(t, tt.expectedGPUs, idleGPUs,
				"test: %s, expected GPU %f, got %f", tt.name, tt.expectedGPUs, idleGPUs)
			assert.Equalf(t, tt.expectedMemory, idleMemory,
				"test: %s, expected GPU memory %d, got %d", tt.name, tt.expectedMemory, idleMemory)
		})
	}
}

func createPod(namespace, name string, options podCreationOptions) *pod_info.PodInfo {
	pod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Annotations: map[string]string{
				commonconstants.PodGroupAnnotationForPod: "pg1",
			},
		},
		Spec: v1.PodSpec{
			NodeName: "node1",
			Containers: []v1.Container{
				{
					Resources: v1.ResourceRequirements{
						Requests: v1.ResourceList{},
						Limits:   v1.ResourceList{},
					},
				},
			},
		},
		Status: v1.PodStatus{
			Phase: v1.PodRunning,
		},
	}

	if options.releasing {
		pod.DeletionTimestamp = &metav1.Time{Time: time.Now()}
	}

	numGPUsStr := strconv.FormatFloat(options.GPUs, 'f', -1, 64)
	if options.GPUs >= 1 {
		pod.Spec.Containers[0].Resources.Requests[resource_info.GPUResourceName] = resource.MustParse(numGPUsStr)
		pod.Spec.Containers[0].Resources.Limits[resource_info.GPUResourceName] = resource.MustParse(numGPUsStr)
	} else {
		pod.Annotations[commonconstants.GpuFraction] = numGPUsStr
	}

	task := pod_info.NewTaskInfo(pod)
	task.GPUGroups = []string{options.gpuGroup}
	return task
}

func Test_isMigResource(t *testing.T) {
	tests := []struct {
		name  string
		rName string
		want  bool
	}{
		{
			"is mig",
			"nvidia.com/mig-1g.5gb",
			true,
		},
		{
			"not mig",
			"nvidia.com/gpu",
			false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isMigResource(tt.rName); got != tt.want {
				t.Errorf("isMigResource() = %v, want %v", got, tt.want)
			}
		})
	}
}
