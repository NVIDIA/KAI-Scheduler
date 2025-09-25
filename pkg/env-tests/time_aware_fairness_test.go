// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package env_tests

import (
	"context"
	"fmt"
	"os"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/xyproto/randomstring"
	"sigs.k8s.io/controller-runtime/pkg/client"

	corev1 "k8s.io/api/core/v1"
	resourcev1beta1 "k8s.io/api/resource/v1beta1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	kaiv1alpha2 "github.com/NVIDIA/KAI-scheduler/pkg/apis/scheduling/v1alpha2"
	schedulingv2 "github.com/NVIDIA/KAI-scheduler/pkg/apis/scheduling/v2"
	schedulingv2alpha2 "github.com/NVIDIA/KAI-scheduler/pkg/apis/scheduling/v2alpha2"
	"github.com/NVIDIA/KAI-scheduler/pkg/common/constants"
	"github.com/NVIDIA/KAI-scheduler/pkg/env-tests/binder"
	"github.com/NVIDIA/KAI-scheduler/pkg/env-tests/podgroupcontroller"
	"github.com/NVIDIA/KAI-scheduler/pkg/env-tests/queuecontroller"
	"github.com/NVIDIA/KAI-scheduler/pkg/env-tests/scheduler"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/actions"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/common_info"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/queue_info"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/cache/usagedb/api"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/cache/usagedb/fake"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/conf_util"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/plugins"
)

var _ = Describe("Time Aware Fairness", Ordered, func() {
	// Define a timeout for eventually assertions
	const interval = time.Millisecond * 10
	const defaultTimeout = interval * 200

	var (
		testNamespace  *corev1.Namespace
		testDepartment *schedulingv2.Queue
		testQueue      *schedulingv2.Queue
		testNode       *corev1.Node

		usageClient   *fake.FakeWithHistoryClient
		backgroundCtx context.Context
		cancel        context.CancelFunc
		stopCh        chan struct{}
	)

	BeforeEach(func(ctx context.Context) {
		// Create a test namespace
		testNamespace = &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test-" + randomstring.HumanFriendlyEnglishString(10),
			},
		}
		Expect(ctrlClient.Create(ctx, testNamespace)).To(Succeed())

		testDepartment = CreateQueueObject("test-department", "")
		Expect(ctrlClient.Create(ctx, testDepartment)).To(Succeed(), "Failed to create test department")

		testQueue = CreateQueueObject("test-queue", testDepartment.Name)
		Expect(ctrlClient.Create(ctx, testQueue)).To(Succeed(), "Failed to create test queue")

		testNode = CreateNodeObject(ctx, ctrlClient, DefaultNodeConfig("test-node"))
		Expect(ctrlClient.Create(ctx, testNode)).To(Succeed(), "Failed to create test node")

		backgroundCtx, cancel = context.WithCancel(context.Background())

		err := queuecontroller.RunQueueController(cfg, backgroundCtx)
		Expect(err).NotTo(HaveOccurred(), "Failed to run queuecontroller")

		actions.InitDefaultActions()
		plugins.InitDefaultPlugins()

		schedulerConf, err := conf_util.GetDefaultSchedulerConf()
		Expect(err).NotTo(HaveOccurred(), "Failed to get default scheduler config")

		schedulerConf.UsageDBConfig = &api.UsageDBConfig{
			ClientType:       "fake-with-history",
			ConnectionString: "fake-connection",
			UsageParams: &api.UsageParams{
				WindowSize:    &[]time.Duration{time.Second * 5}[0],
				FetchInterval: &[]time.Duration{time.Millisecond}[0],
			},
		}

		stopCh = make(chan struct{})
		err = scheduler.RunScheduler(cfg, schedulerConf, stopCh)
		Expect(err).NotTo(HaveOccurred(), "Failed to run scheduler")

		fakeClient, err := fake.NewFakeWithHistoryClient("fake-connection", nil)
		Expect(err).NotTo(HaveOccurred(), "Failed to create fake usage client")
		usageClient = fakeClient.(*fake.FakeWithHistoryClient)

		err = podgroupcontroller.RunPodGroupController(cfg, backgroundCtx)
		Expect(err).NotTo(HaveOccurred(), "Failed to run podgroupcontroller")

		err = binder.RunBinder(cfg, backgroundCtx)
		Expect(err).NotTo(HaveOccurred(), "Failed to run binder")

		cfg.QPS = -1
		cfg.Burst = -1
	})

	AfterEach(func(ctx context.Context) {
		Expect(ctrlClient.Delete(ctx, testDepartment)).To(Succeed(), "Failed to delete test department")
		Expect(ctrlClient.Delete(ctx, testQueue)).To(Succeed(), "Failed to delete test queue")
		Expect(ctrlClient.Delete(ctx, testNode)).To(Succeed(), "Failed to delete test node")

		err := WaitForObjectDeletion(ctx, ctrlClient, testDepartment, defaultTimeout, interval)
		Expect(err).NotTo(HaveOccurred(), "Failed to wait for test department to be deleted")

		err = WaitForObjectDeletion(ctx, ctrlClient, testQueue, defaultTimeout, interval)
		Expect(err).NotTo(HaveOccurred(), "Failed to wait for test queue to be deleted")

		err = WaitForObjectDeletion(ctx, ctrlClient, testNode, defaultTimeout, interval)
		Expect(err).NotTo(HaveOccurred(), "Failed to wait for test node to be deleted")

		cancel()
		close(stopCh)
	})

	Context("2 queues test", func() {
		var (
			testQueue1 *schedulingv2.Queue
			testQueue2 *schedulingv2.Queue
		)

		BeforeAll(func(ctx context.Context) {
			testQueue1 = CreateQueueObject("test-queue1", testDepartment.Name)
			Expect(ctrlClient.Create(ctx, testQueue1)).To(Succeed(), "Failed to create test queue1")

			testQueue2 = CreateQueueObject("test-queue2", testDepartment.Name)
			Expect(ctrlClient.Create(ctx, testQueue2)).To(Succeed(), "Failed to create test queue2")
		})

		AfterAll(func(ctx context.Context) {
			Expect(ctrlClient.Delete(ctx, testQueue1)).To(Succeed(), "Failed to delete test queue1")
			Expect(ctrlClient.Delete(ctx, testQueue2)).To(Succeed(), "Failed to delete test queue2")
		})

		BeforeEach(func(ctx context.Context) {
			usageClient.ResetClient()
		})

		AfterEach(func(ctx context.Context) {
			err := DeleteAllInNamespace(ctx, ctrlClient, testNamespace.Name,
				&corev1.Pod{},
				&schedulingv2alpha2.PodGroup{},
				&resourcev1beta1.ResourceClaim{},
				&kaiv1alpha2.BindRequest{},
			)
			Expect(err).NotTo(HaveOccurred(), "Failed to delete test resources")

			err = WaitForNoObjectsInNamespace(ctx, ctrlClient, testNamespace.Name, defaultTimeout, interval,
				&corev1.PodList{},
				&schedulingv2alpha2.PodGroupList{},
				&resourcev1beta1.ResourceClaimList{},
				&kaiv1alpha2.BindRequestList{},
			)
			Expect(err).NotTo(HaveOccurred(), "Failed to wait for test resources to be deleted")
		})

		It("Should oscillate allocation", func(ctx context.Context) {
			for range 100 {
				queueJob(ctx, ctrlClient, testNamespace.Name, testQueue1, 4)
				queueJob(ctx, ctrlClient, testNamespace.Name, testQueue2, 4)
			}

			for range 100 {
				time.Sleep(interval * 10)
				usageClient.AppendQueuedAllocation(getAllocations(ctx, ctrlClient), getClusterResources(ctx, ctrlClient, true))
			}

			allocationHistory := usageClient.GetAllocationHistory()
			csv := allocationHistory.ToTsv()
			// write csv to file
			err := os.WriteFile("allocation_history.tsv", []byte(csv), 0644)
			Expect(err).NotTo(HaveOccurred(), "Failed to write allocation history to file")
			fmt.Println(csv)
		})
	})
})

var totalInCluster map[corev1.ResourceName]float64

func getClusterResources(ctx context.Context, ctrlClient client.Client, allowCache bool) map[corev1.ResourceName]float64 {
	if allowCache && totalInCluster != nil {
		return totalInCluster
	}

	totalInCluster = make(map[corev1.ResourceName]float64)

	var nodes corev1.NodeList
	Expect(ctrlClient.List(ctx, &nodes)).To(Succeed(), "Failed to list nodes")

	for _, node := range nodes.Items {
		for resource, allocation := range node.Status.Allocatable {
			totalInCluster[resource] += float64(allocation.Value())
		}
	}
	return totalInCluster
}

func getAllocations(ctx context.Context, ctrlClient client.Client) map[common_info.QueueID]queue_info.QueueUsage {
	allocations := make(map[common_info.QueueID]queue_info.QueueUsage)
	var queues schedulingv2.QueueList
	Expect(ctrlClient.List(ctx, &queues)).To(Succeed(), "Failed to list queues")

	for _, queue := range queues.Items {
		if allocations[common_info.QueueID(queue.Name)] == nil {
			allocations[common_info.QueueID(queue.Name)] = make(queue_info.QueueUsage)
		}
		for resource, allocation := range queue.Status.Allocated {
			allocations[common_info.QueueID(queue.Name)][resource] = float64(allocation.Value())
		}
	}
	return allocations
}

func queueJob(ctx context.Context, ctrlClient client.Client, namespace string, queue *schedulingv2.Queue, gpus int) {
	name := randomstring.HumanFriendlyEnglishString(10)
	testPod := CreatePodObject(namespace, name, corev1.ResourceRequirements{
		Limits: corev1.ResourceList{
			constants.GpuResource: resource.MustParse(fmt.Sprintf("%d", gpus)),
		},
	})
	Expect(ctrlClient.Create(ctx, testPod)).To(Succeed(), "Failed to create test pod")

	Expect(GroupPods(ctx, ctrlClient, podGroupConfig{
		queueName:    queue.Name,
		podgroupName: name,
		minMember:    1,
	}, []*corev1.Pod{testPod})).To(Succeed(), "Failed to group pod")
}
