/*
Copyright 2025 NVIDIA CORPORATION
SPDX-License-Identifier: Apache-2.0
*/
package watcher

import (
	"context"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"
	runtimeClient "sigs.k8s.io/controller-runtime/pkg/client"
)

type PodsWatcher struct {
	client       runtimeClient.WithWatch
	condition    CheckCondition
	namespace    string
	satisfiedMap map[string]bool
	minRequired  int
}

func NewPodsWatcher(
	client runtimeClient.WithWatch, condition CheckCondition, namespace string, pods []*corev1.Pod, minRequired int,
) *PodsWatcher {
	return &PodsWatcher{
		client,
		condition,
		namespace,
		initSatisfiedPodsMap(pods),
		minRequired,
	}
}

func initSatisfiedPodsMap(pods []*corev1.Pod) map[string]bool {
	satisfiedPodsMap := map[string]bool{}
	for _, pod := range pods {
		podKey := types.NamespacedName{Namespace: pod.Namespace, Name: pod.Name}
		satisfiedPodsMap[podKey.String()] = false
	}
	return satisfiedPodsMap
}

func (w *PodsWatcher) watch(ctx context.Context) watch.Interface {
	podsList := &corev1.PodList{}
	watcher, err := w.client.Watch(ctx, podsList, runtimeClient.InNamespace(w.namespace))
	if err != nil {
		Fail(fmt.Sprintf("Failed to create watcher for pods: %v", err))
	}
	return watcher
}

func (w *PodsWatcher) sync(ctx context.Context) {
	podsList := &corev1.PodList{}
	err := w.client.List(ctx, podsList, runtimeClient.InNamespace(w.namespace))
	if err != nil {
		Fail(fmt.Sprintf("Failed to list pods: %v", err))
	}

	for _, pod := range podsList.Items {
		podKey := types.NamespacedName{Namespace: pod.Namespace, Name: pod.Name}
		alreadySatisfied, relevant := w.satisfiedMap[podKey.String()]
		if !relevant || alreadySatisfied {
			continue
		}
		w.satisfiedMap[podKey.String()] = w.condition(watch.Event{Object: &pod})
	}
}

func (w *PodsWatcher) processEvent(_ context.Context, event watch.Event) {
	pod, ok := event.Object.(*corev1.Pod)
	if !ok {
		return
	}

	podKey := types.NamespacedName{Namespace: pod.Namespace, Name: pod.Name}
	alreadySatisfied, relevant := w.satisfiedMap[podKey.String()]
	if !relevant || alreadySatisfied {
		return
	}
	w.satisfiedMap[podKey.String()] = w.condition(event)
}

func (w *PodsWatcher) satisfied() bool {
	counter := 0
	for _, satisfied := range w.satisfiedMap {
		if satisfied {
			counter += 1
		}
	}
	return counter >= w.minRequired
}
