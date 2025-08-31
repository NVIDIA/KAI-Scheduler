// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package status_reconciler

import (
	"context"
	"fmt"
	"testing"

	kaiv1 "github.com/NVIDIA/KAI-scheduler/pkg/apis/kai/v1"
	"github.com/NVIDIA/KAI-scheduler/pkg/common/constants"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestStatusController(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Status Controller Suite")
}

var _ = Describe("Status Controller", func() {
	var (
		fakeClient client.Client
		kaiConfig  *kaiv1.Config
	)
	BeforeEach(func() {
		scheme := scheme.Scheme
		kaiv1.AddToScheme(scheme)

		kaiConfig = &kaiv1.Config{
			ObjectMeta: metav1.ObjectMeta{
				Name:       constants.DefaultKAIConfigSingeltonInstanceName,
				Generation: 1,
			},
		}

		fakeClient = fake.NewClientBuilder().
			WithScheme(scheme).
			WithStatusSubresource(&kaiv1.Config{}).
			WithObjects(kaiConfig).
			Build()
	})

	Describe("updateStartReconcileStatus", func() {
		It("should set the reconciling condition", func() {
			fakeDeployable := &fakeDeployable{
				isDeployed:     true,
				isDeployedErr:  false,
				isAvailable:    true,
				isAvailableErr: false,
			}

			statusReconciler := &StatusReconciler{
				deployable: fakeDeployable,
				Client:     fakeClient,
			}

			err := statusReconciler.UpdateStartReconcileStatus(
				context.TODO(), &KAIConfigWithStatusWrapper{Config: kaiConfig},
			)
			Expect(err).ToNot(HaveOccurred())

			Expect(fakeClient.Get(context.TODO(), client.ObjectKeyFromObject(kaiConfig), kaiConfig)).To(Succeed())
			found := false
			for _, condition := range kaiConfig.Status.Conditions {
				if condition.Type == string(kaiv1.ConditionTypeReconciling) &&
					condition.Status == metav1.ConditionTrue &&
					condition.ObservedGeneration == kaiConfig.Generation &&
					condition.Reason == string(kaiv1.Reconciled) {
					found = true
				}
			}

			Expect(found).To(BeTrue())
		})
	})

	Describe("reconcileStatus", func() {
		DescribeTable("should set the reconciling condition", func(
			isDeployErr bool, isDeployed bool, isAvailableErr bool, isAvailable bool) {
			fakeDeployable := &fakeDeployable{
				isDeployed:     isDeployed,
				isDeployedErr:  isDeployErr,
				isAvailable:    isAvailable,
				isAvailableErr: isAvailableErr,
			}

			statusReconciler := &StatusReconciler{
				deployable: fakeDeployable,
				Client:     fakeClient,
			}
			err := statusReconciler.ReconcileStatus(
				context.TODO(), &KAIConfigWithStatusWrapper{Config: kaiConfig},
			)
			Expect(err).ToNot(HaveOccurred())

			Expect(fakeClient.Get(context.TODO(), client.ObjectKeyFromObject(kaiConfig), kaiConfig)).To(Succeed())

			isDeployed = isDeployed && !isDeployErr
			isAvailable = isAvailable && !isAvailableErr
			Expect(checkIsDeployed(kaiConfig.Status.Conditions)).To(Equal(isDeployed))
			Expect(checkIsAvailable(kaiConfig.Status.Conditions)).To(Equal(isAvailable))
		},
			Entry("no errors, all set", false, true, false, true),
			Entry("no errors, deployed not avaialbe", false, true, false, false),
			Entry("no errors, not deployed not avaialbe", false, true, false, false),
			Entry("error isDeployed", true, false, false, false),
			Entry("error isAvailable", false, false, true, false),
		)
	})
})

func checkIsDeployed(conditions []metav1.Condition) bool {
	isDeployedCondition := getConditionByType(conditions, string(kaiv1.ConditionTypeDeployed))
	if isDeployedCondition == nil {
		return false
	}
	return isDeployedCondition.Status == metav1.ConditionTrue
}

func checkIsAvailable(conditions []metav1.Condition) bool {
	isAvailableCondition := getConditionByType(conditions, string(kaiv1.ConditionTypeAvailable))
	if isAvailableCondition == nil {
		return false
	}
	return isAvailableCondition.Status == metav1.ConditionTrue
}

func getConditionByType(conditions []metav1.Condition, conditionType string) *metav1.Condition {
	for _, condition := range conditions {
		if condition.Type == conditionType {
			return &condition
		}
	}
	return nil
}

type fakeDeployable struct {
	failDeploy     bool
	isDeployed     bool
	isDeployedErr  bool
	isAvailable    bool
	isAvailableErr bool
}

func (f *fakeDeployable) Deploy(ctx context.Context, runtimeClient client.Client, kaiConfig *kaiv1.Config, reconciler client.Object) error {
	if f.failDeploy {
		return fmt.Errorf("Deploy failed")
	}
	return nil
}

func (f *fakeDeployable) IsDeployed(ctx context.Context, runtimeClient client.Reader) (bool, error) {
	if f.isDeployedErr {
		return false, fmt.Errorf("IsDeployed failed")
	}
	return f.isDeployed, nil
}

func (f *fakeDeployable) IsAvailable(ctx context.Context, runtimeClient client.Reader) (bool, error) {
	if f.isAvailableErr {
		return false, fmt.Errorf("IsAvailable failed")
	}
	return f.isAvailable, nil
}

func (f *fakeDeployable) Name() string {
	return "fakeDeployable"
}
