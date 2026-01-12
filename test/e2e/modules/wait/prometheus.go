/*
Copyright 2025 NVIDIA CORPORATION
SPDX-License-Identifier: Apache-2.0
*/
package wait

import (
	"context"
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/watch"
	runtimeClient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/NVIDIA/KAI-scheduler/test/e2e/modules/constant"
	"github.com/NVIDIA/KAI-scheduler/test/e2e/modules/resources/rd"
	"github.com/NVIDIA/KAI-scheduler/test/e2e/modules/wait/watcher"
)

const (
	prometheusAppLabel = "prometheus"
	prometheusPodName  = "prometheus-prometheus-0"
)

// ForPrometheusReady waits for the Prometheus pod to be running and ready
func ForPrometheusReady(ctx context.Context, client runtimeClient.WithWatch, timeout time.Duration) {
	ctxWithTimeout, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	condition := func(event watch.Event) bool {
		podListObj, ok := event.Object.(*v1.PodList)
		if !ok {
			return false
		}

		// Look for the prometheus pod
		for _, pod := range podListObj.Items {
			if pod.Name == prometheusPodName {
				return rd.IsPodReady(&pod)
			}
		}
		return false
	}

	pw := watcher.NewGenericWatcher[v1.PodList](client, watcher.CheckCondition(condition),
		runtimeClient.InNamespace(constant.SystemPodsNamespace),
		runtimeClient.MatchingLabels{"app.kubernetes.io/name": prometheusAppLabel})

	if !watcher.ForEvent(ctxWithTimeout, client, pw) {
		Fail(fmt.Sprintf("Timed out waiting for Prometheus pod to be ready (timeout: %v)", timeout))
	}
}

// ForPrometheusDeleted waits for the Prometheus pod to be deleted
func ForPrometheusDeleted(ctx context.Context, client runtimeClient.WithWatch, timeout time.Duration) {
	ctxWithTimeout, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	condition := func(event watch.Event) bool {
		podListObj, ok := event.Object.(*v1.PodList)
		if !ok {
			return false
		}

		// Check if prometheus pod exists
		for _, pod := range podListObj.Items {
			if pod.Name == prometheusPodName {
				return false // Pod still exists
			}
		}
		return true // No prometheus pod found
	}

	pw := watcher.NewGenericWatcher[v1.PodList](client, watcher.CheckCondition(condition),
		runtimeClient.InNamespace(constant.SystemPodsNamespace),
		runtimeClient.MatchingLabels{"app.kubernetes.io/name": prometheusAppLabel})

	if !watcher.ForEvent(ctxWithTimeout, client, pw) {
		Fail(fmt.Sprintf("Timed out waiting for Prometheus pod to be deleted (timeout: %v)", timeout))
	}
}
