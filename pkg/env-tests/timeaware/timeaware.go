// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package timeaware

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/xyproto/randomstring"
	"sigs.k8s.io/controller-runtime/pkg/client"

	corev1 "k8s.io/api/core/v1"
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
	"github.com/NVIDIA/KAI-scheduler/pkg/env-tests/utils"
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

func (q *TestQueue) SetDefaults() {
	if q.Priority == nil {
		q.Priority = ptr.To(100)
	}
	if q.DeservedGPUs == nil {
		q.DeservedGPUs = ptr.To(0.0)
	}
	if q.Weight == nil {
		q.Weight = ptr.To(1.0)
	}
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
	Queues     []TestQueue
	Jobs       map[string]TestJobs // key is the queue name
	Nodes      []TestNodes
	Cycles     *int     // default is 100
	WindowSize *int     // default is 5
	KValue     *float64 // default is 1.0
}

func (s *TimeAwareSimulation) SetDefaults() {
	for i := range s.Queues {
		s.Queues[i].SetDefaults()
	}
	if s.Cycles == nil {
		s.Cycles = ptr.To(100)
	}
	if s.WindowSize == nil {
		s.WindowSize = ptr.To(5)
	}
	if s.KValue == nil {
		s.KValue = ptr.To(1.0)
	}
}

func setupControllers(backgroundCtx context.Context, cfg *rest.Config, windowSize *int, kValue *float64) (chan struct{}, context.CancelFunc, *fake.FakeUsageDBClient, error) {
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
			WindowSize:    &[]time.Duration{time.Second * time.Duration(*windowSize)}[0],
			FetchInterval: &[]time.Duration{time.Millisecond}[0],
		},
	}

	for i := range schedulerConf.Tiers[0].Plugins {
		if schedulerConf.Tiers[0].Plugins[i].Name != "proportion" {
			continue
		}
		if schedulerConf.Tiers[0].Plugins[i].Arguments == nil {
			schedulerConf.Tiers[0].Plugins[i].Arguments = map[string]string{}
		}
		schedulerConf.Tiers[0].Plugins[i].Arguments["kValue"] = fmt.Sprintf("%f", *kValue)
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

func RunSimulation(ctx context.Context, ctrlClient client.Client, cfg *rest.Config, simulation TimeAwareSimulation) (allocationHistory fake.AllocationHistory, err error, cleanupErr error) {
	simulationName := randomstring.HumanFriendlyEnglishString(10)
	simulation.SetDefaults()

	stopCh, cancel, usageClient, err := setupControllers(ctx, cfg, simulation.WindowSize, simulation.KValue)
	if err != nil {
		return nil, fmt.Errorf("failed to setup controllers: %w", err), nil
	}
	defer close(stopCh)
	defer cancel()

	testNamespace := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: fmt.Sprintf("simulation-test-%s", simulationName),
			Labels: map[string]string{
				"simulation": simulationName,
			},
		},
	}
	if err := ctrlClient.Create(ctx, testNamespace); err != nil {
		return nil, fmt.Errorf("failed to create test namespace: %w", err), nil
	}

	defer func() {
		cleanupErrors := cleanupSimulation(ctx, ctrlClient, testNamespace, simulationName)
		if len(cleanupErrors) > 0 {
			cleanupErr = errors.Join(cleanupErrors...)
		}
	}()

	var nodes []*corev1.Node
	for _, node := range simulation.Nodes {
		nodeObject := utils.CreateNodeObject(ctx, ctrlClient, utils.DefaultNodeConfig(fmt.Sprintf("test-node-%d", node.GPUs)))
		nodeObject.ObjectMeta.Labels = map[string]string{
			"simulation": simulationName,
		}
		if err := ctrlClient.Create(ctx, nodeObject); err != nil {
			return nil, fmt.Errorf("failed to create test node: %w", err), nil
		}
		nodes = append(nodes, nodeObject)
	}

	var queues []*schedulingv2.Queue
	for _, queue := range simulation.Queues {
		queueObject := utils.CreateQueueObject(queue.Name, queue.Parent)

		queueObject.ObjectMeta.Labels = map[string]string{
			"simulation": simulationName,
		}
		queueObject.Spec.Priority = queue.Priority
		queueObject.Spec.Resources = &schedulingv2.QueueResources{
			GPU: schedulingv2.QueueResource{
				Quota:           *queue.DeservedGPUs,
				OverQuotaWeight: *queue.Weight,
				Limit:           -1,
			},
		}

		err := ctrlClient.Create(ctx, queueObject)
		if err != nil {
			return nil, fmt.Errorf("failed to create test queue: %w", err), nil
		}
		queues = append(queues, queueObject)
	}

	for queueName, job := range simulation.Jobs {
		for range job.NumJobs {
			err := queueJob(ctx, ctrlClient, testNamespace.Name, queueName, job.NumPods, job.GPUs)
			if err != nil {
				return nil, fmt.Errorf("failed to queue job: %w", err), nil
			}
		}
	}

	for range *simulation.Cycles {
		time.Sleep(simulationCycleInterval)
		allocations, err := getAllocations(ctx, ctrlClient)
		if err != nil {
			return nil, fmt.Errorf("failed to get allocations: %w", err), nil
		}
		clusterResources, err := getClusterResources(ctx, ctrlClient, true)
		if err != nil {
			return nil, fmt.Errorf("failed to get cluster resources: %w", err), nil
		}
		usageClient.AppendQueueAllocation(allocations, clusterResources)
	}

	allocationHistory = usageClient.GetAllocationHistory()

	return allocationHistory, err, nil
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

	if err := utils.DeleteAllInNamespace(ctx, ctrlClient, testNamespace.Name,
		&corev1.Pod{},
		&schedulingv2alpha2.PodGroup{},
	); err != nil {
		cleanupErrors = append(cleanupErrors, fmt.Errorf("failed to delete simulation pods and podgroups during cleanup: %w", err))
	}

	if err := utils.WaitForNoObjectsInNamespace(ctx, ctrlClient, testNamespace.Name, defaultTimeout, simulationCycleInterval,
		&corev1.PodList{},
		&schedulingv2alpha2.PodGroupList{},
		&kaiv1alpha2.BindRequestList{},
	); err != nil {
		cleanupErrors = append(cleanupErrors, fmt.Errorf("failed to wait for simulation pods and podgroups to be deleted during cleanup: %w", err))
	}

	if err := ctrlClient.Delete(context.Background(), testNamespace); err != nil {
		cleanupErrors = append(cleanupErrors, fmt.Errorf("failed to delete test namespace during cleanup: %w", err))
	}

	return cleanupErrors
}

var totalInCluster map[corev1.ResourceName]float64

func getClusterResources(ctx context.Context, ctrlClient client.Client, allowCache bool) (map[corev1.ResourceName]float64, error) {
	if allowCache && totalInCluster != nil {
		return totalInCluster, nil
	}

	totalInCluster = make(map[corev1.ResourceName]float64)

	var nodes corev1.NodeList
	if err := ctrlClient.List(ctx, &nodes); err != nil {
		return nil, fmt.Errorf("failed to list nodes: %w", err)
	}

	for _, node := range nodes.Items {
		for resource, allocation := range node.Status.Allocatable {
			totalInCluster[resource] += float64(allocation.Value())
		}
	}
	return totalInCluster, nil
}

func getAllocations(ctx context.Context, ctrlClient client.Client) (map[common_info.QueueID]queue_info.QueueUsage, error) {
	allocations := make(map[common_info.QueueID]queue_info.QueueUsage)
	var queues schedulingv2.QueueList
	if err := ctrlClient.List(ctx, &queues); err != nil {
		return nil, fmt.Errorf("failed to list queues: %w", err)
	}

	for _, queue := range queues.Items {
		if allocations[common_info.QueueID(queue.Name)] == nil {
			allocations[common_info.QueueID(queue.Name)] = make(queue_info.QueueUsage)
		}
		for resource, allocation := range queue.Status.Allocated {
			allocations[common_info.QueueID(queue.Name)][resource] = float64(allocation.Value())
		}
	}
	return allocations, nil
}

func queueJob(ctx context.Context, ctrlClient client.Client, namespace, queueName string, pods, gpus int) error {
	pgName := randomstring.HumanFriendlyEnglishString(10)

	var testPods []*corev1.Pod
	for range pods {
		name := randomstring.HumanFriendlyEnglishString(10)
		testPod := utils.CreatePodObject(namespace, name, corev1.ResourceRequirements{
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

	err := utils.GroupPods(ctx, ctrlClient, utils.PodGroupConfig{
		QueueName:    queueName,
		PodgroupName: pgName,
		MinMember:    1,
	}, testPods)

	if err != nil {
		return fmt.Errorf("failed to group pod: %w", err)
	}

	return nil
}
