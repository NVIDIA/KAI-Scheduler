// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package env_tests

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/go-gota/gota/dataframe"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/xyproto/randomstring"
	"sigs.k8s.io/controller-runtime/pkg/client"

	corev1 "k8s.io/api/core/v1"
	resourceapi "k8s.io/api/resource/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
	"k8s.io/utils/ptr"

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

const simulationCycleInterval = time.Millisecond * 10
const defaultTimeout = time.Second * 10

type TestQueue struct {
	Name         string
	Parent       string   // if empty, the queue is a department
	Priority     *int     // default is 100
	DeservedGPUs *float64 // default is 0
	Weight       *float64 // default is 1
}

type TestJobs struct {
	GPUs    int
	NumPods int
	NumJobs int
}

type TestNodes struct {
	GPUs  int
	Count int
}

type TimeAwareSimulation struct {
	Queues []TestQueue
	Jobs   map[string]TestJobs // key is the queue name
	Nodes  []TestNodes
	Cycles *int //default is 100
}

func setupControllers(backgroundCtx context.Context, cfg *rest.Config) (chan struct{}, context.CancelFunc, *fake.FakeUsageDBClient, error) {
	ctx, cancel := context.WithCancel(backgroundCtx)

	err := queuecontroller.RunQueueController(cfg, ctx)
	if err != nil {
		return nil, cancel, nil, fmt.Errorf("failed to run queuecontroller: %w", err)
	}

	actions.InitDefaultActions()
	plugins.InitDefaultPlugins()

	schedulerConf, err := conf_util.GetDefaultSchedulerConf()
	if err != nil {
		return nil, cancel, nil, fmt.Errorf("failed to get default scheduler config: %w", err)
	}

	schedulerConf.UsageDBConfig = &api.UsageDBConfig{
		ClientType:       "fake-with-history",
		ConnectionString: "fake-connection",
		UsageParams: &api.UsageParams{
			WindowSize:    &[]time.Duration{time.Second * 6}[0],
			FetchInterval: &[]time.Duration{time.Millisecond}[0],
		},
	}

	stopCh := make(chan struct{})
	err = scheduler.RunScheduler(cfg, schedulerConf, stopCh)
	if err != nil {
		return nil, cancel, nil, fmt.Errorf("failed to run scheduler: %w", err)
	}

	err = podgroupcontroller.RunPodGroupController(cfg, ctx)
	if err != nil {
		return nil, cancel, nil, fmt.Errorf("failed to run podgroupcontroller: %w", err)
	}

	err = binder.RunBinder(cfg, ctx)
	if err != nil {
		return nil, cancel, nil, fmt.Errorf("failed to run binder: %w", err)
	}

	fakeClient, err := fake.NewFakeWithHistoryClient("fake-connection", nil)
	if err != nil {
		return nil, cancel, nil, fmt.Errorf("failed to create fake usage client: %w", err)
	}
	usageClient := fakeClient.(*fake.FakeUsageDBClient)

	cfg.QPS = -1
	cfg.Burst = -1

	return stopCh, cancel, usageClient, nil
}

func RunSimulation(ctx context.Context, simulation TimeAwareSimulation) (fake.AllocationHistory, error, error) {
	simulationName := randomstring.HumanFriendlyEnglishString(10)

	stopCh, cancel, usageClient, err := setupControllers(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to setup controllers: %w", err), nil
	}
	defer close(stopCh)
	defer cancel()

	testNamespace := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "simulation-test",
			Labels: map[string]string{
				"simulation": simulationName,
			},
		},
	}
	if err := ctrlClient.Create(ctx, testNamespace); err != nil {
		return nil, fmt.Errorf("failed to create test namespace: %w", err), nil
	}

	var cleanupErrors []error
	defer func() {
		cleanupErrors = append(cleanupErrors, cleanupSimulation(ctx, ctrlClient, testNamespace, simulationName)...)
	}()

	var nodes []*corev1.Node
	for _, node := range simulation.Nodes {
		nodeObject := CreateNodeObject(ctx, ctrlClient, DefaultNodeConfig(fmt.Sprintf("test-node-%d", node.GPUs)))
		nodeObject.ObjectMeta.Labels = map[string]string{
			"simulation": simulationName,
		}
		if err := ctrlClient.Create(ctx, nodeObject); err != nil {
			return nil, fmt.Errorf("failed to create test node: %w", err), errors.Join(cleanupErrors...)
		}
		nodes = append(nodes, nodeObject)
	}

	var queues []*schedulingv2.Queue
	for _, queue := range simulation.Queues {
		queueObject := CreateQueueObject(queue.Name, queue.Parent)

		queueObject.ObjectMeta.Labels = map[string]string{
			"simulation": simulationName,
		}

		queueObject.Spec.Priority = ptr.To(100)
		if queue.Priority != nil {
			queueObject.Spec.Priority = queue.Priority
		}

		queueObject.Spec.Resources.GPU.Quota = 0
		if queue.DeservedGPUs != nil {
			queueObject.Spec.Resources.GPU.Quota = *queue.DeservedGPUs
		}

		queueObject.Spec.Resources.GPU.OverQuotaWeight = 1
		if queue.Weight != nil {
			queueObject.Spec.Resources.GPU.OverQuotaWeight = *queue.Weight
		}

		err := ctrlClient.Create(ctx, queueObject)
		if err != nil {
			return nil, fmt.Errorf("failed to create test queue: %w", err), errors.Join(cleanupErrors...)
		}
		queues = append(queues, queueObject)
	}

	for queueName, job := range simulation.Jobs {
		for range job.NumJobs {
			queueJob(ctx, ctrlClient, testNamespace.Name, queueName, job.NumPods, job.GPUs)
		}
	}

	if simulation.Cycles == nil {
		simulation.Cycles = ptr.To(100)
	}
	for range *simulation.Cycles {
		time.Sleep(simulationCycleInterval * 10)
		usageClient.AppendQueuedAllocation(getAllocations(ctx, ctrlClient), getClusterResources(ctx, ctrlClient, true))
	}

	allocationHistory := usageClient.GetAllocationHistory()

	return allocationHistory, err, errors.Join(cleanupErrors...)
}

func cleanupSimulation(ctx context.Context, ctrlClient client.Client, testNamespace *corev1.Namespace, simulationName string) []error {
	var cleanupErrors []error

	if err := ctrlClient.DeleteAllOf(context.Background(), &corev1.Node{},
		client.MatchingLabels{"simulation": simulationName},
		client.GracePeriodSeconds(0),
	); err != nil {
		cleanupErrors = append(cleanupErrors, fmt.Errorf("failed to delete test nodes during cleanup: %w", err))
	}

	if err := ctrlClient.DeleteAllOf(context.Background(), &schedulingv2.Queue{},
		client.MatchingLabels{"simulation": simulationName},
		client.GracePeriodSeconds(0),
	); err != nil {
		cleanupErrors = append(cleanupErrors, fmt.Errorf("failed to delete simulation queues during cleanup: %w", err))
	}

	if err := DeleteAllInNamespace(ctx, ctrlClient, testNamespace.Name,
		&corev1.Pod{},
		&schedulingv2alpha2.PodGroup{},
	); err != nil {
		cleanupErrors = append(cleanupErrors, fmt.Errorf("failed to delete simulation pods and podgroups during cleanup: %w", err))
	}

	if err := WaitForNoObjectsInNamespace(ctx, ctrlClient, testNamespace.Name, defaultTimeout, simulationCycleInterval,
		&corev1.PodList{},
		&schedulingv2alpha2.PodGroupList{},
		&resourceapi.ResourceClaimList{},
		&kaiv1alpha2.BindRequestList{},
	); err != nil {
		cleanupErrors = append(cleanupErrors, fmt.Errorf("failed to wait for simulation pods and podgroups to be deleted during cleanup: %w", err))
	}

	if err := ctrlClient.Delete(context.Background(), testNamespace); err != nil {
		cleanupErrors = append(cleanupErrors, fmt.Errorf("failed to delete test namespace during cleanup: %w", err))
	}

	if err := WaitForNamespaceDeletion(ctx, ctrlClient, testNamespace.Name, defaultTimeout, simulationCycleInterval); err != nil {
		cleanupErrors = append(cleanupErrors, fmt.Errorf("failed to wait for test namespace to be deleted during cleanup: %w", err))
	}

	return cleanupErrors
}

var _ = Describe("Time Aware Fairness", Ordered, func() {
	BeforeAll(func() {
		runtests, err := strconv.ParseBool(os.Getenv("RUN_TIME_AWARE_TESTS"))
		if err != nil {
			Skip(fmt.Sprintf("Failed to parse RUN_TIME_AWARE_TESTS environment variable: %v", err))
		}
		if !runtests {
			Skip("Skipping time aware fairness tests (RUN_TIME_AWARE_TESTS is not set to true)")
		}
	})

	It("Should run simulation", func(ctx context.Context) {
		allocationHistory, err, cleanupError := RunSimulation(ctx, TimeAwareSimulation{
			Queues: []TestQueue{
				{Name: "test-department", Parent: ""},
				{Name: "test-queue1", Parent: "test-department"},
				{Name: "test-queue2", Parent: "test-department"},
			},
			Jobs: map[string]TestJobs{
				"test-queue1": {NumPods: 1, NumJobs: 100, GPUs: 4},
				"test-queue2": {NumPods: 1, NumJobs: 100, GPUs: 4},
			},
			Nodes: []TestNodes{
				{GPUs: 4, Count: 1},
			},
		})
		Expect(err).NotTo(HaveOccurred(), "Failed to run simulation")
		Expect(cleanupError).NotTo(HaveOccurred(), "Failed to cleanup simulation")

		df := allocationHistory.ToDataFrame()

		// Sum allocations for each queue
		queueSums := df.GroupBy("QueueID").
			Aggregation([]dataframe.AggregationType{dataframe.Aggregation_SUM}, []string{"Allocation"})

		// Convert queueSums dataframe to map from queueID to sum
		queueSumMap := make(map[string]float64)
		for i := 0; i < queueSums.Nrow(); i++ {
			queueID := queueSums.Elem(i, 1).String()
			allocation := queueSums.Elem(i, 0).Float()
			queueSumMap[queueID] = allocation
		}

		// Assert that test-queue1 and test-queue2 allocations sum to approximately the department allocation
		// Small difference could happen due to queue controller non-atomic updates
		Expect(queueSumMap["test-queue1"]+queueSumMap["test-queue2"]).To(
			BeNumerically("~", queueSumMap["test-department"], queueSumMap["test-department"]*0.1),
			"Sum of queue1 and queue2 should equal department allocation")

		// Assert that test-queue1 and test-queue2 have approximately equal allocations (within 10%)
		Expect(queueSumMap["test-queue1"]).To(
			BeNumerically("~", queueSumMap["test-queue2"], queueSumMap["test-queue2"]*0.1),
			"Queue1 and Queue2 should have approximately equal allocations")
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

func queueJob(ctx context.Context, ctrlClient client.Client, namespace, queueName string, pods, gpus int) error {
	pgName := randomstring.HumanFriendlyEnglishString(10)

	var testPods []*corev1.Pod
	for range pods {
		name := randomstring.HumanFriendlyEnglishString(10)
		testPod := CreatePodObject(namespace, name, corev1.ResourceRequirements{
			Limits: corev1.ResourceList{
				constants.GpuResource: resource.MustParse(fmt.Sprintf("%d", gpus)),
			},
		})
		err := ctrlClient.Create(ctx, testPod)
		if err != nil {
			return fmt.Errorf("failed to create test pod: %w", err)
		}

		testPods = append(testPods, testPod)
	}

	err := GroupPods(ctx, ctrlClient, podGroupConfig{
		queueName:    queueName,
		podgroupName: pgName,
		minMember:    1,
	}, testPods)

	if err != nil {
		return fmt.Errorf("failed to group pod: %w", err)
	}

	return nil
}
