// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package solvers

import (
	"fmt"
	"strconv"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"golang.org/x/exp/slices"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	schedulingv2 "github.com/NVIDIA/KAI-scheduler/pkg/apis/scheduling/v2"
	schedulingv2alpha2 "github.com/NVIDIA/KAI-scheduler/pkg/apis/scheduling/v2alpha2"
	commonconstants "github.com/NVIDIA/KAI-scheduler/pkg/common/constants"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/actions/common/solvers/scenario"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/actions/utils"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/common_info"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/node_info"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/pod_info"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/podgroup_info"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/queue_info"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/resource_info"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/framework"
)

var _ = Describe("PodAccumulatedScenarioBuilder", func() {
	var (
		ssn             *framework.Session
		reclaimerJob    *podgroup_info.PodGroupInfo
		scenarioBuilder *PodAccumulatedScenarioBuilder
	)
	Context("with no jobs", func() {
		BeforeEach(func() {
			ssn, _ = initializeSession(0, 0)
			submitQueue := createQueue("team-a")
			ssn.Queues[submitQueue.UID] = submitQueue
			reclaimerJob, _ = createJobWithTasks(1, 1, "team-a", v1.PodPending)
			recordedVictimsJobs := []*podgroup_info.PodGroupInfo{}
			victimsQueue := utils.GetVictimsQueue(ssn, nil)

			scenarioBuilder = NewPodAccumulatedScenarioBuilder(ssn, reclaimerJob, recordedVictimsJobs, victimsQueue)
		})

		It("If reclaimer job has pods to scheduler, first scenario exists", func() {
			Expect(scenarioBuilder.GetCurrentScenario()).To(Not(BeNil()))
		})
		It("returns nil scenario", func() {
			Expect(scenarioBuilder.GetNextScenario()).To(BeNil())
		})
	})

	Context("with optional victim jobs", func() {
		BeforeEach(func() {
			ssn, _ = initializeSession(2, 2)
			submitQueue := createQueue("team-a")
			ssn.Queues[submitQueue.UID] = submitQueue
			reclaimerJob, _ = createJobWithTasks(1, 1, "team-a", v1.PodPending)
		})

		It("returns scenario with all tasks in single groups when minAvailable is 1", func() {
			scenarioBuilder = NewPodAccumulatedScenarioBuilder(ssn, reclaimerJob, []*podgroup_info.PodGroupInfo{},
				utils.GetVictimsQueue(ssn, nil))

			var lastScenario *scenario.ByNodeScenario
			for tempScenario := scenarioBuilder.GetCurrentScenario(); tempScenario != nil; tempScenario =
				scenarioBuilder.GetNextScenario() {
				lastScenario = tempScenario
			}

			Expect(lastScenario).NotTo(BeNil())

			Expect(len(lastScenario.PotentialVictimsTasks())).To(Equal(4))
			for _, task := range lastScenario.PotentialVictimsTasks() {
				matchingJob := lastScenario.GetVictimJobRepresentativeById(task)
				Expect(len(matchingJob.PodInfos)).To(Equal(1))
			}

		})

		It("returns scenario with all tasks in single groups when minAvailable is amount of pods", func() {
			for _, podGroupInfo := range ssn.PodGroupInfos {
				podGroupInfo.MinAvailable = int32(len(podGroupInfo.PodInfos))
				podGroupInfo.PodGroup.Spec.MinMember = int32(len(podGroupInfo.PodInfos))
			}
			scenarioBuilder = NewPodAccumulatedScenarioBuilder(ssn, reclaimerJob, []*podgroup_info.PodGroupInfo{},
				utils.GetVictimsQueue(ssn, nil))

			var lastScenario *scenario.ByNodeScenario
			for tempScenario := scenarioBuilder.GetCurrentScenario(); tempScenario != nil; tempScenario =
				scenarioBuilder.GetNextScenario() {
				lastScenario = tempScenario
			}

			Expect(lastScenario).NotTo(BeNil())

			Expect(len(lastScenario.PotentialVictimsTasks())).To(Equal(4))
			for _, task := range lastScenario.PotentialVictimsTasks() {
				matchingJob := lastScenario.GetVictimJobRepresentativeById(task)
				Expect(len(matchingJob.PodInfos)).To(Equal(2))
			}
		})
	})

	Context("with recorded victims", func() {
		It("All scenarios have the same recorded victims", func() {
			ssn, _ = initializeSession(3, 2)
			for _, podGroupInfo := range ssn.PodGroupInfos {
				podGroupInfo.MinAvailable = int32(len(podGroupInfo.PodInfos))
				podGroupInfo.PodGroup.Spec.MinMember = int32(len(podGroupInfo.PodInfos))
			}
			submitQueue := createQueue("team-a")
			ssn.Queues[submitQueue.UID] = submitQueue
			reclaimerJob, _ = createJobWithTasks(1, 1, "team-a", v1.PodPending)

			var recordedVictimsJobs []*podgroup_info.PodGroupInfo
			recordedVictimIndexes := []int{0, 2}
			podGroupIndex := 0

			for _, podGroupInfo := range ssn.PodGroupInfos {
				if slices.Contains(recordedVictimIndexes, podGroupIndex) {
					recordedVictimsJobs = append(recordedVictimsJobs, podGroupInfo)
				}
				podGroupIndex += 1
			}

			victimsQueue := utils.GetVictimsQueue(ssn, nil)

			scenarioBuilder = NewPodAccumulatedScenarioBuilder(ssn, reclaimerJob, recordedVictimsJobs, victimsQueue)

			numberOfGeneratedScenarios := 0
			for sn := scenarioBuilder.GetCurrentScenario(); sn != nil; sn = scenarioBuilder.GetNextScenario() {
				Expect(len(sn.RecordedVictimsJobs())).To(Equal(len(recordedVictimsJobs)))
				numberOfGeneratedScenarios += 1
			}

			Expect(numberOfGeneratedScenarios).To(Equal(2))
		})
	})
})

func initializeSession(jobsCount, tasksPerJob int) (*framework.Session, []*pod_info.PodInfo) {
	tasks := []*pod_info.PodInfo{}
	jobs := []*podgroup_info.PodGroupInfo{}

	defaultQueue := createQueue("default")
	defaultQueue.ParentQueue = ""
	queues := []*queue_info.QueueInfo{defaultQueue}

	for jobID := 0; jobID < jobsCount; jobID++ {
		jobTasks := []*pod_info.PodInfo{}
		queueName := fmt.Sprintf("team-%d", jobID)
		newJob, jobTasks := createJobWithTasks(tasksPerJob, jobID, queueName, v1.PodRunning)
		jobs = append(jobs, newJob)
		tasks = append(tasks, jobTasks...)
		queues = append(queues, createQueue(queueName))
	}

	podgroup_infos := map[common_info.PodGroupID]*podgroup_info.PodGroupInfo{}
	for _, job := range jobs {
		podgroup_infos[job.UID] = job
	}

	queuesMap := map[common_info.QueueID]*queue_info.QueueInfo{}
	for _, queue := range queues {
		queuesMap[queue.UID] = queue
	}

	ssn := &framework.Session{
		PodGroupInfos: podgroup_infos,
		Queues:        queuesMap,
		Nodes: map[string]*node_info.NodeInfo{
			"node-1": node_info.NewNodeInfo(&v1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "node-1",
				},
			}, nil),
		},
	}
	return ssn, tasks
}

func createQueue(queueName string) *queue_info.QueueInfo {
	return queue_info.NewQueueInfo(&schedulingv2.Queue{
		ObjectMeta: metav1.ObjectMeta{
			Name: queueName,
		},
		Spec: schedulingv2.QueueSpec{
			ParentQueue: "default",
			Resources: &schedulingv2.QueueResources{
				GPU: schedulingv2.QueueResource{
					Quota: 1,
				},
			},
		},
	})
}

func createJobWithTasks(
	tasksPerJob int, jobID int, queueName string, tasksStatus v1.PodPhase,
) (*podgroup_info.PodGroupInfo, []*pod_info.PodInfo) {
	jobTasks := []*pod_info.PodInfo{}

	namespace := "kai-" + queueName
	jobUID := strconv.Itoa(jobID)

	for taskID := 0; taskID < tasksPerJob; taskID++ {
		taskNum := jobID*tasksPerJob + taskID
		jobTasks = append(jobTasks, pod_info.NewTaskInfo(
			buildPod(
				strconv.Itoa(taskNum),
				fmt.Sprintf("pod-%d", taskNum),
				namespace, jobUID, tasksStatus,
				[]v1.ResourceRequirements{requireOneGPU()},
			)))
	}

	newJob := podgroup_info.NewPodGroupInfo(common_info.PodGroupID(strconv.Itoa(jobID)), jobTasks...)
	newJob.SetPodGroup(&schedulingv2alpha2.PodGroup{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("job%d", jobID),
			Namespace: namespace,
			UID:       types.UID(jobUID),
		},
		Spec: schedulingv2alpha2.PodGroupSpec{
			MinMember: 1,
			Queue:     queueName,
		},
	})

	return newJob, jobTasks
}

func buildPod(
	uid, name, namespace, podGroupID string, status v1.PodPhase, resources []v1.ResourceRequirements,
) *v1.Pod {
	pod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			UID:       types.UID(uid),
			Annotations: map[string]string{
				commonconstants.PodGroupAnnotationForPod: podGroupID,
			},
		},
		Spec: v1.PodSpec{
			Containers: func() []v1.Container {
				var containers []v1.Container
				for _, r := range resources {
					containers = append(containers, v1.Container{
						Resources: r,
					})
				}
				return containers
			}(),
		},
		Status: v1.PodStatus{
			Phase: status,
		},
	}

	if status == v1.PodRunning {
		pod.Spec.NodeName = "node-1"
	}

	return pod
}

func requireOneGPU() v1.ResourceRequirements {
	return v1.ResourceRequirements{
		Requests: v1.ResourceList{
			resource_info.GPUResourceName: resource.MustParse("1"),
		},
	}
}
