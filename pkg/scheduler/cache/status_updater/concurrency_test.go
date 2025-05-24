// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package status_updater

import (
	"context"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	fakecorev1 "k8s.io/client-go/kubernetes/typed/core/v1/fake"
	faketesting "k8s.io/client-go/testing"
	"k8s.io/client-go/tools/record"
	"k8s.io/utils/ptr"

	kubeaischedfake "github.com/NVIDIA/KAI-scheduler/pkg/apis/client/clientset/versioned/fake"
	fakeschedulingv2alpha2 "github.com/NVIDIA/KAI-scheduler/pkg/apis/client/clientset/versioned/typed/scheduling/v2alpha2/fake"
	schedulingv2alpha2 "github.com/NVIDIA/KAI-scheduler/pkg/apis/scheduling/v2alpha2"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/pod_status"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/test_utils/jobs_fake"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/test_utils/tasks_fake"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/utils"
)

func TestConcurrency(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Status Updater Concurrency Suite")
}

var _ = Describe("Status Updater Concurrency", func() {
	var (
		kubeClient        *fake.Clientset
		kubeAiSchedClient *kubeaischedfake.Clientset
		statusUpdater     *defaultStatusUpdater
	)
	BeforeEach(func() {
		kubeClient = fake.NewSimpleClientset()
		kubeAiSchedClient = kubeaischedfake.NewSimpleClientset()
		recorder := record.NewFakeRecorder(100)
		statusUpdater = New(kubeClient, kubeAiSchedClient, recorder, 4, false)
	})

	Context("Pod Groups Syncing", func() {
		var (
			stopCh             chan struct{}
			finishUpdatesChan  chan struct{}
			podGroupsOriginals []*schedulingv2alpha2.PodGroup
			podsOriginals      []*v1.Pod
			wgPodGroups        sync.WaitGroup
			wgPods             sync.WaitGroup
		)

		BeforeEach(func() {
			wgPodGroups = sync.WaitGroup{}
			wgPods = sync.WaitGroup{}
			finishUpdatesChan = make(chan struct{})
			// wait with pod groups update until signal is given.
			kubeAiSchedClient.SchedulingV2alpha2().(*fakeschedulingv2alpha2.FakeSchedulingV2alpha2).PrependReactor(
				"update", "podgroups", func(action faketesting.Action) (handled bool, ret runtime.Object, err error) {
					<-finishUpdatesChan
					wgPodGroups.Done()
					return false, nil, nil
				},
			)
			kubeClient.CoreV1().(*fakecorev1.FakeCoreV1).PrependReactor(
				"patch", "pods", func(action faketesting.Action) (handled bool, ret runtime.Object, err error) {
					<-finishUpdatesChan
					wgPods.Done()
					return false, nil, nil
				})

			stopCh = make(chan struct{})
			statusUpdater.Run(stopCh)

			numberOfJobs := 10
			jobs := []*jobs_fake.TestJobBasic{}
			for i := 0; i < numberOfJobs; i++ {
				jobs = append(jobs, &jobs_fake.TestJobBasic{
					Name:         "job-" + strconv.Itoa(i),
					Namespace:    "default",
					QueueName:    "queue-1",
					MinAvailable: ptr.To(int32(1)),
					Tasks: []*tasks_fake.TestTaskBasic{
						{
							Name:  "task-" + strconv.Itoa(i),
							State: pod_status.Pending,
						},
					},
				})
			}

			jobInfos, _, _ := jobs_fake.BuildJobsAndTasksMaps(jobs)
			podGroupsOriginals = []*schedulingv2alpha2.PodGroup{}
			podsOriginals = []*v1.Pod{}

			for _, job := range jobInfos {
				// Can't merge the loops because the fake client is handling concurrency by using a single lock
				_, err := kubeAiSchedClient.SchedulingV2alpha2().PodGroups(job.Namespace).Create(
					context.Background(), job.PodGroup, metav1.CreateOptions{})
				Expect(err).To(Succeed())
				podGroupsOriginals = append(podGroupsOriginals, job.PodGroup.DeepCopy())
				for _, task := range job.PodInfos {
					_, err := kubeClient.CoreV1().Pods(task.Namespace).Create(
						context.Background(), task.Pod, metav1.CreateOptions{})
					Expect(err).To(Succeed())
					podsOriginals = append(podsOriginals, task.Pod.DeepCopy())
				}
			}

			for _, job := range jobInfos {
				wgPodGroups.Add(1)
				wgPods.Add(len(job.PodInfos))
				if jobIndex, _ := strconv.Atoi(strings.Split(job.Name, "-")[0]); jobIndex%2 == 0 {
					job.StalenessInfo.TimeStamp = ptr.To(time.Now())
					job.StalenessInfo.Stale = true
					job.LastStartTimestamp = ptr.To(time.Now())
				}
				Expect(statusUpdater.RecordJobStatusEvent(job)).To(Succeed())
				for _, task := range job.PodInfos {
					wgPods.Add(1)
					statusUpdater.PatchPodLabels(task.Pod, map[string]any{
						"pod-group-name": job.Name,
					})
				}
			}
		})

		AfterEach(func() {
			select {
			case <-finishUpdatesChan:
			default:
				close(finishUpdatesChan)
			}
			wgPodGroups.Wait()
			wgPods.Wait()
			close(stopCh)
		})

		It("should update pod groups", func() {
			// check that the pods groups are now not updated anymore
			statusUpdater.SyncPodGroupsWithPendingUpdates(podGroupsOriginals)
			for _, podGroup := range podGroupsOriginals {
				Expect(podGroup.Status.SchedulingConditions).NotTo(BeEmpty())
			}
		})

		It("should NOT clear update cache if update was needed", func() {
			podGroupsOriginalsCopy := make([]*schedulingv2alpha2.PodGroup, len(podGroupsOriginals))
			for i := range podGroupsOriginals {
				podGroupsOriginalsCopy[i] = podGroupsOriginals[i].DeepCopy()
			}
			// check that the pod groups are updated with the changes that are not yet applied
			statusUpdater.SyncPodGroupsWithPendingUpdates(podGroupsOriginalsCopy)
			for _, podGroup := range podGroupsOriginalsCopy {
				Expect(podGroup.Status.SchedulingConditions).NotTo(BeEmpty())
			}

			// check that the pods groups are still updated
			statusUpdater.SyncPodGroupsWithPendingUpdates(podGroupsOriginals)
			for _, podGroup := range podGroupsOriginals {
				Expect(podGroup.Status.SchedulingConditions).NotTo(BeEmpty())
			}
		})

		It("should clear update cache after it syncs with pods groups that are updated", func() {
			close(finishUpdatesChan)
			wgPodGroups.Wait()

			podGroupsList, _ := kubeAiSchedClient.SchedulingV2alpha2().PodGroups("default").List(context.TODO(), metav1.ListOptions{})
			podGroupsFromCluster := make([]*schedulingv2alpha2.PodGroup, 0, len(podGroupsList.Items))
			for _, podGroup := range podGroupsList.Items {
				podGroupsFromCluster = append(podGroupsFromCluster, podGroup.DeepCopy())
			}

			statusUpdater.SyncPodGroupsWithPendingUpdates(podGroupsFromCluster)

			// check that the pods groups are now not updated anymore
			statusUpdater.SyncPodGroupsWithPendingUpdates(podGroupsOriginals)
			for _, podGroup := range podGroupsOriginals {
				Expect(podGroup.Status.SchedulingConditions).To(BeEmpty())
			}
		})

		It("should clear pod groups that don't show on sync from inFlight cache", func() {
			close(finishUpdatesChan)
			wgPodGroups.Wait()

			statusUpdater.SyncPodGroupsWithPendingUpdates([]*schedulingv2alpha2.PodGroup{})

			// check that the pods groups are now not updated anymore
			statusUpdater.SyncPodGroupsWithPendingUpdates(podGroupsOriginals)
			for _, podGroup := range podGroupsOriginals {
				Expect(podGroup.Status.SchedulingConditions).To(BeEmpty())
			}
		})

		It("should accept newer pod group versions as synced", func() {
			close(finishUpdatesChan)
			wgPodGroups.Wait()

			podGroupsList, _ := kubeAiSchedClient.SchedulingV2alpha2().PodGroups("default").List(context.TODO(), metav1.ListOptions{})
			podGroupsFromCluster := make([]*schedulingv2alpha2.PodGroup, 0, len(podGroupsList.Items))
			for _, podGroup := range podGroupsList.Items {
				podGroupCopy := podGroup.DeepCopy()
				lastTransitionIdStr := utils.GetLastSchedulingCondition(podGroupCopy).TransitionID
				lastTransitionId, _ := strconv.Atoi(lastTransitionIdStr)
				podGroupCopy.Status.SchedulingConditions = append(
					podGroupCopy.Status.SchedulingConditions,
					schedulingv2alpha2.SchedulingCondition{
						Type:               schedulingv2alpha2.UnschedulableOnNodePool,
						NodePool:           "other",
						Reason:             "test",
						Message:            "test",
						TransitionID:       "0" + strconv.Itoa(lastTransitionId+1),
						LastTransitionTime: metav1.Now(),
						Status:             v1.ConditionTrue,
					},
				)
				podGroupsFromCluster = append(podGroupsFromCluster, podGroupCopy)
			}

			statusUpdater.SyncPodGroupsWithPendingUpdates(podGroupsFromCluster)

			// check that the pods groups are now not updated anymore
			statusUpdater.SyncPodGroupsWithPendingUpdates(podGroupsOriginals)
			for _, podGroup := range podGroupsOriginals {
				Expect(podGroup.Status.SchedulingConditions).To(BeEmpty())
			}
		})

		It("should update snapshot pods", func() {
			// check that the pods are now not updated anymore
			statusUpdater.SyncPodsWithPendingUpdates(podsOriginals)
			for _, pod := range podsOriginals {
				Expect(len(pod.Status.Conditions)).To(BeEquivalentTo(1))
				Expect(len(pod.Labels)).To(BeEquivalentTo(2))
			}
		})

		It("Do not update status snapshot pods if they are already scheduled", func() {
			close(finishUpdatesChan)
			wgPods.Wait()

			scheduledPodsFromCluster := make([]*v1.Pod, 0, len(podsOriginals))
			for _, pod := range podsOriginals {
				copy := pod.DeepCopy()
				copy.Status.Conditions = append(copy.Status.Conditions, v1.PodCondition{
					Type:   v1.PodScheduled,
					Status: v1.ConditionTrue,
				})
				scheduledPodsFromCluster = append(scheduledPodsFromCluster, copy)
			}
			statusUpdater.SyncPodsWithPendingUpdates(scheduledPodsFromCluster)

			statusUpdater.SyncPodsWithPendingUpdates(podsOriginals)
			for _, pod := range podsOriginals {
				Expect(pod.Status.Conditions).To(BeEmpty())
			}
		})

	})

	Context("large scale: increase queue size", func() {
		It("should increase queue size", func() {
			wg := sync.WaitGroup{}
			signalCh := make(chan struct{})
			kubeClient.CoreV1().(*fakecorev1.FakeCoreV1).PrependReactor("patch", "pods", func(action faketesting.Action) (handled bool, ret runtime.Object, err error) {
				<-signalCh
				wg.Done()
				return true, nil, nil
			})
			stopCh := make(chan struct{})
			statusUpdater.Run(stopCh)
			defer close(stopCh)

			for i := 0; i < 2000; i++ {
				wg.Add(1)
				statusUpdater.updatePodCondition(
					&v1.Pod{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "pod-" + strconv.Itoa(i),
							Namespace: "default",
						},
					},
					&v1.PodCondition{
						Type:   v1.PodScheduled,
						Status: v1.ConditionTrue,
					},
				)
			}

			close(signalCh)
			wg.Wait()
		})
	})
})
