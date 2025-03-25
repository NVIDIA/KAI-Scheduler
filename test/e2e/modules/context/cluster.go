/*
Copyright 2025 NVIDIA CORPORATION
SPDX-License-Identifier: Apache-2.0
*/
package context

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	v2 "github.com/NVIDIA/KAI-scheduler/pkg/apis/scheduling/v2"
	"github.com/NVIDIA/KAI-scheduler/test/e2e/modules/resources/rd"
	"github.com/NVIDIA/KAI-scheduler/test/e2e/modules/resources/rd/queue"
)

func (tc *TestContext) createClusterQueues(ctx context.Context) error {
	for _, testQueue := range tc.Queues {
		err := createQueueContext(ctx, testQueue)
		if err != nil {
			return err
		}
	}
	return nil
}

func createQueueContext(ctx context.Context, q *v2.Queue) error {
	_, err := queue.Create(kubeAiSchedClientset, ctx, q, metav1.CreateOptions{})
	if err != nil {
		return err
	}

	namespaceName := queue.GetConnectedNamespaceToQueue(q)
	ns := rd.CreateNamespaceObject(namespaceName, q.Name)
	_, err = kubeClientset.
		CoreV1().
		Namespaces().
		Create(ctx, ns, metav1.CreateOptions{})
	if err != nil {
		return err
	}

	// TODO: add RBAC role bindings
	// TODO: patch the namespace to add appropriate secret to the service account

	return nil
}
