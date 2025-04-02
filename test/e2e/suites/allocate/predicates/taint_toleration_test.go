/*
Copyright 2025 NVIDIA CORPORATION
SPDX-License-Identifier: Apache-2.0
*/
package predicates

import (
	"context"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"

	v2 "github.com/NVIDIA/KAI-scheduler/pkg/apis/scheduling/v2"
	"github.com/NVIDIA/KAI-scheduler/test/e2e/modules/constant"
	testcontext "github.com/NVIDIA/KAI-scheduler/test/e2e/modules/context"
	"github.com/NVIDIA/KAI-scheduler/test/e2e/modules/resources/capacity"
	"github.com/NVIDIA/KAI-scheduler/test/e2e/modules/resources/rd"
	"github.com/NVIDIA/KAI-scheduler/test/e2e/modules/resources/rd/queue"
	"github.com/NVIDIA/KAI-scheduler/test/e2e/modules/utils"
	"github.com/NVIDIA/KAI-scheduler/test/e2e/modules/wait"
)

const (
	taintKey   = "testKey"
	taintValue = "testValue"
)

var _ = Describe("Taint and toleration scheduling", Ordered, func() {
	var (
		testCtx  *testcontext.TestContext
		nodeName string
	)

	BeforeAll(func(ctx context.Context) {
		testCtx = testcontext.GetConnectivity(ctx, Default)
		capacity.SkipIfInsufficientClusterResources(testCtx.KubeClientset,
			&capacity.ResourceList{
				PodCount: 1,
			},
		)

		node, err := getNodeWithoutTaints(ctx, testCtx.KubeClientset)
		Expect(err).To(Succeed())
		nodeName = node.Name
		err = taintNode(ctx, testCtx.KubeClientset, nodeName)
		Expect(err).To(Succeed())

		parentQueue := queue.CreateQueueObject(utils.GenerateRandomK8sName(10), "")
		childQueue := queue.CreateQueueObject(utils.GenerateRandomK8sName(10), parentQueue.Name)
		testCtx.InitQueues([]*v2.Queue{childQueue, parentQueue})
	})

	AfterAll(func(ctx context.Context) {
		err := untaintNode(ctx, testCtx.KubeClientset, nodeName)
		Expect(err).To(Succeed())
		testCtx.ClusterCleanup(ctx)
	})

	AfterEach(func(ctx context.Context) {
		testCtx.TestContextCleanup(ctx)
	})

	It("Not scheduling pod without toleration", func(ctx context.Context) {
		pod := getPodWithRequiredNodeAffinity(testCtx.Queues[0], nodeName)

		_, err := rd.CreatePod(ctx, testCtx.KubeClientset, pod)
		Expect(err).NotTo(HaveOccurred())

		wait.ForPodUnschedulable(ctx, testCtx.ControllerClient, pod)
	})

	It("Scheduling pod with toleration", func(ctx context.Context) {
		pod := getPodWithRequiredNodeAffinity(testCtx.Queues[0], nodeName)
		pod.Spec.Tolerations = []v1.Toleration{
			{
				Key:      taintKey,
				Operator: v1.TolerationOpEqual,
				Value:    taintValue,
				Effect:   v1.TaintEffectNoSchedule,
			},
		}

		_, err := rd.CreatePod(ctx, testCtx.KubeClientset, pod)
		Expect(err).NotTo(HaveOccurred())

		wait.ForPodScheduled(ctx, testCtx.ControllerClient, pod)
	})
})

func getPodWithRequiredNodeAffinity(queue *v2.Queue, node string) *v1.Pod {
	pod := rd.CreatePodObject(queue, v1.ResourceRequirements{})
	pod.Spec.Affinity = &v1.Affinity{
		NodeAffinity: &v1.NodeAffinity{
			RequiredDuringSchedulingIgnoredDuringExecution: &v1.NodeSelector{
				NodeSelectorTerms: []v1.NodeSelectorTerm{
					{
						MatchExpressions: []v1.NodeSelectorRequirement{
							{
								Key:      constant.NodeNamePodLabelName,
								Operator: v1.NodeSelectorOpIn,
								Values:   []string{node},
							},
						},
					},
				},
			},
		},
	}
	return pod
}

func getNodeWithoutTaints(ctx context.Context, client kubernetes.Interface) (*v1.Node, error) {
	nodes, err := client.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	for _, node := range nodes.Items {
		if len(node.Spec.Taints) == 0 {
			return &node, nil
		}
	}
	return nil, fmt.Errorf("could not find node without taint")
}

func taintNode(ctx context.Context, client kubernetes.Interface, nodeName string) error {
	patch := fmt.Sprintf(`{"spec": {"taints": [{"key": "%s", "value": "%s", "effect": "%s"}]}}`,
		taintKey, taintValue, v1.TaintEffectNoSchedule)
	patchBytes := []byte(patch)
	_, err := client.CoreV1().Nodes().Patch(ctx, nodeName, types.MergePatchType, patchBytes, metav1.PatchOptions{})
	return err
}

func untaintNode(ctx context.Context, client kubernetes.Interface, nodeName string) error {
	patch := `{"spec": {"taints": []}}`
	patchBytes := []byte(patch)
	_, err := client.CoreV1().Nodes().Patch(ctx, nodeName, types.MergePatchType, patchBytes, metav1.PatchOptions{})
	return err
}
