// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package solvers

import (
	"encoding/binary"
	"fmt"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	kaiv1 "github.com/kai-scheduler/KAI-scheduler/pkg/apis/kai/v1"
	"github.com/kai-scheduler/KAI-scheduler/pkg/common/constants"
	"github.com/kai-scheduler/KAI-scheduler/pkg/scheduler/actions/common/solvers/scenario"
	"github.com/kai-scheduler/KAI-scheduler/pkg/scheduler/actions/utils"
	"github.com/kai-scheduler/KAI-scheduler/pkg/scheduler/api"
	"github.com/kai-scheduler/KAI-scheduler/pkg/scheduler/api/common_info"
	"github.com/kai-scheduler/KAI-scheduler/pkg/scheduler/api/node_info"
	"github.com/kai-scheduler/KAI-scheduler/pkg/scheduler/api/pod_info"
	"github.com/kai-scheduler/KAI-scheduler/pkg/scheduler/api/podgroup_info"
	"github.com/kai-scheduler/KAI-scheduler/pkg/scheduler/api/queue_info"
	"github.com/kai-scheduler/KAI-scheduler/pkg/scheduler/api/resource_info"
	"github.com/kai-scheduler/KAI-scheduler/pkg/scheduler/framework"
)

func TestNewJobsSolverDefaultsNilBudgetToUnlimited(t *testing.T) {
	solver := NewJobsSolver(nil, nil, nil, framework.Reclaim, nil)

	require.NotNil(t, solver.actionBudget)
	require.False(t, solver.actionBudget.Exhausted())
	require.Greater(t, solver.actionBudget.BeginJob().Remaining(), time.Hour)
}

func TestSolveWithResultReturnsTerminalResultWhenNoTasksToAllocate(t *testing.T) {
	solver := NewJobsSolver(nil, nil, nil, framework.Reclaim, nil)
	pendingJob := podgroup_info.NewPodGroupInfo("pending-job")

	solved, statement, victims, result := solver.SolveWithResult(&framework.Session{}, pendingJob)

	require.False(t, solved)
	require.Nil(t, statement)
	require.Empty(t, victims)
	require.Equal(t, SearchResultGeneratorsExhausted, result.Reason())
	require.False(t, result.ReducedBudget())
}

func TestSolveWithResultRecordsNoSearchMetricAsNotAttempted(t *testing.T) {
	labels := map[string]string{
		"action":         "reclaim",
		"result":         string(SearchResultNotAttempted),
		"reduced_budget": "false",
	}
	before := scenarioSearchCounterValue(t, "scenario_search_jobs_total", labels)
	solver := NewJobsSolver(nil, nil, nil, framework.Reclaim, nil)
	pendingJob := podgroup_info.NewPodGroupInfo("pending-job")

	_, _, _, result := solver.SolveWithResult(&framework.Session{}, pendingJob)

	require.Equal(t, SearchResultGeneratorsExhausted, result.Reason())
	require.Equal(t, before+1, scenarioSearchCounterValue(t, "scenario_search_jobs_total", labels))
}

func TestSolveWithResultReturnsNoGeneratorWhenGeneratorFuncIsNil(t *testing.T) {
	ssn, pendingJob := newJobSolverResultTestSession(t, 1)
	solver := NewJobsSolver(nil, nil, nil, framework.Reclaim, nil)

	solved, statement, victims, result := solver.SolveWithResult(ssn, pendingJob)

	require.False(t, solved)
	require.Nil(t, statement)
	require.Empty(t, victims)
	require.Equal(t, SearchResultNoGenerator, result.Reason())
	require.False(t, result.ReducedBudget())
}

func TestSolveWithResultReturnsNoGeneratorWhenGeneratorReturnsNil(t *testing.T) {
	ssn, pendingJob := newJobSolverResultTestSession(t, 1)
	solver := NewJobsSolver(
		nil,
		nil,
		func() *utils.JobsOrderByQueues {
			return nil
		},
		framework.Reclaim,
		nil,
	)

	solved, statement, victims, result := solver.SolveWithResult(ssn, pendingJob)

	require.False(t, solved)
	require.Nil(t, statement)
	require.Empty(t, victims)
	require.Equal(t, SearchResultNoGenerator, result.Reason())
	require.False(t, result.ReducedBudget())
}

func TestSolveWithResultUsesMinJobBudgetAfterActionBudgetExpired(t *testing.T) {
	clock := &fakeClock{now: time.Unix(0, 0)}
	actionBudget, err := newActionSearchBudgetWithClock(
		sessionWithScenarioSearchBudgets(&kaiv1.ScenarioSearchBudgets{
			MaxActionSearchDuration: map[string]metav1.Duration{
				constants.ActionReclaim: scenarioSearchDurationForTest("10ms"),
			},
			MaxJobSearchDuration: scenarioSearchDurationPtrForTest("1s"),
			MinJobSearchDuration: scenarioSearchDurationPtrForTest("50ms"),
		}),
		framework.Reclaim,
		clock.Now,
	)
	require.NoError(t, err)
	ssn, pendingJob := newJobSolverResultTestSession(t, 1)
	solver := NewJobsSolver(nil, nil, nil, framework.Reclaim, actionBudget)

	clock.Advance(10 * time.Millisecond)
	solved, statement, victims, result := solver.SolveWithResult(ssn, pendingJob)

	require.False(t, solved)
	require.Nil(t, statement)
	require.Empty(t, victims)
	require.Equal(t, SearchResultNoGenerator, result.Reason())
	require.True(t, result.ReducedBudget())
}

func TestSolveWithResultReportsDeadlineWhenBudgetExhaustsDuringScenarioSearch(t *testing.T) {
	clock := &fakeClock{now: time.Unix(0, 0)}
	actionBudget, err := newActionSearchBudgetWithClock(
		sessionWithScenarioSearchBudgets(&kaiv1.ScenarioSearchBudgets{
			MaxActionSearchDuration: map[string]metav1.Duration{
				constants.ActionReclaim: scenarioSearchDurationForTest("10ms"),
			},
			MaxJobSearchDuration: scenarioSearchDurationPtrForTest("1ms"),
		}),
		framework.Reclaim,
		clock.Now,
	)
	require.NoError(t, err)
	ssn, pendingJob := newJobSolverResultTestSession(t, 1)
	node := node_info.NewNodeInfo(
		common_info.BuildNode("node-0", common_info.BuildResourceList("4", "16Gi")),
		nil, resource_info.NewResourceVectorMap(),
	)
	ssn.ClusterInfo.Nodes[node.Name] = node
	ssn.AddScenarioGenerator("deadline-test", func(ctx framework.ScenarioGeneratorContext) framework.ScenarioGenerator {
		solveCtx := ctx.(*SolveContext)
		solveCtx.GenerateVictimsQueue()
		return &portfolioTestGenerator{name: "deadline-test"}
	})
	solver := NewJobsSolver(
		[]*node_info.NodeInfo{node},
		nil,
		func() *utils.JobsOrderByQueues {
			clock.Advance(time.Millisecond)
			return utils.GetVictimsQueue(ssn, nil)
		},
		framework.Reclaim,
		actionBudget,
	)

	solved, statement, victims, result := solver.SolveWithResult(ssn, pendingJob)

	require.False(t, solved)
	require.Nil(t, statement)
	require.Empty(t, victims)
	require.Equal(t, SearchResultDeadlineExhausted, result.Reason())
}

func TestSolveWithResultRecordsGeneratorExhaustedMetricAfterGeneratorAttempt(t *testing.T) {
	labels := map[string]string{
		"action":         "reclaim",
		"result":         string(SearchResultGeneratorsExhausted),
		"reduced_budget": "false",
	}
	before := scenarioSearchCounterValue(t, "scenario_search_jobs_total", labels)
	ssn, pendingJob := newJobSolverResultTestSession(t, 1)
	ssn.AddScenarioGenerator("empty", portfolioTestFactory(&portfolioTestGenerator{name: "empty"}))
	solver := NewJobsSolver(
		nil,
		nil,
		func() *utils.JobsOrderByQueues {
			return utils.GetVictimsQueue(ssn, nil)
		},
		framework.Reclaim,
		nil,
	)

	_, _, _, result := solver.SolveWithResult(ssn, pendingJob)

	require.Equal(t, SearchResultGeneratorsExhausted, result.Reason())
	require.Equal(t, before+1, scenarioSearchCounterValue(t, "scenario_search_jobs_total", labels))
}

func TestSolveWithResultRecordsUnsolvedScenarioDurationAfterSimulation(t *testing.T) {
	generatorName := "test-unsolved-duration"
	labels := map[string]string{
		"action":    "reclaim",
		"generator": generatorName,
		"result":    scenarioSearchResultUnsolved,
	}
	before := scenarioSearchHistogramCount(t, "scenario_search_duration_seconds", labels)
	ssn, pendingJob := newJobSolverResultTestSession(t, 1)
	ssn.ClusterInfo.Nodes = map[string]*node_info.NodeInfo{"node-1": {}}
	scenarioToSolve := scenario.NewByNodeScenario(
		ssn, pendingJob,
		podgroup_info.GetTasksToAllocate(pendingJob, ssn.SubGroupOrderFn, ssn.TaskOrderFn, false),
		nil, nil,
	)
	ssn.AddScenarioGenerator(generatorName, portfolioTestFactory(&portfolioTestGenerator{
		name:      generatorName,
		scenarios: []api.ScenarioInfo{scenarioToSolve},
	}))
	solver := NewJobsSolver(
		nil,
		nil,
		func() *utils.JobsOrderByQueues {
			return utils.GetVictimsQueue(ssn, nil)
		},
		framework.Reclaim,
		nil,
	)

	solver.SolveWithResult(ssn, pendingJob)

	require.Equal(t, before+1, scenarioSearchHistogramCount(t, "scenario_search_duration_seconds", labels))
}

func TestSolveWithResultRunsCompletePartialSearchForOneGeneratorBeforeNext(t *testing.T) {
	ssn := newGeneratorTestSession(t, map[string]int{
		"node-1": 1,
		"node-2": 1,
		"node-3": 1,
	})
	require.NoError(t, ssn.InitNodeScoringPool())
	pendingJob := addGeneratorTestPendingJob(t, ssn, 3, 10, "team-pending")
	setGeneratorTestMinAvailable(pendingJob, 3)
	victimJob, victimTasks := addGeneratorTestJob(t, ssn, 3, 20, "team-victim", "node-1", "node-2", "node-3")
	factoryCalls := []string{}

	ssn.AddScenarioGenerator("first", func(ctx framework.ScenarioGeneratorContext) framework.ScenarioGenerator {
		solveCtx := ctx.(*SolveContext)
		factoryCalls = append(factoryCalls, fmt.Sprintf("first:%d", solveCtx.ProbeK))
		return &portfolioTestGenerator{name: "first"}
	})
	ssn.AddScenarioGenerator("second", func(ctx framework.ScenarioGeneratorContext) framework.ScenarioGenerator {
		solveCtx := ctx.(*SolveContext)
		factoryCalls = append(factoryCalls, fmt.Sprintf("second:%d", solveCtx.ProbeK))
		pendingTasks := podgroup_info.GetTasksToAllocate(
			solveCtx.PartialPendingJob, ssn.SubGroupOrderFn, ssn.TaskOrderFn, false,
		)
		sn := scenario.NewByNodeScenario(
			ssn, solveCtx.PartialPendingJob, pendingTasks,
			unrecordedVictimsForProbe(victimTasks, solveCtx.RecordedVictimsTasks, solveCtx.ProbeK),
			solveCtx.RecordedVictimsJobs,
		)
		return &portfolioTestGenerator{name: "second", scenarios: []api.ScenarioInfo{sn}}
	})
	solver := NewJobsSolver(
		jobSolverResultTestFeasibleNodes(ssn),
		nil,
		generatorTestVictimsQueueFactory(ssn, victimJob),
		framework.Reclaim,
		nil,
	)

	solved, statement, _, result := solver.SolveWithResult(ssn, pendingJob)
	if statement != nil {
		defer statement.Discard()
	}

	require.True(t, solved)
	require.Equal(t, SearchResultSolved, result.Reason())
	require.Equal(t, []string{"first:1", "second:1", "second:2", "second:3", "second:3"}, factoryCalls)
}

func TestSolveWithResultStillSolvesWhenGeneratorRepeatsScenarios(t *testing.T) {
	ssn := newGeneratorTestSession(t, map[string]int{
		"node-1": 1,
		"node-2": 1,
		"node-3": 1,
	})
	require.NoError(t, ssn.InitNodeScoringPool())
	pendingJob := addGeneratorTestPendingJob(t, ssn, 3, 10, "team-pending")
	setGeneratorTestMinAvailable(pendingJob, 3)
	victimJob, victimTasks := addGeneratorTestJob(t, ssn, 3, 20, "team-victim", "node-1", "node-2", "node-3")
	generatorName := "dedup-e2e"

	ssn.AddScenarioGenerator(generatorName, func(ctx framework.ScenarioGeneratorContext) framework.ScenarioGenerator {
		solveCtx := ctx.(*SolveContext)
		pendingTasks := podgroup_info.GetTasksToAllocate(
			solveCtx.PartialPendingJob, ssn.SubGroupOrderFn, ssn.TaskOrderFn, false,
		)
		failing := scenario.NewByNodeScenario(
			ssn, solveCtx.PartialPendingJob, pendingTasks, nil, solveCtx.RecordedVictimsJobs,
		)
		failingDuplicate := scenario.NewByNodeScenario(
			ssn, solveCtx.PartialPendingJob, pendingTasks, nil, solveCtx.RecordedVictimsJobs,
		)
		solving := scenario.NewByNodeScenario(
			ssn, solveCtx.PartialPendingJob, pendingTasks,
			unrecordedVictimsForProbe(victimTasks, solveCtx.RecordedVictimsTasks, solveCtx.ProbeK),
			solveCtx.RecordedVictimsJobs,
		)
		return &portfolioTestGenerator{
			name:      generatorName,
			scenarios: []api.ScenarioInfo{failing, failingDuplicate, solving},
		}
	})
	labels := map[string]string{"action": "reclaim", "generator": generatorName, "state": scenarioStateDuplicate}
	before := scenarioSearchCounterValue(t, "scenario_search_scenarios_total", labels)
	solver := NewJobsSolver(
		jobSolverResultTestFeasibleNodes(ssn),
		nil,
		generatorTestVictimsQueueFactory(ssn, victimJob),
		framework.Reclaim,
		nil,
	)

	solved, statement, _, result := solver.SolveWithResult(ssn, pendingJob)
	if statement != nil {
		defer statement.Discard()
	}

	require.True(t, solved)
	require.NotNil(t, statement)
	require.Equal(t, SearchResultSolved, result.Reason())
	require.Greater(t, scenarioSearchCounterValue(t, "scenario_search_scenarios_total", labels), before)
}

func TestSearchMaxSolvableKStopsAfterTerminalPartialProbe(t *testing.T) {
	probes := map[int]*SearchResult{
		1: solvedSearchResult(&solutionResult{solved: true}, false),
		2: terminalSearchResult(SearchResultDeadlineExhausted, false),
	}

	maxSolvedK, result := searchMaxSolvableK(3, func(k int) *SearchResult {
		return probes[k]
	})

	require.Equal(t, 0, maxSolvedK)
	require.Equal(t, SearchResultDeadlineExhausted, result.Reason())
}

func TestSearchMaxSolvableKResumesWithoutEarlierProbes(t *testing.T) {
	firstCalls := []int{}
	_, result, cursor := searchMaxSolvableKFromCursor(8, framework.JobSolverCursor{}, func(k int) *SearchResult {
		firstCalls = append(firstCalls, k)
		if k == 4 {
			return terminalSearchResult(SearchResultDeadlineExhausted, false)
		}
		return solvedSearchResult(&solutionResult{solved: true}, false)
	})
	require.Equal(t, []int{1, 2, 4}, firstCalls)
	require.Equal(t, SearchResultDeadlineExhausted, result.Reason())
	require.Equal(t, jobSolverPhaseExponential, cursor.Phase)
	require.EqualValues(t, 2, cursor.Lo)
	require.EqualValues(t, 4, cursor.ProbeK)

	secondCalls := []int{}
	maxSolvedK, result, cursor := searchMaxSolvableKFromCursor(8, cursor, func(k int) *SearchResult {
		secondCalls = append(secondCalls, k)
		if k == 4 {
			return terminalSearchResult(SearchResultGeneratorsExhausted, false)
		}
		return solvedSearchResult(&solutionResult{solved: true}, false)
	})
	require.Equal(t, []int{4, 3}, secondCalls)
	require.Equal(t, 3, maxSolvedK)
	require.Equal(t, SearchResultGeneratorsExhausted, result.Reason())
	require.Equal(t, jobSolverPhaseFinal, cursor.Phase)
	require.EqualValues(t, 3, cursor.Lo)
	require.EqualValues(t, 8, cursor.ProbeK)
}

func TestJobSolverCheckpointResumesNextCandidateAcrossSessions(t *testing.T) {
	store := framework.NewScenarioCheckpointStore()
	firstClock := &fakeClock{now: time.Unix(0, 0)}
	firstCalls := []string{}
	firstSession, firstPending, firstVictim, firstVictims := newCheckpointAcceptanceSession(t, store)
	firstBudget := checkpointAcceptanceBudget(t, firstSession, firstClock)
	firstSession.AddScenarioGenerator("checkpoint-test", checkpointAcceptanceGeneratorFactory(firstSession, firstVictims, firstClock, true, 2*time.Millisecond, false, 0, &firstCalls))
	firstSession.InitializeScenarioCheckpointState()
	firstSolver := NewJobsSolver(
		jobSolverResultTestFeasibleNodes(firstSession), nil, generatorTestVictimsQueueFactory(firstSession, firstVictim), framework.Reclaim, firstBudget,
	)
	_, statement, _, result := firstSolver.SolveWithResult(firstSession, firstPending)
	if statement != nil {
		statement.Discard()
	}
	require.Equal(t, SearchResultDeadlineExhausted, result.Reason())
	checkpoint, found := store.Load(checkpointKey(framework.Reclaim, firstPending))
	require.True(t, found)
	require.Equal(t, jobSolverPhaseExponential, checkpoint.SolverCursor.Phase)
	require.EqualValues(t, 1, checkpoint.SolverCursor.Lo)
	require.EqualValues(t, 2, checkpoint.SolverCursor.ProbeK)
	require.NotEmpty(t, checkpoint.RecordedVictims)
	require.Equal(t, []string{"1:1", "1:2", "2:1"}, firstCalls)

	secondClock := &fakeClock{now: time.Unix(0, 0)}
	secondCalls := []string{}
	secondSession, secondPending, secondVictim, secondVictims := newCheckpointAcceptanceSession(t, store)
	secondBudget := checkpointAcceptanceBudget(t, secondSession, secondClock)
	secondSession.AddScenarioGenerator("checkpoint-test", checkpointAcceptanceGeneratorFactory(secondSession, secondVictims, secondClock, false, 0, false, 0, &secondCalls))
	secondSession.InitializeScenarioCheckpointState()
	secondSolver := NewJobsSolver(
		jobSolverResultTestFeasibleNodes(secondSession), nil, generatorTestVictimsQueueFactory(secondSession, secondVictim), framework.Reclaim, secondBudget,
	)
	loadLabels := map[string]string{"operation": "load", "result": "hit"}
	loadsBefore := scenarioSearchCounterValue(t, "scenario_search_checkpoints_total", loadLabels)
	solved, statement, _, result := secondSolver.SolveWithResult(secondSession, secondPending)
	if statement != nil {
		statement.Discard()
	}
	require.True(t, solved)
	require.Equal(t, SearchResultSolved, result.Reason())
	require.NotEmpty(t, secondCalls)
	require.Equal(t, "2:2", secondCalls[0], "restored search must simulate candidate N+1 first")
	require.NotContains(t, secondCalls, "1:1")
	require.NotContains(t, secondCalls, "1:2")
	require.Equal(t, loadsBefore+1, scenarioSearchCounterValue(t, "scenario_search_checkpoints_total", loadLabels))
	_, found = store.Load(checkpointKey(framework.Reclaim, secondPending))
	require.False(t, found)
}

func TestJobSolverCheckpointSavesDiscardedProbeBeforeNextCandidate(t *testing.T) {
	store := framework.NewScenarioCheckpointStore()
	firstClock := &fakeClock{now: time.Unix(0, 0)}
	firstCalls := []string{}
	firstSession, firstPending, firstVictim, firstVictims := newCheckpointAcceptanceSession(t, store)
	firstBudget := checkpointAcceptanceBudget(t, firstSession, firstClock)
	firstSession.AddScenarioGenerator("checkpoint-test", checkpointAcceptanceGeneratorFactory(firstSession, firstVictims, firstClock, false, 2*time.Millisecond, false, 0, &firstCalls, true))
	firstSession.InitializeScenarioCheckpointState()
	firstSolver := NewJobsSolver(jobSolverResultTestFeasibleNodes(firstSession), nil, generatorTestVictimsQueueFactory(firstSession, firstVictim), framework.Reclaim, firstBudget)
	_, statement, _, result := firstSolver.SolveWithResult(firstSession, firstPending)
	if statement != nil {
		statement.Discard()
	}
	require.Equal(t, SearchResultDeadlineExhausted, result.Reason())
	checkpoint, found := store.Load(checkpointKey(framework.Reclaim, firstPending))
	require.True(t, found)
	require.True(t, checkpoint.StateOnly)
	require.EqualValues(t, 1, checkpoint.GeneratorCursor.Version)
	require.Equal(t, jobSolverPhaseExponential, checkpoint.SolverCursor.Phase)
	require.EqualValues(t, 1, checkpoint.SolverCursor.Lo)
	require.EqualValues(t, 2, checkpoint.SolverCursor.ProbeK)
	require.NotEmpty(t, checkpoint.RecordedVictims)
	require.Equal(t, []string{"1:1", "1:2"}, firstCalls)

	secondClock := &fakeClock{now: time.Unix(0, 0)}
	secondCalls := []string{}
	secondSession, secondPending, secondVictim, secondVictims := newCheckpointAcceptanceSession(t, store)
	secondBudget := checkpointAcceptanceBudget(t, secondSession, secondClock)
	secondSession.AddScenarioGenerator("checkpoint-test", checkpointAcceptanceGeneratorFactory(secondSession, secondVictims, secondClock, false, 0, true, 0, &secondCalls))
	secondSession.InitializeScenarioCheckpointState()
	secondSolver := NewJobsSolver(jobSolverResultTestFeasibleNodes(secondSession), nil, generatorTestVictimsQueueFactory(secondSession, secondVictim), framework.Reclaim, secondBudget)
	solved, statement, _, result := secondSolver.SolveWithResult(secondSession, secondPending)
	if statement != nil {
		statement.Discard()
	}
	require.True(t, solved)
	require.Equal(t, SearchResultSolved, result.Reason())
	require.NotEmpty(t, secondCalls)
	require.Equal(t, "2:1", secondCalls[0])
	require.NotContains(t, secondCalls, "1:1")
	require.NotContains(t, secondCalls, "1:2")
	_, found = store.Load(checkpointKey(framework.Reclaim, secondPending))
	require.False(t, found)
}

func TestJobSolverDeletesCheckpointAfterGeneratorExhaustion(t *testing.T) {
	store := framework.NewScenarioCheckpointStore()
	clock := &fakeClock{now: time.Unix(0, 0)}
	calls := []string{}
	first, pending, _, firstVictims := newCheckpointAcceptanceSession(t, store)
	_ = checkpointAcceptanceBudget(t, first, clock)
	first.AddScenarioGenerator("checkpoint-test", checkpointAcceptanceGeneratorFactory(first, firstVictims, clock, false, 0, false, 0, &calls))
	first.InitializeScenarioCheckpointState()
	firstTasks := podgroup_info.GetTasksToAllocate(pending, first.SubGroupOrderFn, first.TaskOrderFn, false)
	firstNodes := map[string]*node_info.NodeInfo{}
	for name, node := range first.ClusterInfo.Nodes {
		firstNodes[name] = node
	}
	firstCtx := &SolveContext{Session: first, ActionType: framework.Reclaim, PartialPendingJob: getPartialJobRepresentative(pending, firstTasks[:1]), FeasibleNodes: firstNodes, ProbeK: 1}
	var cursor framework.ScenarioGeneratorCursor
	cursor.Version = 1
	binary.BigEndian.PutUint32(cursor.Data[:4], 2)
	saveScenarioCheckpoint(firstCtx, firstNodes, "checkpoint-test", cursor, framework.JobSolverCursor{Phase: jobSolverPhaseExponential, ProbeK: 1}, nil, SearchResultDeadlineExhausted)

	second, secondPending, secondVictim, secondVictims := newCheckpointAcceptanceSession(t, store)
	secondBudget := checkpointAcceptanceBudget(t, second, clock)
	second.AddScenarioGenerator("checkpoint-test", checkpointAcceptanceGeneratorFactory(second, secondVictims, clock, false, 0, false, 0, &calls))
	second.InitializeScenarioCheckpointState()
	solver := NewJobsSolver(jobSolverResultTestFeasibleNodes(second), nil, generatorTestVictimsQueueFactory(second, secondVictim), framework.Reclaim, secondBudget)
	solved, statement, _, result := solver.SolveWithResult(second, secondPending)
	if statement != nil {
		statement.Discard()
	}
	require.False(t, solved)
	require.Equal(t, SearchResultGeneratorsExhausted, result.Reason())
	require.Empty(t, calls)
	_, found := store.Load(checkpointKey(framework.Reclaim, secondPending))
	require.False(t, found)
}

func TestJobSolverCheckpointDeletesOnRestoreFailure(t *testing.T) {
	store := framework.NewScenarioCheckpointStore()
	firstClock := &fakeClock{now: time.Unix(0, 0)}
	firstCalls := []string{}
	firstSession, firstPending, firstVictim, firstVictims := newCheckpointAcceptanceSession(t, store)
	firstSession.AddScenarioGenerator("checkpoint-test", checkpointAcceptanceGeneratorFactory(firstSession, firstVictims, firstClock, true, 2*time.Millisecond, false, 0, &firstCalls))
	firstSession.InitializeScenarioCheckpointState()
	firstSolver := NewJobsSolver(jobSolverResultTestFeasibleNodes(firstSession), nil, generatorTestVictimsQueueFactory(firstSession, firstVictim), framework.Reclaim, checkpointAcceptanceBudget(t, firstSession, firstClock))
	_, statement, _, result := firstSolver.SolveWithResult(firstSession, firstPending)
	if statement != nil {
		statement.Discard()
	}
	require.Equal(t, SearchResultDeadlineExhausted, result.Reason())
	_, found := store.Load(checkpointKey(framework.Reclaim, firstPending))
	require.True(t, found)

	secondClock := &fakeClock{now: time.Unix(0, 0)}
	secondCalls := []string{}
	secondSession, secondPending, secondVictim, secondVictims := newCheckpointAcceptanceSession(t, store)
	secondSession.AddScenarioGenerator("checkpoint-test", checkpointAcceptanceGeneratorFactory(secondSession, secondVictims, secondClock, false, 0, true, 0, &secondCalls))
	secondSession.InitializeScenarioCheckpointState()
	secondSolver := NewJobsSolver(jobSolverResultTestFeasibleNodes(secondSession), nil, generatorTestVictimsQueueFactory(secondSession, secondVictim), framework.Reclaim, checkpointAcceptanceBudget(t, secondSession, secondClock))
	_, statement, _, _ = secondSolver.SolveWithResult(secondSession, secondPending)
	if statement != nil {
		statement.Discard()
	}
	_, found = store.Load(checkpointKey(framework.Reclaim, secondPending))
	require.False(t, found)
}

func TestJobSolverCheckpointPreservedWhenRestoreExhaustsBudget(t *testing.T) {
	store := framework.NewScenarioCheckpointStore()
	firstClock := &fakeClock{now: time.Unix(0, 0)}
	firstCalls := []string{}
	firstSession, firstPending, firstVictim, firstVictims := newCheckpointAcceptanceSession(t, store)
	firstSession.AddScenarioGenerator("checkpoint-test", checkpointAcceptanceGeneratorFactory(firstSession, firstVictims, firstClock, true, 2*time.Millisecond, false, 0, &firstCalls))
	firstSession.InitializeScenarioCheckpointState()
	firstSolver := NewJobsSolver(jobSolverResultTestFeasibleNodes(firstSession), nil, generatorTestVictimsQueueFactory(firstSession, firstVictim), framework.Reclaim, checkpointAcceptanceBudget(t, firstSession, firstClock))
	_, statement, _, result := firstSolver.SolveWithResult(firstSession, firstPending)
	if statement != nil {
		statement.Discard()
	}
	require.Equal(t, SearchResultDeadlineExhausted, result.Reason())
	before, found := store.Load(checkpointKey(framework.Reclaim, firstPending))
	require.True(t, found)

	secondClock := &fakeClock{now: time.Unix(0, 0)}
	secondCalls := []string{}
	secondSession, secondPending, secondVictim, secondVictims := newCheckpointAcceptanceSession(t, store)
	secondSession.AddScenarioGenerator("checkpoint-test", checkpointAcceptanceGeneratorFactory(secondSession, secondVictims, secondClock, false, 0, false, 2*time.Millisecond, &secondCalls))
	secondSession.InitializeScenarioCheckpointState()
	secondSolver := NewJobsSolver(jobSolverResultTestFeasibleNodes(secondSession), nil, generatorTestVictimsQueueFactory(secondSession, secondVictim), framework.Reclaim, checkpointAcceptanceBudget(t, secondSession, secondClock))
	_, statement, _, result = secondSolver.SolveWithResult(secondSession, secondPending)
	if statement != nil {
		statement.Discard()
	}
	require.Equal(t, SearchResultDeadlineExhausted, result.Reason())
	require.Empty(t, secondCalls)
	after, found := store.Load(checkpointKey(framework.Reclaim, secondPending))
	require.True(t, found)
	require.Equal(t, before, after)
}

func TestJobSolverCheckpointRestoreStartsFreshGeneratorBudget(t *testing.T) {
	store := framework.NewScenarioCheckpointStore()
	firstClock := &fakeClock{now: time.Unix(0, 0)}
	firstCalls := []string{}
	first, pending, victim, victims := newCheckpointAcceptanceSession(t, store)
	first.AddScenarioGenerator("checkpoint-test", checkpointAcceptanceGeneratorFactory(first, victims, firstClock, true, 11*time.Millisecond, false, 0, &firstCalls))
	first.InitializeScenarioCheckpointState()
	firstSolver := NewJobsSolver(jobSolverResultTestFeasibleNodes(first), nil, generatorTestVictimsQueueFactory(first, victim), framework.Reclaim, checkpointAcceptanceBudgetWith(t, first, firstClock, "10ms", "1ms"))
	_, statement, _, result := firstSolver.SolveWithResult(first, pending)
	if statement != nil {
		statement.Discard()
	}
	require.Equal(t, SearchResultDeadlineExhausted, result.Reason())

	secondClock := &fakeClock{now: time.Unix(0, 0)}
	secondCalls := []string{}
	second, secondPending, secondVictim, secondVictims := newCheckpointAcceptanceSession(t, store)
	second.AddScenarioGenerator("checkpoint-test", checkpointAcceptanceGeneratorFactory(second, secondVictims, secondClock, false, 0, false, 2*time.Millisecond, &secondCalls))
	second.InitializeScenarioCheckpointState()
	secondSolver := NewJobsSolver(jobSolverResultTestFeasibleNodes(second), nil, generatorTestVictimsQueueFactory(second, secondVictim), framework.Reclaim, checkpointAcceptanceBudgetWith(t, second, secondClock, "10ms", "1ms"))
	solved, statement, _, result := secondSolver.SolveWithResult(second, secondPending)
	if statement != nil {
		statement.Discard()
	}
	require.True(t, solved)
	require.Equal(t, SearchResultSolved, result.Reason())
	require.Equal(t, "2:2", secondCalls[0])
}

func TestScenarioCheckpointInvalidatesBeforeBitmapDecode(t *testing.T) {
	store := framework.NewScenarioCheckpointStore()
	ssn, pending, _, _ := newCheckpointAcceptanceSession(t, store)
	ssn.InitializeScenarioCheckpointState()
	tasks := podgroup_info.GetTasksToAllocate(pending, ssn.SubGroupOrderFn, ssn.TaskOrderFn, false)
	base := map[string]*node_info.NodeInfo{}
	for name, node := range ssn.ClusterInfo.Nodes {
		base[name] = node
	}
	ctx := &SolveContext{Session: ssn, ActionType: framework.Reclaim, PartialPendingJob: getPartialJobRepresentative(pending, tasks[:1]), FeasibleNodes: base, ProbeK: 1}
	key := checkpointKey(framework.Reclaim, pending)
	store.Save(key, framework.ScenarioCheckpoint{
		GeneratorName: "checkpoint-test", GeneratorCursor: framework.ScenarioGeneratorCursor{Version: 1}, SolverCursor: framework.JobSolverCursor{ProbeK: 1},
		RecordedVictims: []byte{0xff}, // Invalid length and set bits: decoder must not inspect it on fingerprint mismatch.
	})
	checkpoint, err := loadScenarioCheckpoint(ctx, base, "checkpoint-test")
	require.Nil(t, checkpoint)
	require.Error(t, err)
	_, found := store.Load(key)
	require.False(t, found)
}

func TestScenarioCheckpointInvalidatesOnPodUniverseChurn(t *testing.T) {
	store := framework.NewScenarioCheckpointStore()
	first, pending, _, _ := newCheckpointAcceptanceSession(t, store)
	first.InitializeScenarioCheckpointState()
	firstTasks := podgroup_info.GetTasksToAllocate(pending, first.SubGroupOrderFn, first.TaskOrderFn, false)
	firstNodes := map[string]*node_info.NodeInfo{}
	for name, node := range first.ClusterInfo.Nodes {
		firstNodes[name] = node
	}
	firstCtx := &SolveContext{Session: first, ActionType: framework.Reclaim, PartialPendingJob: getPartialJobRepresentative(pending, firstTasks[:1]), FeasibleNodes: firstNodes, ProbeK: 1}
	var cursor framework.ScenarioGeneratorCursor
	cursor.Version = 1
	saveScenarioCheckpoint(firstCtx, firstNodes, "checkpoint-test", cursor, framework.JobSolverCursor{Phase: jobSolverPhaseExponential, ProbeK: 1}, nil, SearchResultDeadlineExhausted)
	_, found := store.Load(checkpointKey(framework.Reclaim, pending))
	require.True(t, found)

	second, secondPending, _, _ := newCheckpointAcceptanceSession(t, store)
	addGeneratorTestJob(t, second, 1, 99, "team-churn", "node-1")
	second.InitializeScenarioCheckpointState()
	secondTasks := podgroup_info.GetTasksToAllocate(secondPending, second.SubGroupOrderFn, second.TaskOrderFn, false)
	secondNodes := map[string]*node_info.NodeInfo{}
	for name, node := range second.ClusterInfo.Nodes {
		secondNodes[name] = node
	}
	secondCtx := &SolveContext{Session: second, ActionType: framework.Reclaim, PartialPendingJob: getPartialJobRepresentative(secondPending, secondTasks[:1]), FeasibleNodes: secondNodes, ProbeK: 1}
	checkpoint, err := loadScenarioCheckpoint(secondCtx, secondNodes, "checkpoint-test")
	require.Nil(t, checkpoint)
	require.Error(t, err)
	_, found = store.Load(checkpointKey(framework.Reclaim, secondPending))
	require.False(t, found)
}

func TestScenarioCheckpointInvalidatesOnPodReplacement(t *testing.T) {
	store := framework.NewScenarioCheckpointStore()
	first, pending, _, _ := newCheckpointAcceptanceSession(t, store)
	first.InitializeScenarioCheckpointState()
	firstTasks := podgroup_info.GetTasksToAllocate(pending, first.SubGroupOrderFn, first.TaskOrderFn, false)
	firstNodes := map[string]*node_info.NodeInfo{}
	for name, node := range first.ClusterInfo.Nodes {
		firstNodes[name] = node
	}
	firstCtx := &SolveContext{Session: first, ActionType: framework.Reclaim, PartialPendingJob: getPartialJobRepresentative(pending, firstTasks[:1]), FeasibleNodes: firstNodes, ProbeK: 1}
	saveScenarioCheckpoint(firstCtx, firstNodes, "checkpoint-test", framework.ScenarioGeneratorCursor{Version: 1}, framework.JobSolverCursor{Phase: jobSolverPhaseExponential, ProbeK: 1}, nil, SearchResultDeadlineExhausted)

	second, secondPending, _, secondVictims := newCheckpointAcceptanceSession(t, store)
	secondVictims[0].UID = "replacement-pod"
	secondVictims[0].Pod.UID = "replacement-pod"
	second.InitializeScenarioCheckpointState()
	secondTasks := podgroup_info.GetTasksToAllocate(secondPending, second.SubGroupOrderFn, second.TaskOrderFn, false)
	secondNodes := map[string]*node_info.NodeInfo{}
	for name, node := range second.ClusterInfo.Nodes {
		secondNodes[name] = node
	}
	secondCtx := &SolveContext{Session: second, ActionType: framework.Reclaim, PartialPendingJob: getPartialJobRepresentative(secondPending, secondTasks[:1]), FeasibleNodes: secondNodes, ProbeK: 1}
	checkpoint, err := loadScenarioCheckpoint(secondCtx, secondNodes, "checkpoint-test")
	require.Nil(t, checkpoint)
	require.Error(t, err)
	_, found := store.Load(checkpointKey(framework.Reclaim, secondPending))
	require.False(t, found)
}

func TestScenarioCheckpointInvalidatesOnPodDeletion(t *testing.T) {
	store := framework.NewScenarioCheckpointStore()
	first, pending, _, _ := newCheckpointAcceptanceSession(t, store)
	first.InitializeScenarioCheckpointState()
	firstTasks := podgroup_info.GetTasksToAllocate(pending, first.SubGroupOrderFn, first.TaskOrderFn, false)
	firstNodes := map[string]*node_info.NodeInfo{}
	for name, node := range first.ClusterInfo.Nodes {
		firstNodes[name] = node
	}
	firstCtx := &SolveContext{Session: first, ActionType: framework.Reclaim, PartialPendingJob: getPartialJobRepresentative(pending, firstTasks[:1]), FeasibleNodes: firstNodes, ProbeK: 1}
	cursor := framework.ScenarioGeneratorCursor{Version: 1}
	saveScenarioCheckpoint(firstCtx, firstNodes, "checkpoint-test", cursor, framework.JobSolverCursor{Phase: jobSolverPhaseExponential, ProbeK: 1}, nil, SearchResultDeadlineExhausted)

	second, secondPending, deletedJob, _ := newCheckpointAcceptanceSession(t, store)
	delete(second.ClusterInfo.PodGroupInfos, deletedJob.UID)
	second.InitializeScenarioCheckpointState()
	secondTasks := podgroup_info.GetTasksToAllocate(secondPending, second.SubGroupOrderFn, second.TaskOrderFn, false)
	secondNodes := map[string]*node_info.NodeInfo{}
	for name, node := range second.ClusterInfo.Nodes {
		secondNodes[name] = node
	}
	secondCtx := &SolveContext{Session: second, ActionType: framework.Reclaim, PartialPendingJob: getPartialJobRepresentative(secondPending, secondTasks[:1]), FeasibleNodes: secondNodes, ProbeK: 1}
	checkpoint, err := loadScenarioCheckpoint(secondCtx, secondNodes, "checkpoint-test")
	require.Nil(t, checkpoint)
	require.Error(t, err)
	_, found := store.Load(checkpointKey(framework.Reclaim, secondPending))
	require.False(t, found)
}

func TestJobSolverDoesNotCheckpointUnsupportedGenerator(t *testing.T) {
	store := framework.NewScenarioCheckpointStore()
	clock := &fakeClock{now: time.Unix(0, 0)}
	ssn, pending, _, _ := newCheckpointAcceptanceSession(t, store)
	ssn.AddScenarioGenerator("unsupported-checkpoint-test", func(ctx framework.ScenarioGeneratorContext) framework.ScenarioGenerator {
		pendingTasks := podgroup_info.GetTasksToAllocate(ctx.(*SolveContext).PartialPendingJob, ssn.SubGroupOrderFn, ssn.TaskOrderFn, false)
		return &portfolioTestGenerator{name: "unsupported-checkpoint-test", scenarios: []api.ScenarioInfo{scenario.NewByNodeScenario(ssn, ctx.(*SolveContext).PartialPendingJob, pendingTasks, nil, nil)}, onNext: func() { clock.Advance(2 * time.Millisecond) }}
	})
	ssn.InitializeScenarioCheckpointState()
	budgetSession := sessionWithScenarioSearchBudgets(&kaiv1.ScenarioSearchBudgets{MaxActionSearchDuration: map[string]metav1.Duration{constants.ActionReclaim: scenarioSearchDurationForTest("1ms")}, MaxJobSearchDuration: scenarioSearchDurationPtrForTest("1ms")})
	ssn.Config = budgetSession.Config
	budget, err := newActionSearchBudgetWithClock(ssn, framework.Reclaim, clock.Now)
	require.NoError(t, err)
	solver := NewJobsSolver(jobSolverResultTestFeasibleNodes(ssn), nil, func() *utils.JobsOrderByQueues { return utils.GetVictimsQueue(ssn, nil) }, framework.Reclaim, budget)
	_, statement, _, result := solver.SolveWithResult(ssn, pending)
	if statement != nil {
		statement.Discard()
	}
	require.Equal(t, SearchResultDeadlineExhausted, result.Reason())
	require.Equal(t, 0, store.Len())
}

func TestJobSolverDeletesCheckpointForGeneratorMismatch(t *testing.T) {
	store := framework.NewScenarioCheckpointStore()
	ssn, pending, _, _ := newCheckpointAcceptanceSession(t, store)
	store.Save(checkpointKey(framework.Reclaim, pending), framework.ScenarioCheckpoint{GeneratorName: "old", SolverCursor: framework.JobSolverCursor{ProbeK: 1}})
	tasks := podgroup_info.GetTasksToAllocate(pending, ssn.SubGroupOrderFn, ssn.TaskOrderFn, false)
	solver := NewJobsSolver(jobSolverResultTestFeasibleNodes(ssn), nil, nil, framework.Reclaim, nil)
	solver.restoreCheckpointedSolverState(ssn, &solvingState{}, pending, tasks, framework.ScenarioGeneratorRegistration{Name: "new"})
	require.Equal(t, 0, store.Len())
}

func newCheckpointAcceptanceSession(t *testing.T, store *framework.ScenarioCheckpointStore) (*framework.Session, *podgroup_info.PodGroupInfo, *podgroup_info.PodGroupInfo, []*pod_info.PodInfo) {
	t.Helper()
	ssn := newGeneratorTestSession(t, map[string]int{"node-1": 1, "node-2": 1, "node-3": 1})
	require.NoError(t, ssn.InitNodeScoringPool())
	pending := addGeneratorTestPendingJob(t, ssn, 3, 10, "team-pending")
	setGeneratorTestMinAvailable(pending, 3)
	victim, victims := addGeneratorTestJob(t, ssn, 3, 20, "team-victim", "node-1", "node-2", "node-3")
	ssn.ScenarioCheckpointStore = store
	return ssn, pending, victim, victims
}

func checkpointAcceptanceBudget(t *testing.T, ssn *framework.Session, clock *fakeClock) *ActionSearchBudget {
	return checkpointAcceptanceBudgetWith(t, ssn, clock, "1ms", "1s")
}

func checkpointAcceptanceBudgetWith(t *testing.T, ssn *framework.Session, clock *fakeClock, outerLimit, generatorLimit string) *ActionSearchBudget {
	t.Helper()
	budgetSession := sessionWithScenarioSearchBudgets(&kaiv1.ScenarioSearchBudgets{
		MaxActionSearchDuration:    map[string]metav1.Duration{constants.ActionReclaim: scenarioSearchDurationForTest(outerLimit)},
		MaxJobSearchDuration:       scenarioSearchDurationPtrForTest(outerLimit),
		MaxGeneratorSearchDuration: map[string]metav1.Duration{"checkpoint-test": scenarioSearchDurationForTest(generatorLimit)},
	})
	ssn.Config = budgetSession.Config
	budget, err := newActionSearchBudgetWithClock(ssn, framework.Reclaim, clock.Now)
	require.NoError(t, err)
	return budget
}

func checkpointAcceptanceGeneratorFactory(ssn *framework.Session, victims []*pod_info.PodInfo, clock *fakeClock, expireProbeTwo bool, expireAdvance time.Duration, failRestore bool, restoreAdvance time.Duration, calls *[]string, expireAfterFirstProbeSuccess ...bool) framework.ScenarioGeneratorFactory {
	return func(ctx framework.ScenarioGeneratorContext) framework.ScenarioGenerator {
		return &checkpointAcceptanceGenerator{ctx: ctx.(*SolveContext), session: ssn, victims: victims, clock: clock, expireProbeTwo: expireProbeTwo, expireAdvance: expireAdvance, expireAfterFirstProbeSuccess: len(expireAfterFirstProbeSuccess) > 0 && expireAfterFirstProbeSuccess[0], failRestore: failRestore, restoreAdvance: restoreAdvance, calls: calls}
	}
}

type checkpointAcceptanceGenerator struct {
	ctx                          *SolveContext
	session                      *framework.Session
	victims                      []*pod_info.PodInfo
	clock                        *fakeClock
	expireProbeTwo               bool
	expireAdvance                time.Duration
	expireAfterFirstProbeSuccess bool
	failRestore                  bool
	restoreAdvance               time.Duration
	calls                        *[]string
	next                         uint32
}

func (g *checkpointAcceptanceGenerator) Name() string { return "checkpoint-test" }

func (g *checkpointAcceptanceGenerator) Next() api.ScenarioInfo {
	if g.next >= 2 {
		return nil
	}
	g.next++
	*g.calls = append(*g.calls, fmt.Sprintf("%d:%d", g.ctx.ProbeK, g.next))
	if (g.expireProbeTwo && g.ctx.ProbeK == 2 && g.next == 1) ||
		(g.expireAfterFirstProbeSuccess && g.ctx.ProbeK == 1 && g.next == 2) {
		g.clock.Advance(g.expireAdvance)
	}
	pendingTasks := podgroup_info.GetTasksToAllocate(g.ctx.PartialPendingJob, g.session.SubGroupOrderFn, g.session.TaskOrderFn, false)
	if g.next == 1 {
		return scenario.NewByNodeScenario(g.session, g.ctx.PartialPendingJob, pendingTasks, nil, g.ctx.RecordedVictimsJobs)
	}
	return scenario.NewByNodeScenario(g.session, g.ctx.PartialPendingJob, pendingTasks,
		unrecordedVictimsForProbe(g.victims, g.ctx.RecordedVictimsTasks, g.ctx.ProbeK), g.ctx.RecordedVictimsJobs)
}

func (g *checkpointAcceptanceGenerator) Cursor() (framework.ScenarioGeneratorCursor, bool) {
	if g.next == 0 {
		return framework.ScenarioGeneratorCursor{}, false
	}
	var cursor framework.ScenarioGeneratorCursor
	cursor.Version = 1
	binary.BigEndian.PutUint32(cursor.Data[:4], g.next)
	return cursor, true
}

func (g *checkpointAcceptanceGenerator) Restore(cursor framework.ScenarioGeneratorCursor) error {
	if g.failRestore {
		return fmt.Errorf("injected checkpoint restore failure")
	}
	g.clock.Advance(g.restoreAdvance)
	if cursor.Version != 1 || binary.BigEndian.Uint32(cursor.Data[:4]) == 0 || binary.BigEndian.Uint32(cursor.Data[:4]) > 2 {
		return fmt.Errorf("invalid checkpoint acceptance cursor")
	}
	g.next = binary.BigEndian.Uint32(cursor.Data[:4])
	return nil
}

func jobSolverResultTestFeasibleNodes(ssn *framework.Session) []*node_info.NodeInfo {
	nodes := make([]*node_info.NodeInfo, 0, len(ssn.ClusterInfo.Nodes))
	for _, node := range ssn.ClusterInfo.Nodes {
		nodes = append(nodes, node)
	}
	return nodes
}

func unrecordedVictimsForProbe(
	victimTasks []*pod_info.PodInfo, recordedVictims []*pod_info.PodInfo, probeK int,
) []*pod_info.PodInfo {
	recordedByUID := map[common_info.PodID]struct{}{}
	for _, task := range recordedVictims {
		recordedByUID[task.UID] = struct{}{}
	}

	neededVictims := probeK - len(recordedVictims)
	if neededVictims <= 0 {
		return nil
	}

	selectedVictims := make([]*pod_info.PodInfo, 0, neededVictims)
	for _, task := range victimTasks {
		if _, alreadyRecorded := recordedByUID[task.UID]; alreadyRecorded {
			continue
		}
		selectedVictims = append(selectedVictims, task)
		if len(selectedVictims) == neededVictims {
			return selectedVictims
		}
	}
	return selectedVictims
}

func newJobSolverResultTestSession(t *testing.T, tasksCount int) (*framework.Session, *podgroup_info.PodGroupInfo) {
	t.Helper()

	pendingJob, _ := createJobWithTasks(tasksCount, 1, "team-a", v1.PodPending, nil)
	defaultQueue := createQueue("default")
	defaultQueue.ParentQueue = ""
	submitQueue := createQueue("team-a")

	return &framework.Session{
		ClusterInfo: &api.ClusterInfo{
			PodGroupInfos: map[common_info.PodGroupID]*podgroup_info.PodGroupInfo{
				pendingJob.UID: pendingJob,
			},
			Queues: map[common_info.QueueID]*queue_info.QueueInfo{
				defaultQueue.UID: defaultQueue,
				submitQueue.UID:  submitQueue,
			},
			Nodes: map[string]*node_info.NodeInfo{},
		},
	}, pendingJob
}

func scenarioSearchCounterValue(t *testing.T, metricName string, labels map[string]string) float64 {
	t.Helper()

	metric := scenarioSearchMetric(t, metricName, labels)
	if metric == nil || metric.GetCounter() == nil {
		return 0
	}
	return metric.GetCounter().GetValue()
}

func scenarioSearchHistogramCount(t *testing.T, metricName string, labels map[string]string) uint64 {
	t.Helper()

	metric := scenarioSearchMetric(t, metricName, labels)
	if metric == nil || metric.GetHistogram() == nil {
		return 0
	}
	return metric.GetHistogram().GetSampleCount()
}

func scenarioSearchMetric(t *testing.T, metricName string, labels map[string]string) *dto.Metric {
	t.Helper()

	families, err := prometheus.DefaultGatherer.Gather()
	require.NoError(t, err)
	for _, family := range families {
		if family.GetName() != metricName {
			continue
		}
		for _, metric := range family.GetMetric() {
			if scenarioSearchMetricHasLabels(metric, labels) {
				return metric
			}
		}
	}
	return nil
}

func scenarioSearchMetricHasLabels(metric *dto.Metric, labels map[string]string) bool {
	if len(metric.GetLabel()) != len(labels) {
		return false
	}
	for _, label := range metric.GetLabel() {
		expectedValue, found := labels[label.GetName()]
		if !found || expectedValue != label.GetValue() {
			return false
		}
	}
	return true
}
