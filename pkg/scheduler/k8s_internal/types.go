// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package k8s_internal

import (
	"context"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	k8sframework "k8s.io/kubernetes/pkg/scheduler/framework"
)

type NodeFilter interface {
	Filter(ctx context.Context, state *k8sframework.CycleState,
		pod *v1.Pod, nodeInfo *k8sframework.NodeInfo) *k8sframework.Status
}

type NodePreFilter interface {
	PreFilter(ctx context.Context,
		cycleState *k8sframework.CycleState, pod *v1.Pod) (*k8sframework.PreFilterResult, *k8sframework.Status)
}

type NodeScorer interface {
	Score(ctx context.Context, cycleState *k8sframework.CycleState, pod *v1.Pod, nodeName string) (int64, *k8sframework.Status)
}

type ExtendedNodeScorer interface {
	NodeScorer
	PreScore(context.Context, *k8sframework.CycleState, *v1.Pod, []*k8sframework.NodeInfo) *k8sframework.Status
	// PreScore(pCtx context.Context, cycleState *framework.CycleState, pod *v1.Pod, nodes []*framework.NodeInfo) *framework.Status
}

type FitPredicateRequired func(pod *v1.Pod) bool
type FitPredicatePreFilter func(pod *v1.Pod) (sets.Set[string], *k8sframework.Status)
type FitPredicateFilter func(pod *v1.Pod, nodeInfo *k8sframework.NodeInfo) (bool, []string, error)

type SessionPredicate struct {
	Name                string
	IsPreFilterRequired FitPredicateRequired
	PreFilter           FitPredicatePreFilter
	IsFilterRequired    FitPredicateRequired
	Filter              FitPredicateFilter
}

type PreScoreFn func(pod *v1.Pod, fittingNodes []*k8sframework.NodeInfo) *k8sframework.Status
type ScorePredicate func(pod *v1.Pod, nodeInfo *k8sframework.NodeInfo) (int64, []string, error)

type PredicateName string
type SessionPredicates map[PredicateName]SessionPredicate

type SessionScoreFns struct {
	PrePodAffinity PreScoreFn
	PodAffinity    ScorePredicate
}

type SessionState *k8sframework.CycleState

type SessionStateProvider interface {
	GetSessionStateForResource(podUID types.UID) SessionState
}
