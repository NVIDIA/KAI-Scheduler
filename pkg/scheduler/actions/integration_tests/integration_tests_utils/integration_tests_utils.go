// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package integration_tests_utils

import (
	"fmt"
	"testing"
	"time"

	. "go.uber.org/mock/gomock"

	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/actions/allocate"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/actions/consolidation"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/actions/preempt"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/actions/reclaim"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/actions/stalegangeviction"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/common_info"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/pod_status"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/framework"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/log"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/test_utils"
)

type TestTopologyMetadata struct {
	Name string
	test_utils.TestTopologyBasic
	RoundsUntilMatch   int
	RoundsAfterMatch   int           // Used in order to verify the test scenario remain (no allocation/delete loop)
	SchedulingDuration time.Duration // Used to specify delay between rounds, leave empty if not needed
}

const (
	defaultRoundsAfterMatch = 5
	defaultRoundsUntilMatch = 2
)

var schedulerActions []framework.Action

func RunTests(t *testing.T, testsMetadata []TestTopologyMetadata) {
	test_utils.InitTestingInfrastructure()
	SetSchedulerActions()
	controller := NewController(t)
	defer controller.Finish()

	for testNumber, testMetadata := range testsMetadata {
		RunTest(t, testMetadata, testNumber, controller)
	}
}

func RunTest(t *testing.T, testMetadata TestTopologyMetadata, testNumber int, controller *Controller) {
	t.Logf("Running test number: %v, test name: %v", testNumber, testMetadata.TestTopologyBasic.Name)
	ssn := test_utils.BuildSession(testMetadata.TestTopologyBasic, controller)

	runRoundsUntilMatch(testMetadata, controller, &ssn)
	test_utils.MatchExpectedAndRealTasks(t, testNumber, testMetadata.TestTopologyBasic, ssn)
	runRoundsAfterAndMatch(t, testMetadata, controller, ssn, testNumber)
}

func runRoundsAfterAndMatch(t *testing.T, testMetadata TestTopologyMetadata, controller *Controller, ssn *framework.Session, testNumber int) {
	roundsAfterMatch := defaultRoundsAfterMatch
	if testMetadata.RoundsAfterMatch != 0 {
		roundsAfterMatch = testMetadata.RoundsAfterMatch
	}
	for i := 0; i < roundsAfterMatch; i++ {
		runSchedulerOneRound(&testMetadata, controller, &ssn)
		test_utils.MatchExpectedAndRealTasks(t, testNumber, testMetadata.TestTopologyBasic, ssn)
	}
}

func runRoundsUntilMatch(testMetadata TestTopologyMetadata, controller *Controller, ssn **framework.Session) {
	roundsUntilMatch := defaultRoundsUntilMatch
	if testMetadata.RoundsUntilMatch != 0 {
		roundsUntilMatch = testMetadata.RoundsUntilMatch
	}
	for i := 0; i < roundsUntilMatch; i++ {
		runSchedulerOneRound(&testMetadata, controller, ssn)
		time.Sleep(testMetadata.SchedulingDuration)
	}
}

func runSchedulerOneRound(testMetadata *TestTopologyMetadata, controller *Controller, ssn **framework.Session) {
	for _, action := range schedulerActions {
		log.InfraLogger.SetAction(string(action.Name()))
		action.Execute(*ssn)
	}

	for _, jobMetadata := range testMetadata.Jobs {
		jobId := common_info.PodGroupID(jobMetadata.Name)
		job := (*ssn).PodGroupInfos[jobId]
		for taskId, taskMetadata := range jobMetadata.Tasks {
			task := job.PodInfos[common_info.PodID(fmt.Sprintf("%s-%d", jobId, taskId))]
			switch task.Status {
			case pod_status.Releasing:
				if jobMetadata.DeleteJobInTest {
					taskMetadata.NodeName = task.NodeName
					taskMetadata.GPUGroups = task.GPUGroups
					taskMetadata.State = pod_status.Releasing
				} else {
					taskMetadata.NodeName = ""
					taskMetadata.State = pod_status.Pending
				}

			case pod_status.Pipelined:
				taskMetadata.NodeName = ""
				taskMetadata.State = pod_status.Pending

			case pod_status.Binding:
				taskMetadata.State = pod_status.Running
				taskMetadata.NodeName = task.NodeName
				taskMetadata.GPUGroups = task.GPUGroups

			default:
				taskMetadata.State = task.Status
				taskMetadata.NodeName = task.NodeName
				taskMetadata.GPUGroups = task.GPUGroups
			}

		}
	}

	*ssn = test_utils.BuildSession(testMetadata.TestTopologyBasic, controller)
}

func SetSchedulerActions() {
	schedulerActions = []framework.Action{allocate.New(), consolidation.New(), reclaim.New(), preempt.New(), stalegangeviction.New()}
}
