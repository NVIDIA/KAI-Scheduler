/*
Copyright 2025 NVIDIA CORPORATION
SPDX-License-Identifier: Apache-2.0
*/
package watcher

import (
	"context"

	"k8s.io/apimachinery/pkg/watch"
	runtimeClient "sigs.k8s.io/controller-runtime/pkg/client"
)

type PollCondition func() bool

type PollWatcher struct {
	condition   PollCondition
	isSatisfied bool
}

func NewPollWatcher(client runtimeClient.WithWatch, condition PollCondition) *PollWatcher {
	return &PollWatcher{
		condition,
		false,
	}
}

func (w *PollWatcher) watch(context.Context) watch.Interface {
	return watch.NewEmptyWatch()
}

func (w *PollWatcher) sync(context.Context) {
	w.isSatisfied = w.condition()
}

func (w *PollWatcher) processEvent(ctx context.Context, _ watch.Event) {
	w.sync(ctx)
}

func (w *PollWatcher) satisfied() bool {
	return w.isSatisfied
}
