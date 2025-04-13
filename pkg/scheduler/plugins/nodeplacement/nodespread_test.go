// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package nodeplacement_test

import (
	"strconv"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/node_info"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/pod_info"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/resource_info"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/constants"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/plugins/nodeplacement"

	commonconstants "github.com/NVIDIA/KAI-scheduler/pkg/common/constants"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestNodeSpread(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "NodeSpread Suite")
}

var _ = Describe("NodeSpread", func() {
	Describe("calculateScore", func() {
		It("should score gpu jobs based on free whole gpus", func() {
			cases := []struct {
				gpuCount     int
				nonAllocated float64
				expected     float64
			}{
				{
					gpuCount:     0,
					nonAllocated: 0,
					expected:     0,
				},
				{
					gpuCount:     1,
					nonAllocated: 0,
					expected:     0,
				},
				{
					gpuCount:     1,
					nonAllocated: 1,
					expected:     1,
				},
				{
					gpuCount:     2,
					nonAllocated: 1,
					expected:     0.5,
				},
				{
					gpuCount:     4,
					nonAllocated: 1,
					expected:     0.25,
				},
				{
					gpuCount:     4,
					nonAllocated: 3,
					expected:     0.75,
				},
			}

			for _, c := range cases {
				task := pod_info.NewTaskInfo(&corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-pod",
						Annotations: map[string]string{
							"kai.scheduler/placementStrategy":        "spread",
							commonconstants.PodGroupAnnotationForPod: "test-group",
						},
					},
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{
							{
								Resources: corev1.ResourceRequirements{
									Requests: corev1.ResourceList{
										resource_info.GPUResourceName: resource.MustParse("1"),
									},
								},
							},
						},
					},
				})

				node := &node_info.NodeInfo{
					Node: &corev1.Node{
						ObjectMeta: metav1.ObjectMeta{
							Labels: map[string]string{
								node_info.GpuCountLabel: strconv.Itoa(c.gpuCount),
							},
						},
					},
					Idle:      resource_info.NewResource(0, 0, c.nonAllocated),
					Releasing: resource_info.EmptyResource(),
				}

				plugin := nodeplacement.New(map[string]string{
					constants.GPUResource: constants.SpreadStrategy,
					constants.CPUResource: constants.SpreadStrategy,
				})
				ssn := createFakeTestSession(map[string]*node_info.NodeInfo{node.Name: node})
				Expect(ssn.NodeOrderFns).To(HaveLen(0), "NodeOrderFns should be empty")
				plugin.OnSessionOpen(ssn)
				Expect(ssn.NodeOrderFns).To(HaveLen(1), "NodeOrderFns should have one element")
				nof := ssn.NodeOrderFns[len(ssn.NodeOrderFns)-1]

				actual, err := nof(task, node)
				Expect(err).To(Not(HaveOccurred()))
				Expect(actual).To(Equal(c.expected))

				task = pod_info.NewTaskInfo(&corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-pod-cpu",
						Annotations: map[string]string{
							"kai.scheduler/placementStrategy":        "spread",
							commonconstants.PodGroupAnnotationForPod: "test-group",
						},
					},
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{
							{
								Resources: corev1.ResourceRequirements{
									Requests: corev1.ResourceList{
										corev1.ResourceCPU: resource.MustParse("1"),
									},
								},
							},
						},
					},
				})

				node = &node_info.NodeInfo{
					Node:        &corev1.Node{},
					Idle:        resource_info.NewResource(c.nonAllocated, 0, 0),
					Allocatable: resource_info.NewResource(float64(c.gpuCount), 0, 0),
					Releasing:   resource_info.EmptyResource(),
				}

				actual, err = nof(task, node)
				Expect(err).To(Not(HaveOccurred()))
				Expect(actual).To(Equal(c.expected))
			}
		})
	})
})
