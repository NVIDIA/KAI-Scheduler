/*
Copyright 2025 NVIDIA CORPORATION
SPDX-License-Identifier: Apache-2.0
*/
package watcher

import (
	"context"
	"fmt"
	"reflect"

	. "github.com/onsi/ginkgo/v2"
	"k8s.io/apimachinery/pkg/watch"
	runtimeClient "sigs.k8s.io/controller-runtime/pkg/client"
)

type GenericWatcher[T any, PT interface {
	*T
	runtimeClient.ObjectList
}] struct {
	client      runtimeClient.WithWatch
	condition   CheckCondition
	listOptions []runtimeClient.ListOption
	isSatisfied bool
}

func NewGenericWatcher[T any, PT interface {
	*T
	runtimeClient.ObjectList
}](
	client runtimeClient.WithWatch, condition CheckCondition, listOptions ...runtimeClient.ListOption,
) *GenericWatcher[T, PT] {
	return &GenericWatcher[T, PT]{
		client,
		condition,
		listOptions,
		false,
	}
}

func (w *GenericWatcher[T, PT]) watch(ctx context.Context) watch.Interface {
	var objectList T
	pObjectList := PT(&objectList)
	watcher, err := w.client.Watch(ctx, pObjectList, w.listOptions...)
	if err != nil {
		Fail(fmt.Sprintf("Failed to create watcher for type: %v. err: %v", reflect.TypeOf(pObjectList), err))
	}
	return watcher
}

func (w *GenericWatcher[T, PT]) sync(ctx context.Context) {
	var objectList T
	pObjectList := PT(&objectList)
	err := w.client.List(ctx, pObjectList, w.listOptions...)
	if err != nil {
		Fail(fmt.Sprintf("Failed to list type: %v, err: %v", reflect.TypeOf(pObjectList), err))
	}
	w.isSatisfied = w.condition(watch.Event{Object: pObjectList})
}

func (w *GenericWatcher[T, PT]) processEvent(ctx context.Context, _ watch.Event) {
	w.sync(ctx)
}

func (w *GenericWatcher[T, PT]) satisfied() bool {
	return w.isSatisfied
}
