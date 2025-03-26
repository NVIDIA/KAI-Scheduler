// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package cache

import (
	"context"
	"fmt"
	"sync"
	"time"

	"go.uber.org/multierr"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/rand"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	listv1 "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/record"
	k8sframework "k8s.io/kubernetes/pkg/scheduler/framework"

	kubeaischedulerver "github.com/NVIDIA/KAI-scheduler/pkg/apis/client/clientset/versioned"
	kubeaischedulerschema "github.com/NVIDIA/KAI-scheduler/pkg/apis/client/clientset/versioned/scheme"
	kubeaischedulerinfo "github.com/NVIDIA/KAI-scheduler/pkg/apis/client/informers/externalversions"
	enginelisters "github.com/NVIDIA/KAI-scheduler/pkg/apis/client/listers/scheduling/v2alpha2"
	schedulingv1alpha2 "github.com/NVIDIA/KAI-scheduler/pkg/apis/scheduling/v1alpha2"
	enginev2alpha2 "github.com/NVIDIA/KAI-scheduler/pkg/apis/scheduling/v2alpha2"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/bindrequest_info"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/eviction_info"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/pod_info"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/podgroup_info"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/cache/cluster_info"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/cache/evictor"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/cache/status_updater"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/conf"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/constants/status"
	k8splugins "github.com/NVIDIA/KAI-scheduler/pkg/scheduler/k8s_internal/plugins"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/log"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/metrics"
)

func init() {
	schemeBuilder := runtime.SchemeBuilder{
		v1.AddToScheme,
	}

	utilruntime.Must(schemeBuilder.AddToScheme(kubeaischedulerschema.Scheme))
}

// New returns a Cache implementation.
func New(schedulerCacheParams *SchedulerCacheParams) Cache {
	return newSchedulerCache(schedulerCacheParams)
}

type SchedulerCacheParams struct {
	SchedulerName               string
	NodePoolParams              *conf.SchedulingNodePoolParams
	RestrictNodeScheduling      bool
	KubeClient                  kubernetes.Interface
	KubeAISchedulerClient       kubeaischedulerver.Interface
	DetailedFitErrors           bool
	ScheduleCSIStorage          bool
	FullHierarchyFairness       bool
	NodeLevelScheduler          bool
	AllowConsolidatingReclaim   bool
	NumOfStatusRecordingWorkers int
}

type SchedulerCache struct {
	workersWaitGroup               sync.WaitGroup
	kubeClient                     kubernetes.Interface
	kubeAiSchedulerClient          kubeaischedulerver.Interface
	informerFactory                informers.SharedInformerFactory
	kubeAiSchedulerInformerFactory kubeaischedulerinfo.SharedInformerFactory
	podLister                      listv1.PodLister
	podGroupLister                 enginelisters.PodGroupLister
	clusterInfo                    *cluster_info.ClusterInfo

	schedulingNodePoolParams *conf.SchedulingNodePoolParams

	Evictor       evictor.Interface
	StatusUpdater status_updater.Interface

	detailedFitErrors      bool
	restrictNodeScheduling bool
	scheduleCSIStorage     bool
	fullHierarchyFairness  bool

	internalPlugins *k8splugins.K8sPlugins

	K8sClusterPodAffinityInfo
}

func newSchedulerCache(schedulerCacheParams *SchedulerCacheParams) *SchedulerCache {
	sc := &SchedulerCache{
		schedulingNodePoolParams: schedulerCacheParams.NodePoolParams,
		restrictNodeScheduling:   schedulerCacheParams.RestrictNodeScheduling,
		detailedFitErrors:        schedulerCacheParams.DetailedFitErrors,
		scheduleCSIStorage:       schedulerCacheParams.ScheduleCSIStorage,
		fullHierarchyFairness:    schedulerCacheParams.FullHierarchyFairness,
		kubeClient:               schedulerCacheParams.KubeClient,
		kubeAiSchedulerClient:    schedulerCacheParams.KubeAISchedulerClient,
	}

	schedulerName := schedulerCacheParams.SchedulerName

	// Prepare event clients.
	broadcaster := record.NewBroadcaster()
	// The new broadcaster objects uses watch.NewLongQueueBroadcaster(maxQueuedEvents, watch.DropIfChannelFull) under the hood.
	// This means that we need to be careful when writing events using the recorder.
	// If the broadcaster will have more then maxQueuedEvents waiting to be published, he will drop all incoming recording requests.
	broadcaster.StartRecordingToSink(&corev1.EventSinkImpl{Interface: sc.kubeClient.CoreV1().Events("")})
	recorder := broadcaster.NewRecorder(kubeaischedulerschema.Scheme, v1.EventSource{Component: schedulerName})

	sc.Evictor = evictor.New(sc.kubeClient)

	sc.StatusUpdater = status_updater.New(sc.kubeClient, sc.kubeAiSchedulerClient, recorder, schedulerCacheParams.NumOfStatusRecordingWorkers, sc.detailedFitErrors)

	sc.informerFactory = informers.NewSharedInformerFactory(sc.kubeClient, 0)
	sc.kubeAiSchedulerInformerFactory = kubeaischedulerinfo.NewSharedInformerFactory(sc.kubeAiSchedulerClient, 0)

	sc.internalPlugins = k8splugins.InitializeInternalPlugins(sc.kubeClient, sc.informerFactory, sc.SnapshotSharedLister())

	sc.podLister = sc.informerFactory.Core().V1().Pods().Lister()
	sc.podGroupLister = sc.kubeAiSchedulerInformerFactory.Scheduling().V2alpha2().PodGroups().Lister()

	clusterInfo, err := cluster_info.New(sc.informerFactory, sc.kubeAiSchedulerInformerFactory, sc.schedulingNodePoolParams,
		sc.restrictNodeScheduling, &sc.K8sClusterPodAffinityInfo, sc.scheduleCSIStorage, sc.fullHierarchyFairness, sc.StatusUpdater)

	if err != nil {
		log.InfraLogger.Errorf("Failed to create cluster info object: %v", err)
		return nil
	}
	sc.clusterInfo = clusterInfo

	return sc
}

func (sc *SchedulerCache) Snapshot() (*api.ClusterInfo, error) {
	sc.K8sClusterPodAffinityInfo = *NewK8sClusterPodAffinityInfo()
	snapshot, err := sc.clusterInfo.Snapshot()
	if err != nil {
		log.InfraLogger.Errorf("Error during snapshot: %v", err)
		return nil, err
	}

	if cleanErr := sc.cleanStaleBindRequest(snapshot.BindRequests, snapshot.BindRequestsForDeletedNodes); cleanErr != nil {
		log.InfraLogger.V(2).Warnf("Failed to clean stale bind requests: %v", cleanErr)
		err = multierr.Append(err, cleanErr)
	}

	return snapshot, err
}

func (sc *SchedulerCache) Run(stopCh <-chan struct{}) {
	sc.informerFactory.Start(stopCh)
	sc.kubeAiSchedulerInformerFactory.Start(stopCh)
	sc.StatusUpdater.Run(stopCh)
}

func (sc *SchedulerCache) WaitForCacheSync(stopCh <-chan struct{}) {
	sc.informerFactory.WaitForCacheSync(stopCh)
	sc.kubeAiSchedulerInformerFactory.WaitForCacheSync(stopCh)
}

func (sc *SchedulerCache) Evict(evictedPod *v1.Pod, evictedPodGroup *podgroup_info.PodGroupInfo,
	evictionMetadata eviction_info.EvictionMetadata, message string) error {
	pod, err := sc.podLister.Pods(evictedPod.Namespace).Get(evictedPod.Name)
	if err != nil {
		return err
	}

	podGroup, err := sc.podGroupLister.PodGroups(evictedPodGroup.Namespace).Get(
		evictedPodGroup.Name)
	if err != nil {
		return err
	}

	if isTerminated(pod.Status.Phase) {
		return fmt.Errorf("received an eviction attempt for a terminated task: <%v/%v>", pod.Namespace, pod.Name)
	}

	sc.evict(pod, podGroup, evictionMetadata, message)
	return nil
}

func (sc *SchedulerCache) evict(evictedPod *v1.Pod, evictedPodGroup *enginev2alpha2.PodGroup, evictionMetadata eviction_info.EvictionMetadata, message string) {
	sc.workersWaitGroup.Add(1)
	go func() {
		defer sc.workersWaitGroup.Done()
		if len(message) > 0 {
			sc.StatusUpdater.Evicted(evictedPodGroup, evictionMetadata, message)
		}

		log.InfraLogger.V(6).Infof("Evicting pod %v/%v, reason: %v, message: %v",
			evictedPod.Namespace, evictedPod.Name, status.Preempted, message)
		err := sc.Evictor.Evict(evictedPod)
		if err != nil {
			log.InfraLogger.Errorf("Failed to evict pod: %v/%v, error: %v", evictedPod.Namespace, evictedPod.Name, err)
		}
	}()
}

func (sc *SchedulerCache) WaitForWorkers(stopCh <-chan struct{}) {
	done := make(chan struct{})
	go func() {
		sc.workersWaitGroup.Wait()
		close(done)
	}()
	select {
	case <-stopCh:
	case <-done:
	}
}

// Bind binds task to the target host.
func (sc *SchedulerCache) Bind(taskInfo *pod_info.PodInfo, hostname string) error {
	startTime := time.Now()
	defer metrics.UpdateTaskBindDuration(startTime)

	err := sc.createBindRequest(taskInfo, hostname)
	return sc.StatusUpdater.Bound(taskInfo.Pod, hostname, err, sc.getNodPoolName())
}

// +kubebuilder:rbac:groups="scheduling.run.ai",resources=bindrequests,verbs=create;update;patch
// +kubebuilder:rbac:groups="",resources=pods/finalizers,verbs=create;delete;update;patch;get;list

func (sc *SchedulerCache) createBindRequest(podInfo *pod_info.PodInfo, nodeName string) error {
	bindRequest := &schedulingv1alpha2.BindRequest{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-%s", podInfo.Pod.Name, rand.String(10)),
			Namespace: podInfo.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: "v1",
					Kind:       "Pod",
					Name:       podInfo.Pod.Name,
					UID:        podInfo.Pod.UID,
				},
			},
			Labels: map[string]string{
				"pod-name":      podInfo.Pod.Name,
				"selected-node": nodeName,
			},
		},
		Spec: schedulingv1alpha2.BindRequestSpec{
			PodName:              podInfo.Name,
			SelectedNode:         nodeName,
			SelectedGPUGroups:    podInfo.GPUGroups,
			ReceivedResourceType: string(podInfo.ResourceReceivedType),
			ReceivedGPU: &schedulingv1alpha2.ReceivedGPU{
				Count:   int(podInfo.AcceptedResource.GetNumOfGpuDevices()),
				Portion: fmt.Sprintf("%.2f", podInfo.AcceptedResource.GpuFractionalPortion()),
			},
			ResourceClaimAllocations: podInfo.ResourceClaimInfo,
		},
	}

	bindRequest.Labels = labels.Merge(bindRequest.Labels, sc.schedulingNodePoolParams.GetLabels())

	_, err := sc.kubeAiSchedulerClient.SchedulingV1alpha2().BindRequests(
		podInfo.Namespace).Create(context.TODO(), bindRequest, metav1.CreateOptions{})
	return err
}

func (sc *SchedulerCache) getNodPoolName() string {
	if sc.schedulingNodePoolParams.NodePoolLabelValue != "" {
		return sc.schedulingNodePoolParams.NodePoolLabelValue
	}
	return "default"
}

func (sc *SchedulerCache) String() string {
	str := "Cache:\n"

	return str
}

// RecordJobStatusEvent records related events according to job status.
func (sc *SchedulerCache) RecordJobStatusEvent(job *podgroup_info.PodGroupInfo) error {
	return sc.StatusUpdater.RecordJobStatusEvent(job)
}

func (sc *SchedulerCache) TaskPipelined(task *pod_info.PodInfo, message string) {
	sc.StatusUpdater.Pipelined(task.Pod, message)
}

// +kubebuilder:rbac:groups="scheduling.run.ai",resources=bindrequests,verbs=delete

// Clean Stale BindRequest
func (sc *SchedulerCache) cleanStaleBindRequest(
	snapshotBindRequests bindrequest_info.BindRequestMap,
	snapshotBindRequestsForDeletedNodes []*bindrequest_info.BindRequestInfo,
) error {
	var err error

	deletionsCompleted := sync.WaitGroup{}
	errChan := make(chan error, len(snapshotBindRequestsForDeletedNodes)+len(snapshotBindRequests))

	deleteBindRequest := func(bri *bindrequest_info.BindRequestInfo) {
		if deleteError := sc.kubeAiSchedulerClient.SchedulingV1alpha2().BindRequests(
			bri.Namespace).Delete(context.Background(), bri.Name, metav1.DeleteOptions{}); deleteError != nil {
			errChan <- fmt.Errorf(
				"failed to delete stale bind request <%v/%v>: %v",
				bri.Namespace, bri.Name, deleteError)
		}
		deletionsCompleted.Done()
	}

	for _, bindRequest := range snapshotBindRequestsForDeletedNodes {
		deletionsCompleted.Add(1)
		go deleteBindRequest(bindRequest)
	}

	for _, bindRequest := range snapshotBindRequests {
		if bindRequest.IsFailed() {
			deletionsCompleted.Add(1)
			go deleteBindRequest(bindRequest)
		}
	}

	deletionsCompleted.Wait()
	close(errChan)
	for errFromChannel := range errChan {
		if errFromChannel != nil {
			err = multierr.Append(err, errFromChannel)
		}
	}

	return err
}

func isTerminated(phase v1.PodPhase) bool {
	return phase == v1.PodFailed || phase == v1.PodSucceeded
}

func (sc *SchedulerCache) KubeClient() kubernetes.Interface {
	return sc.kubeClient
}

func (sc *SchedulerCache) KubeInformerFactory() informers.SharedInformerFactory {
	return sc.informerFactory
}

func (sc *SchedulerCache) SnapshotSharedLister() k8sframework.NodeInfoLister {
	return &sc.K8sClusterPodAffinityInfo
}

func (sc *SchedulerCache) InternalK8sPlugins() *k8splugins.K8sPlugins {
	return sc.internalPlugins
}
