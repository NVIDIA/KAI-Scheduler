// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package solvers

import (
	"fmt"
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/util/sets"

	"github.com/kai-scheduler/KAI-scheduler/pkg/scheduler/actions/utils"
	"github.com/kai-scheduler/KAI-scheduler/pkg/scheduler/api"
	"github.com/kai-scheduler/KAI-scheduler/pkg/scheduler/api/node_info"
	"github.com/kai-scheduler/KAI-scheduler/pkg/scheduler/api/pod_info"
	"github.com/kai-scheduler/KAI-scheduler/pkg/scheduler/api/podgroup_info"
	"github.com/kai-scheduler/KAI-scheduler/pkg/scheduler/api/podgroup_info/subgroup_info"
	"github.com/kai-scheduler/KAI-scheduler/pkg/scheduler/framework"
	"github.com/kai-scheduler/KAI-scheduler/pkg/scheduler/log"
	"github.com/kai-scheduler/KAI-scheduler/pkg/scheduler/metrics"
)

type GenerateVictimsQueue func() *utils.JobsOrderByQueues

type JobSolver struct {
	feasibleNodes        []*node_info.NodeInfo
	solutionValidator    SolutionValidator
	generateVictimsQueue GenerateVictimsQueue
	actionType           framework.ActionType
	actionBudget         *ActionSearchBudget
	failedScenarios      sets.Set[scenarioFingerprint]
}

type solvingState struct {
	recordedVictimsJobs  []*podgroup_info.PodGroupInfo
	recordedVictimsTasks []*pod_info.PodInfo
	solverCursor         framework.JobSolverCursor
	checkpoint           *framework.ScenarioCheckpoint
	generatorBudget      *generatorSearchBudget
	lastGeneratorCursor  framework.ScenarioGeneratorCursor
	lastGeneratorName    string
	saveSolverState      bool
}

const (
	jobSolverPhaseExponential uint8 = 1
	jobSolverPhaseBinary      uint8 = 2
	jobSolverPhaseFinal       uint8 = 3
)

func NewJobsSolver(
	feasibleNodes []*node_info.NodeInfo,
	solutionValidator SolutionValidator,
	generateVictimsQueue GenerateVictimsQueue,
	action framework.ActionType,
	actionBudget *ActionSearchBudget,
) *JobSolver {
	budget := actionBudget
	if budget == nil {
		budget = newUnlimitedActionSearchBudget(action)
	}
	return &JobSolver{
		feasibleNodes:        feasibleNodes,
		solutionValidator:    solutionValidator,
		generateVictimsQueue: generateVictimsQueue,
		actionType:           action,
		actionBudget:         budget,
	}
}

func newUnlimitedActionSearchBudget(action framework.ActionType) *ActionSearchBudget {
	now := time.Now
	return &ActionSearchBudget{
		action:   action,
		deadline: newDeadlineBudget(unlimitedRemaining, now),
	}
}

// Solve attempts to find a feasible allocation for all of pendingJob's pending tasks,
// evicting tasks from other jobs as victims when necessary. It operates with all-or-nothing
// semantics: either the full set of pending tasks is scheduled, or no allocation is produced.
//
// Returns:
//   - solved: true when every pending task was allocated and pendingJob is gang-satisfied.
//   - statement: on success, a live Statement holding the speculative allocations and victim
//     evictions; the caller is responsible for Commit or Discard. nil on failure.
//   - victimTaskNames: formatted "<namespace>/<name>" strings of the victim tasks, for logging.
//
// Session state is mutated only on success (to reflect the speculative operations in the
// returned statement) and is left unchanged on failure.
func (s *JobSolver) Solve(
	ssn *framework.Session, pendingJob *podgroup_info.PodGroupInfo) (bool, *framework.Statement, []string) {
	solved, statement, victimTaskNames, _ := s.SolveWithResult(ssn, pendingJob)
	return solved, statement, victimTaskNames
}

// SolveWithResult attempts to solve pendingJob and returns a structured search result
// describing why the scenario search stopped.
func (s *JobSolver) SolveWithResult(
	ssn *framework.Session, pendingJob *podgroup_info.PodGroupInfo,
) (solved bool, statement *framework.Statement, victimTaskNames []string, searchResult *SearchResult) {
	defer func() {
		if searchResult != nil {
			metrics.IncScenarioSearchJobs(
				s.actionType, searchResult.scenarioSearchMetricResult(), searchResult.ReducedBudget(),
			)
		}
	}()

	originalNumActiveTasks := pendingJob.GetNumActiveUsedTasks()

	tasksToAllocate := podgroup_info.GetTasksToAllocate(pendingJob, ssn.SubGroupOrderFn, ssn.TaskOrderFn, false)
	n := len(tasksToAllocate)
	if n == 0 {
		searchResult := terminalSearchResult(SearchResultGeneratorsExhausted, false)
		searchResult.metricResult = string(SearchResultNotAttempted)
		return false, nil, nil, searchResult
	}

	jobBudget := s.actionBudget.BeginJob()
	if jobBudget.Exhausted() {
		return false, nil, nil, terminalSearchResult(SearchResultNotAttempted, false)
	}

	if s.generateVictimsQueue == nil {
		return false, nil, nil, terminalSearchResult(SearchResultNoGenerator, jobBudget.ReducedBudget())
	}
	availableGenerators := ssn.ScenarioGeneratorRegistrations
	if len(availableGenerators) == 0 {
		return false, nil, nil, terminalSearchResult(SearchResultNoGenerator, jobBudget.ReducedBudget())
	}

	s.failedScenarios = sets.New[scenarioFingerprint]()

	var lastVictimTasks []*pod_info.PodInfo
	var lastResult *SearchResult
	for _, availableGenerator := range availableGenerators {
		state := solvingState{}
		generatorBudget := jobBudget.BeginGenerator(availableGenerator.Name)
		result := s.solvePendingJobWithGenerator(
			ssn, &state, pendingJob, tasksToAllocate, jobBudget, availableGenerator, generatorBudget,
		)
		lastVictimTasks = state.recordedVictimsTasks
		lastResult = result

		if resultSolved(result) {
			solution := result.solution
			numActiveTasks := pendingJob.GetNumActiveUsedTasks()
			jobSolved := pendingJob.IsGangSatisfied()
			if originalNumActiveTasks >= numActiveTasks {
				jobSolved = false
			}

			log.InfraLogger.V(4).Infof(
				"Scenario solved for %d tasks to allocate for %s. Victims: %s",
				n, pendingJob.Name, victimPrintingStruct{solution.victimsTasks})
			return jobSolved, solution.statement, calcVictimNames(solution.victimsTasks), result
		}

		if shouldStopSearch(result) {
			return false, nil, calcVictimNames(lastVictimTasks), result
		}
	}

	if lastResult == nil {
		lastResult = terminalSearchResult(SearchResultGeneratorsExhausted, jobBudget.ReducedBudget())
	}
	return false, nil, calcVictimNames(lastVictimTasks), lastResult
}

func (s *JobSolver) solvePendingJobWithGenerator(
	ssn *framework.Session,
	state *solvingState,
	pendingJob *podgroup_info.PodGroupInfo,
	tasksToAllocate []*pod_info.PodInfo,
	jobBudget *jobSearchBudget,
	availableGenerator framework.ScenarioGeneratorRegistration,
	generatorBudget *generatorSearchBudget,
) *SearchResult {
	n := len(tasksToAllocate)
	state.generatorBudget = generatorBudget
	s.restoreCheckpointedSolverState(ssn, state, pendingJob, tasksToAllocate, availableGenerator)
	maxSolvedK, searchResult := s.searchMaxSolvableK(
		ssn, state, pendingJob, tasksToAllocate, jobBudget, availableGenerator, generatorBudget,
	)
	if maxSolvedK == 0 {
		if searchResult == nil {
			searchResult = terminalSearchResult(SearchResultGeneratorsExhausted, jobBudget.ReducedBudget())
		}
		return searchResult
	}

	result := s.probeAtK(ssn, state, pendingJob, tasksToAllocate, n, jobBudget, availableGenerator, generatorBudget)
	return result
}

// restoreCheckpointedSolverState restores the next probe before the probe state
// machine starts. solvePartialJob performs generator restore for that same probe.
func (s *JobSolver) restoreCheckpointedSolverState(
	ssn *framework.Session,
	state *solvingState,
	pendingJob *podgroup_info.PodGroupInfo,
	tasksToAllocate []*pod_info.PodInfo,
	availableGenerator framework.ScenarioGeneratorRegistration,
) {
	if ssn == nil || ssn.ScenarioCheckpointStore == nil || pendingJob == nil {
		return
	}
	checkpoint, found := ssn.ScenarioCheckpointStore.Load(checkpointKey(s.actionType, pendingJob))
	if !found {
		return
	}
	if checkpoint.GeneratorName != availableGenerator.Name || checkpoint.SolverCursor.ProbeK == 0 ||
		int(checkpoint.SolverCursor.ProbeK) > len(tasksToAllocate) {
		ssn.ScenarioCheckpointStore.Delete(checkpointKey(s.actionType, pendingJob))
		return
	}
	probeK := int(checkpoint.SolverCursor.ProbeK)
	feasible := make(map[string]*node_info.NodeInfo, len(s.feasibleNodes))
	for _, node := range s.feasibleNodes {
		feasible[node.Name] = node
	}
	ctx := &SolveContext{
		Session: ssn, ActionType: s.actionType,
		PartialPendingJob:    getPartialJobRepresentative(pendingJob, tasksToAllocate[:probeK]),
		GenerateVictimsQueue: s.generateVictimsQueue, FeasibleNodes: feasible, ProbeK: probeK,
	}
	loaded, err := validateScenarioCheckpoint(ctx, feasible, availableGenerator.Name, checkpoint)
	if err != nil || loaded == nil {
		return
	}
	state.solverCursor = loaded.SolverCursor
	state.recordedVictimsTasks = ctx.RecordedVictimsTasks
	state.recordedVictimsJobs = ctx.RecordedVictimsJobs
	state.checkpoint = loaded
}

// searchMaxSolvableK returns the largest k in [0, n] for which a probe at k succeeds.
// Each probe is discarded before returning, so session state is clean on return.
// Successful probes update hints in state for use by subsequent probes.
// Complexity: O(log n) probes — exponential doubling to locate a failing k (or reach n),
// then binary search between the last success and first failure.
func (s *JobSolver) searchMaxSolvableK(
	ssn *framework.Session,
	state *solvingState,
	pendingJob *podgroup_info.PodGroupInfo,
	tasksToAllocate []*pod_info.PodInfo,
	jobBudget *jobSearchBudget,
	availableGenerator framework.ScenarioGeneratorRegistration,
	generatorBudget *generatorSearchBudget,
) (int, *SearchResult) {
	n := len(tasksToAllocate)
	if n == 0 {
		return 0, nil
	}

	maxSolvedK, result, cursor := searchMaxSolvableKFromCursor(n, state.solverCursor, func(k int) *SearchResult {
		return s.tryProbeAndDiscard(
			ssn, state, pendingJob, tasksToAllocate, k, jobBudget, availableGenerator, generatorBudget,
		)
	}, func(cursor framework.JobSolverCursor) {
		state.solverCursor = cursor
		if !state.saveSolverState {
			return
		}
		s.saveDiscardedProbeState(ssn, state, pendingJob, tasksToAllocate, availableGenerator, cursor)
		state.saveSolverState = false
	})
	state.solverCursor = cursor
	return maxSolvedK, result
}

func searchMaxSolvableK(n int, probe func(k int) *SearchResult) (int, *SearchResult) {
	maxSolvedK, result, _ := searchMaxSolvableKFromCursor(n, framework.JobSolverCursor{}, probe)
	return maxSolvedK, result
}

// searchMaxSolvableKFromCursor resumes exponential and binary probing at cursor.ProbeK.
// The returned cursor identifies either the next probe or the final full-job probe.
func searchMaxSolvableKFromCursor(
	n int, cursor framework.JobSolverCursor, probe func(k int) *SearchResult, progress ...func(framework.JobSolverCursor),
) (int, *SearchResult, framework.JobSolverCursor) {
	if n == 0 {
		return 0, nil, framework.JobSolverCursor{}
	}
	if cursor.Phase == 0 {
		cursor = framework.JobSolverCursor{Phase: jobSolverPhaseExponential, ProbeK: 1}
	}
	if cursor.ProbeK == 0 || int(cursor.ProbeK) > n || int(cursor.Lo) > n || int(cursor.Hi) > n ||
		(cursor.Phase != jobSolverPhaseExponential && cursor.Phase != jobSolverPhaseBinary && cursor.Phase != jobSolverPhaseFinal) {
		return 0, terminalSearchResult(SearchResultGeneratorsExhausted, false), framework.JobSolverCursor{}
	}
	notify := func() {
		for _, callback := range progress {
			callback(cursor)
		}
	}
	notify()
	var lastUnsolvedResult *SearchResult
	for {
		if cursor.Phase == jobSolverPhaseFinal {
			return int(cursor.Lo), lastUnsolvedResult, cursor
		}
		result := probe(int(cursor.ProbeK))
		if shouldStopSearch(result) {
			return 0, result, cursor
		}
		switch cursor.Phase {
		case jobSolverPhaseExponential:
			if resultSolved(result) {
				cursor.Lo = cursor.ProbeK
				if int(cursor.Lo) == n {
					cursor.Phase = jobSolverPhaseFinal
					cursor.ProbeK = uint32(n)
					notify()
					return n, lastUnsolvedResult, cursor
				}
				next := int(cursor.ProbeK) * 2
				if next > n {
					next = n
				}
				cursor.ProbeK = uint32(next)
				notify()
				continue
			}
			lastUnsolvedResult = result
			cursor.Hi = cursor.ProbeK
			if cursor.Hi-cursor.Lo <= 1 {
				cursor.Phase = jobSolverPhaseFinal
				cursor.ProbeK = uint32(n)
				notify()
				return int(cursor.Lo), lastUnsolvedResult, cursor
			}
			cursor.Phase = jobSolverPhaseBinary
			cursor.ProbeK = (cursor.Lo + cursor.Hi) / 2
			notify()
		case jobSolverPhaseBinary:
			if resultSolved(result) {
				cursor.Lo = cursor.ProbeK
			} else {
				lastUnsolvedResult = result
				cursor.Hi = cursor.ProbeK
			}
			if cursor.Hi-cursor.Lo <= 1 {
				cursor.Phase = jobSolverPhaseFinal
				cursor.ProbeK = uint32(n)
				notify()
				return int(cursor.Lo), lastUnsolvedResult, cursor
			}
			cursor.ProbeK = (cursor.Lo + cursor.Hi) / 2
			notify()
		}
	}
}

// tryProbeAndDiscard probes at k and always discards a solved statement so the session
// is left clean. On success, hints are written to state.
func (s *JobSolver) tryProbeAndDiscard(
	ssn *framework.Session,
	state *solvingState,
	pendingJob *podgroup_info.PodGroupInfo,
	tasksToAllocate []*pod_info.PodInfo,
	k int,
	jobBudget *jobSearchBudget,
	availableGenerator framework.ScenarioGeneratorRegistration,
	generatorBudget *generatorSearchBudget,
) *SearchResult {
	result := s.probeAtK(ssn, state, pendingJob, tasksToAllocate, k, jobBudget, availableGenerator, generatorBudget)
	if !resultSolved(result) {
		log.InfraLogger.V(5).Infof("No solution found for %d tasks out of %d tasks to allocate for %s",
			k, len(tasksToAllocate), pendingJob.Name)
		return result
	}
	solution := result.solution
	log.InfraLogger.V(5).Infof(
		"Scenario probed for %d tasks out of %d tasks to allocate for %s. Victims: %s",
		k, len(tasksToAllocate), pendingJob.Name, victimPrintingStruct{solution.victimsTasks})
	state.recordedVictimsTasks = solution.victimsTasks
	state.recordedVictimsJobs = solution.victimJobs
	state.saveSolverState = state.lastGeneratorCursor.Version != 0 && state.lastGeneratorName == availableGenerator.Name
	if solution.statement != nil {
		solution.statement.Discard()
	}
	return result
}

func (s *JobSolver) saveDiscardedProbeState(
	ssn *framework.Session,
	state *solvingState,
	pendingJob *podgroup_info.PodGroupInfo,
	tasksToAllocate []*pod_info.PodInfo,
	availableGenerator framework.ScenarioGeneratorRegistration,
	cursor framework.JobSolverCursor,
) {
	if ssn == nil || state == nil || cursor.ProbeK == 0 || int(cursor.ProbeK) > len(tasksToAllocate) {
		return
	}
	baseFeasible := make(map[string]*node_info.NodeInfo, len(s.feasibleNodes))
	for _, node := range s.feasibleNodes {
		baseFeasible[node.Name] = node
	}
	ctx := &SolveContext{
		Session:              ssn,
		ActionType:           s.actionType,
		PartialPendingJob:    getPartialJobRepresentative(pendingJob, tasksToAllocate[:cursor.ProbeK]),
		GenerateVictimsQueue: s.generateVictimsQueue,
		FeasibleNodes:        baseFeasible,
		ProbeK:               int(cursor.ProbeK),
	}
	saveScenarioCheckpointStateOnly(ctx, baseFeasible, availableGenerator.Name, state.lastGeneratorCursor.Version, cursor, state.recordedVictimsTasks)
	// The current search already owns the restored state. Avoid reloading the
	// checkpoint between probes; only a later session needs the stored form.
	state.checkpoint = &framework.ScenarioCheckpoint{StateOnly: true}
}

func (s *JobSolver) probeAtK(
	ssn *framework.Session,
	state *solvingState,
	pendingJob *podgroup_info.PodGroupInfo,
	tasksToAllocate []*pod_info.PodInfo,
	k int,
	jobBudget *jobSearchBudget,
	availableGenerator framework.ScenarioGeneratorRegistration,
	generatorBudget *generatorSearchBudget,
) *SearchResult {
	pendingTasks := tasksToAllocate[:k]
	partialPendingJob := getPartialJobRepresentative(pendingJob, pendingTasks)
	return s.solvePartialJob(ssn, state, partialPendingJob, jobBudget, availableGenerator, generatorBudget, k)
}

func (s *JobSolver) solvePartialJob(
	ssn *framework.Session, state *solvingState, partialPendingJob *podgroup_info.PodGroupInfo,
	jobBudget *jobSearchBudget, availableGenerator framework.ScenarioGeneratorRegistration,
	generatorBudget *generatorSearchBudget, probeK int,
) *SearchResult {
	if jobBudget == nil {
		jobBudget = newUnlimitedActionSearchBudget(s.actionType).BeginJob()
	}

	feasibleNodeMap := map[string]*node_info.NodeInfo{}
	for _, node := range s.feasibleNodes {
		feasibleNodeMap[node.Name] = node
	}
	baseFeasibleNodeMap := make(map[string]*node_info.NodeInfo, len(feasibleNodeMap))
	for name, node := range feasibleNodeMap {
		baseFeasibleNodeMap[name] = node
	}
	for _, task := range state.recordedVictimsTasks {
		node := ssn.ClusterInfo.Nodes[task.NodeName]
		feasibleNodeMap[task.NodeName] = node
	}

	solveCtx := &SolveContext{
		Session:              ssn,
		ActionType:           s.actionType,
		PartialPendingJob:    partialPendingJob,
		RecordedVictimsJobs:  state.recordedVictimsJobs,
		RecordedVictimsTasks: state.recordedVictimsTasks,
		GenerateVictimsQueue: s.generateVictimsQueue,
		FeasibleNodes:        feasibleNodeMap,
		ProbeK:               probeK,
	}
	checkpoint := state.checkpoint
	state.checkpoint = nil
	if checkpoint == nil {
		checkpoint, _ = loadScenarioCheckpoint(solveCtx, baseFeasibleNodeMap, availableGenerator.Name)
	}
	if checkpoint != nil {
		state.recordedVictimsTasks = solveCtx.RecordedVictimsTasks
		state.recordedVictimsJobs = solveCtx.RecordedVictimsJobs
		for _, task := range state.recordedVictimsTasks {
			if node := ssn.ClusterInfo.Nodes[task.NodeName]; node != nil {
				feasibleNodeMap[task.NodeName] = node
			}
		}
	}
	portfolio := newSingleGeneratorScenarioPortfolio(solveCtx, jobBudget, availableGenerator, state.generatorBudget)
	if checkpoint != nil && !checkpoint.StateOnly {
		restoreStarted := time.Now()
		err := portfolio.RestoreCurrent(checkpoint.GeneratorCursor)
		if err != nil {
			deleteScenarioCheckpoint(solveCtx)
			checkpoint = nil
			metrics.ObserveScenarioSearchCheckpointRestore(availableGenerator.Name, "failed", time.Since(restoreStarted))
		} else {
			// Restore consumes outer action/job time only. Discard the budget created
			// before restoration so the next generated candidate starts a fresh
			// generator budget.
			state.generatorBudget = jobBudget.BeginGenerator(availableGenerator.Name)
			portfolio.currentBudget = state.generatorBudget
			metrics.ObserveScenarioSearchCheckpointRestore(availableGenerator.Name, "restored", time.Since(restoreStarted))
		}
	} else if checkpoint != nil {
		state.generatorBudget = jobBudget.BeginGenerator(availableGenerator.Name)
		portfolio.currentBudget = state.generatorBudget
	}

	for {
		if jobBudget.Exhausted() {
			s.observeActionBudgetExhausted()
			return terminalSearchResult(SearchResultDeadlineExhausted, jobBudget.ReducedBudget())
		}
		scenarioToSolve := portfolio.Next()
		if scenarioToSolve == nil {
			break
		}
		generatorName := portfolio.CurrentGeneratorName()

		var fingerprint scenarioFingerprint
		if s.failedScenarios != nil {
			fingerprint = fingerprintScenario(scenarioToSolve)
			if s.failedScenarios.Has(fingerprint) {
				metrics.IncScenarioSearchScenario(s.actionType, generatorName, scenarioStateDuplicate)
				portfolio.ObserveCurrentAttempt(scenarioStateDuplicate)
				if cursor, ok := portfolio.CurrentCursor(); ok {
					saveScenarioCheckpoint(solveCtx, baseFeasibleNodeMap, generatorName, cursor, state.solverCursor, state.recordedVictimsTasks, SearchResultDeadlineExhausted)
				}
				continue
			}
		}

		validatorRejected := false
		scenarioSolver := newByPodSolver(feasibleNodeMap, s.solutionValidatorWithMetrics(generatorName, &validatorRejected),
			ssn.AllowConsolidatingReclaim(),
			s.actionType)

		log.InfraLogger.V(5).Infof("Trying to solve scenario: %s", scenarioToSolve)
		metrics.IncScenarioSimulatedByAction()
		metrics.IncScenarioSearchScenario(s.actionType, generatorName, "simulated")

		result := scenarioSolver.solve(ssn, scenarioToSolve)
		attemptResult := scenarioSearchResultUnsolved
		if validatorRejected {
			attemptResult = scenarioSearchResultValidatorRejected
		}
		if result.solved {
			if cursor, ok := portfolio.CurrentCursor(); ok {
				state.lastGeneratorCursor = cursor
				state.lastGeneratorName = generatorName
			}
			portfolio.ObserveCurrentAttempt(string(SearchResultSolved))
			deleteScenarioCheckpoint(solveCtx)
			return solvedSearchResult(result, jobBudget.ReducedBudget())
		}
		if s.failedScenarios != nil {
			s.failedScenarios.Insert(fingerprint)
		}
		portfolio.ObserveCurrentAttempt(attemptResult)
		if cursor, ok := portfolio.CurrentCursor(); ok {
			saveScenarioCheckpoint(solveCtx, baseFeasibleNodeMap, generatorName, cursor, state.solverCursor, state.recordedVictimsTasks, SearchResultDeadlineExhausted)
		}
	}

	result := terminalSearchResult(portfolio.StopReason(), jobBudget.ReducedBudget())
	if result.Reason() != SearchResultDeadlineExhausted {
		deleteScenarioCheckpoint(solveCtx)
	}
	return result
}

func (s *JobSolver) observeActionBudgetExhausted() {
	if s.actionBudget != nil && s.actionBudget.Exhausted() {
		metrics.IncScenarioSearchActionBudgetExhausted(s.actionType)
	}
}

func (s *JobSolver) solutionValidatorWithMetrics(generator string, rejected *bool) SolutionValidator {
	if s.solutionValidator == nil {
		return nil
	}
	return func(scenario api.ScenarioInfo) bool {
		valid := s.solutionValidator(scenario)
		if !valid {
			if rejected != nil {
				*rejected = true
			}
			metrics.IncScenarioSearchScenario(s.actionType, generator, "validator_rejected")
		}
		return valid
	}
}

func shouldStopSearch(result *SearchResult) bool {
	switch result.Reason() {
	case SearchResultDeadlineExhausted, SearchResultNotAttempted, SearchResultNoGenerator:
		return true
	default:
		return false
	}
}

func resultSolved(result *SearchResult) bool {
	return result != nil && result.Reason() == SearchResultSolved &&
		result.solution != nil && result.solution.solved
}

func getPartialJobRepresentative(
	job *podgroup_info.PodGroupInfo, pendingTasks []*pod_info.PodInfo) *podgroup_info.PodGroupInfo {
	representativeTasks := append(job.GetAllAllocatedPods(), pendingTasks...)
	jobRepresentative := job.CloneWithTasks(representativeTasks)

	adjustSubGroupsMinAvailable(jobRepresentative)
	adjustSubGroupsMinSubGroup(jobRepresentative.RootSubGroupSet)

	return jobRepresentative
}

// adjustSubGroupsMinAvailable adjusts the minAvailable of the subGroups of the job representative to the number of tasks in the job representative.
// This is done to ensure that the job representative has the correct minAvailable for each subGroup,
// taking into account that the representative is a PARTIAL clone of the original job.
func adjustSubGroupsMinAvailable(jobRepresentative *podgroup_info.PodGroupInfo) {
	subGroupsPodCount := map[string]int{}
	for _, pendingTask := range jobRepresentative.GetAllPodsMap() {
		if _, found := jobRepresentative.GetAllPodSets()[pendingTask.SubGroupName]; found {
			subGroupsPodCount[pendingTask.SubGroupName] += 1
		} else {
			subGroupsPodCount[podgroup_info.DefaultSubGroup] += 1
		}
	}
	for subGroupName, podCount := range subGroupsPodCount {
		subGroup, found := jobRepresentative.GetAllPodSets()[subGroupName]
		if !found {
			log.InfraLogger.V(2).Warnf("Couldn't find SubGroup with name %s for job %s",
				subGroupName, jobRepresentative.NamespacedName,
			)
			continue
		}
		minAvailable := min(subGroup.GetMinAvailable(), int32(podCount))
		subGroup.SetMinAvailable(minAvailable)
	}
}

// adjustSubGroupsMinSubGroup recursively walks the SubGroupSet tree and sets each node's
// minSubGroup to the number of direct members that have tasks in the partial clone.
// This mirrors the minAvailable adjustment done on PodSets: the clone must only require
// what it actually contains, so that gang-satisfaction checks work correctly on the partial job.
// Returns true if this node contains any tasks.
func adjustSubGroupsMinSubGroup(sgs *subgroup_info.SubGroupSet) bool {
	nonEmptyMembers := int32(0)
	for _, podSet := range sgs.GetDirectPodSets() {
		if len(podSet.GetPodInfos()) > 0 {
			nonEmptyMembers++
		}
	}
	for _, subGroupSet := range sgs.GetDirectSubgroupsSets() {
		if adjustSubGroupsMinSubGroup(subGroupSet) {
			nonEmptyMembers++
		}
	}
	if minSubGroup := sgs.GetMinSubGroup(); minSubGroup != nil {
		minSubGroup := min(*minSubGroup, nonEmptyMembers)
		sgs.SetMinSubGroup(&minSubGroup)
	}
	return nonEmptyMembers > 0
}

func calcVictimNames(victimsTasks []*pod_info.PodInfo) []string {
	var names []string
	for _, victimTask := range victimsTasks {
		names = append(names,
			fmt.Sprintf("<%s/%s>", victimTask.Namespace, victimTask.Name))
	}
	return names
}

type victimPrintingStruct struct {
	victims []*pod_info.PodInfo
}

func (v victimPrintingStruct) String() string {
	if len(v.victims) == 0 {
		return ""
	}
	stringBuilder := strings.Builder{}

	stringBuilder.WriteString(v.victims[0].Namespace)
	stringBuilder.WriteString("/")
	stringBuilder.WriteString(v.victims[0].Name)

	for _, victimTask := range v.victims[1:] {
		stringBuilder.WriteString(", ")
		stringBuilder.WriteString(victimTask.Namespace)
		stringBuilder.WriteString("/")
		stringBuilder.WriteString(victimTask.Name)
	}

	return stringBuilder.String()
}
