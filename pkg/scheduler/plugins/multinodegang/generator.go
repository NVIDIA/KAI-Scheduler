// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package multinodegang

import (
	"encoding/binary"
	"fmt"

	"github.com/kai-scheduler/KAI-scheduler/pkg/common/constants"
	"github.com/kai-scheduler/KAI-scheduler/pkg/scheduler/actions/common/solvers"
	"github.com/kai-scheduler/KAI-scheduler/pkg/scheduler/actions/common/solvers/scenario"
	"github.com/kai-scheduler/KAI-scheduler/pkg/scheduler/api"
	"github.com/kai-scheduler/KAI-scheduler/pkg/scheduler/framework"
)

type multiNodeGangGenerator struct {
	builder *solvers.PodAccumulatedScenarioBuilder
	first   bool
	emitted bool
}

func NewMultiNodeGangGenerator(ctx framework.ScenarioGeneratorContext) framework.ScenarioGenerator {
	solveCtx, generateVictimsQueue, ok := solvers.ValidateScenarioGeneratorContext(ctx)
	if !ok {
		return nil
	}
	victimsQueue := generateVictimsQueue()
	if victimsQueue == nil {
		return nil
	}

	return &multiNodeGangGenerator{
		builder: solvers.NewPodAccumulatedScenarioBuilder(
			solveCtx.Session,
			solveCtx.PartialPendingJob,
			solveCtx.RecordedVictimsJobs,
			victimsQueue,
			solveCtx.FeasibleNodes,
		),
		first: true,
	}
}

func (g *multiNodeGangGenerator) Name() string {
	return constants.GeneratorMultiNodeGang
}

func (g *multiNodeGangGenerator) Next() api.ScenarioInfo {
	var sn *scenario.ByNodeScenario
	if g.first {
		g.first = false
		sn = g.builder.GetValidScenario()
	} else {
		sn = g.builder.GetNextScenario()
	}
	if sn == nil {
		return nil
	}
	g.emitted = true
	return sn
}

const (
	multiNodeGangCursorVersion = 1
	multiNodeGangRecordedOnly  = 1
	multiNodeGangSubEmitter    = 2
)

func (g *multiNodeGangGenerator) Cursor() (framework.ScenarioGeneratorCursor, bool) {
	if g == nil || g.builder == nil || !g.emitted {
		return framework.ScenarioGeneratorCursor{}, false
	}
	var cursor framework.ScenarioGeneratorCursor
	cursor.Version = multiNodeGangCursorVersion
	binary.BigEndian.PutUint64(cursor.Data[1:9], g.builder.VictimQueuePops())
	if nextK, dK, active := g.builder.SubScenarioCursor(); active {
		cursor.Data[0] = multiNodeGangSubEmitter
		binary.BigEndian.PutUint32(cursor.Data[9:13], uint32(nextK))
		binary.BigEndian.PutUint32(cursor.Data[13:17], uint32(dK))
	} else {
		cursor.Data[0] = multiNodeGangRecordedOnly
	}
	return cursor, true
}

func (g *multiNodeGangGenerator) Restore(cursor framework.ScenarioGeneratorCursor) error {
	if g == nil || g.builder == nil || cursor.Version != multiNodeGangCursorVersion {
		return fmt.Errorf("invalid MultiNodeGang cursor")
	}
	for _, value := range cursor.Data[17:] {
		if value != 0 {
			return fmt.Errorf("invalid MultiNodeGang cursor padding")
		}
	}
	if err := g.builder.RestoreVictimQueuePops(binary.BigEndian.Uint64(cursor.Data[1:9])); err != nil {
		return err
	}
	switch cursor.Data[0] {
	case multiNodeGangRecordedOnly:
		base := g.builder.CurrentValidAccumulatedScenario()
		if base == nil || len(base.PotentialVictimsTasks()) != 0 || binary.BigEndian.Uint32(cursor.Data[9:13]) != 0 || binary.BigEndian.Uint32(cursor.Data[13:17]) != 0 {
			return fmt.Errorf("invalid MultiNodeGang recorded-only cursor")
		}
	case multiNodeGangSubEmitter:
		nextK := binary.BigEndian.Uint32(cursor.Data[9:13])
		dK := binary.BigEndian.Uint32(cursor.Data[13:17])
		if dK == 0 || uint64(nextK) > uint64(^uint(0)>>1) || uint64(dK) > uint64(^uint(0)>>1) {
			return fmt.Errorf("invalid MultiNodeGang sub-emitter cursor")
		}
		if err := g.builder.RestoreSubScenarioEmitter(int(nextK), int(dK)); err != nil {
			return err
		}
	default:
		return fmt.Errorf("invalid MultiNodeGang cursor phase")
	}
	g.first = false
	g.emitted = true
	return nil
}
