// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package status_updater

import (
	"strconv"
	"sync"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	fakecorev1 "k8s.io/client-go/kubernetes/typed/core/v1/fake"
	faketesting "k8s.io/client-go/testing"
	"k8s.io/client-go/tools/record"

	kubeaischedfake "github.com/NVIDIA/KAI-scheduler/pkg/apis/client/clientset/versioned/fake"
)

const (
	nodePoolLabelKey = "kai.scheduler/node-pool"
)

func TestConcurrency(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Status Updater Concurrency Suite")
}

var _ = Describe("Status Updater Concurrency - large scale: increase queue size", func() {
	var (
		kubeClient        *fake.Clientset
		kubeAiSchedClient *kubeaischedfake.Clientset
		statusUpdater     *defaultStatusUpdater
	)
	BeforeEach(func() {
		kubeClient = fake.NewSimpleClientset()
		kubeAiSchedClient = kubeaischedfake.NewSimpleClientset()
		recorder := record.NewFakeRecorder(100)
		statusUpdater = New(kubeClient, kubeAiSchedClient, recorder, 4, false,
			nodePoolLabelKey)
	})

	It("should increase queue size", func() {
		wg := sync.WaitGroup{}
		signalCh := make(chan struct{})
		kubeClient.CoreV1().(*fakecorev1.FakeCoreV1).PrependReactor("patch", "pods", func(action faketesting.Action) (handled bool, ret runtime.Object, err error) {
			<-signalCh
			wg.Done()
			return true, nil, nil
		})
		stopCh := make(chan struct{})
		statusUpdater.Run(stopCh)
		defer close(stopCh)

		for i := 0; i < 2000; i++ {
			wg.Add(1)
			statusUpdater.updatePodCondition(
				&v1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "pod-" + strconv.Itoa(i),
						Namespace: "default",
					},
				},
				&v1.PodCondition{
					Type:   v1.PodScheduled,
					Status: v1.ConditionTrue,
				},
			)
		}

		close(signalCh)
		wg.Wait()
	})
})
