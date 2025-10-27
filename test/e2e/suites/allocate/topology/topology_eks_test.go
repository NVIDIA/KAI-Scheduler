/*
Copyright 2025 NVIDIA CORPORATION
SPDX-License-Identifier: Apache-2.0
*/

package topology

import (
	"context"

	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/client-go/kubernetes"
	kueuev1alpha1 "sigs.k8s.io/kueue/apis/kueue/v1alpha1"
	kueue "sigs.k8s.io/kueue/client-go/clientset/versioned"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	v2 "github.com/NVIDIA/KAI-scheduler/pkg/apis/scheduling/v2"
	"github.com/NVIDIA/KAI-scheduler/pkg/apis/scheduling/v2alpha2"
	"github.com/NVIDIA/KAI-scheduler/pkg/common/constants"
	testcontext "github.com/NVIDIA/KAI-scheduler/test/e2e/modules/context"
	"github.com/NVIDIA/KAI-scheduler/test/e2e/modules/resources/rd"
	"github.com/NVIDIA/KAI-scheduler/test/e2e/modules/resources/rd/pod_group"
	"github.com/NVIDIA/KAI-scheduler/test/e2e/modules/resources/rd/queue"
	"github.com/NVIDIA/KAI-scheduler/test/e2e/modules/utils"
	"github.com/NVIDIA/KAI-scheduler/test/e2e/modules/wait"
)

const (
	TopologyName   = "topology"
	EksRegionLabel = "topology.kubernetes.io/region"
	EksZoneLabel   = "topology.kubernetes.io/zone"
)

type subGroupPods struct {
	parent             string
	topologyConstraint *v2alpha2.TopologyConstraint

	//Leaf
	numPods     int
	podResource v1.ResourceRequirements

	//Do not fill
	pods []*v1.Pod
}

var _ = Describe("Topology EKS", Ordered, func() {
	var (
		testCtx          *testcontext.TestContext
		testTopologyData rd.TestTopologyData
		kueueClient      *kueue.Clientset
	)

	BeforeAll(func(ctx context.Context) {
		testCtx = testcontext.GetConnectivity(ctx, Default)
		parentQueue := queue.CreateQueueObject(utils.GenerateRandomK8sName(10), "")
		childQueue := queue.CreateQueueObject(utils.GenerateRandomK8sName(10), parentQueue.Name)
		testCtx.InitQueues([]*v2.Queue{childQueue, parentQueue})

		testTopologyData.TopologyCrd = &kueuev1alpha1.Topology{
			ObjectMeta: metav1.ObjectMeta{
				Name: TopologyName,
			},
			Spec: kueuev1alpha1.TopologySpec{
				Levels: []kueuev1alpha1.TopologyLevel{
					{NodeLabel: "topology.kubernetes.io/region"},
					{NodeLabel: EksZoneLabel},
				},
			},
		}
		kueueClient = kueue.NewForConfigOrDie(testCtx.KubeConfig)
		_, err := kueueClient.KueueV1alpha1().Topologies().Create(
			context.TODO(), testTopologyData.TopologyCrd, metav1.CreateOptions{})
		Expect(err).NotTo(HaveOccurred(), "Failed to create topology tree")
	})

	AfterAll(func(ctx context.Context) {
		err := kueueClient.KueueV1alpha1().Topologies().Delete(ctx, TopologyName, metav1.DeleteOptions{})
		Expect(err).NotTo(HaveOccurred(), "Failed to delete topology tree")
		testCtx.ClusterCleanup(ctx)
	})

	AfterEach(func(ctx context.Context) {
		testCtx.TestContextCleanup(ctx)
	})

	It("Require - 3 subgroups, 2 leafs 1 parent", func(ctx context.Context) {
		subGroups := map[string]*subGroupPods{
			"sg-p": {
				topologyConstraint: &v2alpha2.TopologyConstraint{
					Topology:              TopologyName,
					RequiredTopologyLevel: EksRegionLabel,
				},
			},
			"sg-1": {
				parent: "sg-p",
				topologyConstraint: &v2alpha2.TopologyConstraint{
					Topology:              TopologyName,
					RequiredTopologyLevel: EksZoneLabel,
				},

				numPods: 7,
				podResource: v1.ResourceRequirements{
					Limits: v1.ResourceList{
						constants.GpuResource: resource.MustParse("8"),
					},
				},
			},
			"sg-2": {
				parent: "sg-p",
				topologyConstraint: &v2alpha2.TopologyConstraint{
					Topology:              TopologyName,
					RequiredTopologyLevel: EksZoneLabel,
				},
				numPods: 7,
				podResource: v1.ResourceRequirements{
					Limits: v1.ResourceList{
						constants.GpuResource: resource.MustParse("8"),
					},
				},
			},
		}

		podGroup, subGroupPods := createPodgroupWithSubgroupsWithPods(ctx, testCtx, testCtx.Queues[0], subGroups)

		for _, sgPods := range subGroupPods {
			wait.ForPodsScheduled(ctx, testCtx.ControllerClient, podGroup.Namespace, sgPods.pods)
			assertPodsOnSameTopology(ctx, testCtx, sgPods.pods, testTopologyData.TopologyCrd, sgPods.topologyConstraint)
		}
	})

	/*
		Context("Require", func() {
			It("Single subgroup - schedule to single domain", func(ctx context.Context) {
				subGroups := map[string]*subGroupPods{
					"sg": {
						topologyConstraint: &v2alpha2.TopologyConstraint{
							Topology:              TopologyName,
							RequiredTopologyLevel: EksZoneLabel,
						},
						numPods: 7,
						podResource: v1.ResourceRequirements{
							Limits: v1.ResourceList{
								constants.GpuResource: resource.MustParse("8"),
							},
						},
					},
				}

				podGroup, subGroupPods := createPodgroupWithSubgroupsWithPods(ctx, testCtx, testCtx.Queues[0], subGroups)

				for _, sgPods := range subGroupPods {
					wait.ForPodsScheduled(ctx, testCtx.ControllerClient, podGroup.Namespace, sgPods.pods)
					assertPodsOnSameTopology(ctx, testCtx, sgPods.pods, testTopologyData.TopologyCrd, sgPods.topologyConstraint)
				}
			})

			It("Single subgroup - does not fit to single domain - do not schedule to multiple domains", func(ctx context.Context) {
				subGroups := map[string]*subGroupPods{
					"sg": {
						topologyConstraint: &v2alpha2.TopologyConstraint{
							Topology:              TopologyName,
							RequiredTopologyLevel: EksZoneLabel,
						},
						numPods: 14,
						podResource: v1.ResourceRequirements{
							Limits: v1.ResourceList{
								constants.GpuResource: resource.MustParse("8"),
							},
						},
					},
				}

				podGroup, subGroupPods := createPodgroupWithSubgroupsWithPods(ctx, testCtx, testCtx.Queues[0], subGroups)

				for _, sgPods := range subGroupPods {
					wait.ForAtLeastNPodsUnschedulable(ctx, testCtx.ControllerClient, podGroup.Namespace, sgPods.pods, 14)
				}
			})
		})

		Context("Prefer", func() {
			It("Single subgroup - schedule to single domain", func(ctx context.Context) {
				subGroups := map[string]*subGroupPods{
					"sg": {
						topologyConstraint: &v2alpha2.TopologyConstraint{
							Topology:               TopologyName,
							PreferredTopologyLevel: EksZoneLabel,
						},
						numPods: 7,
						podResource: v1.ResourceRequirements{
							Limits: v1.ResourceList{
								constants.GpuResource: resource.MustParse("8"),
							},
						},
					},
				}

				podGroup, subGroupPods := createPodgroupWithSubgroupsWithPods(ctx, testCtx, testCtx.Queues[0], subGroups)

				for _, sgPods := range subGroupPods {
					wait.ForPodsScheduled(ctx, testCtx.ControllerClient, podGroup.Namespace, sgPods.pods)
					assertPodsOnSameTopology(ctx, testCtx, sgPods.pods, testTopologyData.TopologyCrd, sgPods.topologyConstraint)
				}
			})

			It("Single subgroup - does not fit to single domain - schedule to multiple domains", func(ctx context.Context) {
				subGroups := map[string]*subGroupPods{
					"sg": {
						topologyConstraint: &v2alpha2.TopologyConstraint{
							Topology:               TopologyName,
							PreferredTopologyLevel: EksZoneLabel,
						},
						numPods: 14,
						podResource: v1.ResourceRequirements{
							Limits: v1.ResourceList{
								constants.GpuResource: resource.MustParse("8"),
							},
						},
					},
				}

				podGroup, subGroupPods := createPodgroupWithSubgroupsWithPods(ctx, testCtx, testCtx.Queues[0], subGroups)

				for _, sgPods := range subGroupPods {
					wait.ForPodsScheduled(ctx, testCtx.ControllerClient, podGroup.Namespace, sgPods.pods)
				}
			})
		})
	*/
}, Ordered)

func assertPodsOnSameTopology(ctx context.Context, testCtx *testcontext.TestContext, pods []*v1.Pod, topologyCrd *kueuev1alpha1.Topology, constraint *v2alpha2.TopologyConstraint) {
	if len(pods) == 0 {
		return
	}

	nodes := map[string]v1.Node{}
	for _, pod := range pods {
		clusterPod, err := testCtx.KubeClientset.CoreV1().Pods(pods[0].Namespace).Get(ctx, pod.Name, metav1.GetOptions{})
		Expect(err).NotTo(HaveOccurred())

		node, err := testCtx.KubeClientset.CoreV1().Nodes().Get(ctx, clusterPod.Spec.NodeName, metav1.GetOptions{})
		Expect(err).NotTo(HaveOccurred())

		if _, found := nodes[node.Name]; !found {
			nodes[node.Name] = *node
		}
	}

	if constraint == nil || (constraint.RequiredTopologyLevel == "" && constraint.PreferredTopologyLevel == "") {
		return
	}
	assertionLevel := constraint.RequiredTopologyLevel
	if constraint.PreferredTopologyLevel != "" {
		assertionLevel = constraint.PreferredTopologyLevel
	}

	for _, topologyLevel := range topologyCrd.Spec.Levels {
		var domains []string
		for _, node := range nodes {
			domains = append(domains, node.Labels[topologyLevel.NodeLabel])
		}
		for _, domain := range domains {
			Expect(domain).To(Equal(domains[0]))
		}
		if topologyLevel.NodeLabel == assertionLevel {
			break
		}
	}
}

func createPodgroupWithSubgroupsWithPods(ctx context.Context, testCtx *testcontext.TestContext, testQueue *v2.Queue, subGroups map[string]*subGroupPods) (*v2alpha2.PodGroup, []*subGroupPods) {
	pgName := utils.GenerateRandomK8sName(10)
	namespace := queue.GetConnectedNamespaceToQueue(testQueue)

	for name, subGroup := range subGroups {
		for j := 0; j < subGroup.numPods; j++ {
			pod := createPodOfSubGroup(ctx, testCtx.KubeClientset, testQueue, pgName, name, subGroup.podResource)
			subGroupToAddPod := subGroup
			for subGroupToAddPod != nil {
				subGroupToAddPod.pods = append(subGroupToAddPod.pods, pod)
				if subGroupToAddPod.parent == "" {
					break
				}
				subGroupToAddPod = subGroups[subGroupToAddPod.parent]
			}
		}

	}

	var resSubGroups []*subGroupPods
	for _, subGroup := range subGroups {
		resSubGroups = append(resSubGroups, subGroup)
	}

	podGroup := pod_group.Create(namespace, pgName, testQueue.Name)
	podGroup.Spec.SubGroups = []v2alpha2.SubGroup{}
	for name, subGroup := range subGroups {
		sg := v2alpha2.SubGroup{
			Name:               name,
			MinMember:          int32(subGroup.numPods),
			TopologyConstraint: subGroup.topologyConstraint,
		}
		if subGroup.parent != "" {
			sg.Parent = &subGroup.parent
		}
		podGroup.Spec.SubGroups = append(podGroup.Spec.SubGroups, sg)
	}
	podGroup, err := testCtx.KubeAiSchedClientset.SchedulingV2alpha2().PodGroups(namespace).Create(ctx, podGroup, metav1.CreateOptions{})
	Expect(err).To(Succeed())
	return podGroup, resSubGroups
}

func createPodOfSubGroup(ctx context.Context, client *kubernetes.Clientset, queue *v2.Queue,
	podGroupName, subGroupName string, requirements v1.ResourceRequirements) *v1.Pod {
	pod := rd.CreatePodObject(queue, requirements)
	pod.Annotations[pod_group.PodGroupNameAnnotation] = podGroupName
	pod.Labels[pod_group.PodGroupNameAnnotation] = podGroupName
	pod.Labels["kai.scheduler/subgroup-name"] = subGroupName
	pod, err := rd.CreatePod(ctx, client, pod)
	Expect(err).To(Succeed())
	return pod
}
