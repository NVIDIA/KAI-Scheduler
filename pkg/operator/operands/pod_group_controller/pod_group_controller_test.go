// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package pod_group_controller

import (
	"context"
	"testing"

	"golang.org/x/exp/maps"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	kaiv1 "github.com/NVIDIA/KAI-scheduler/pkg/apis/kai/v1"
	test_utils "github.com/NVIDIA/KAI-scheduler/pkg/operator/operands/common/test_utils"

	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestPodGrouper(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "PodGrouper operand Suite")
}

var _ = Describe("PodGrouper", func() {
	Describe("DesiredState", func() {
		var (
			fakeKubeClient client.Client
			pg             *PodGroupController
			kaiConfig      *kaiv1.Config
		)
		BeforeEach(func(ctx context.Context) {
			fakeKubeClient = fake.NewFakeClient()
			pg = &PodGroupController{}
			kaiConfig = kaiConfigForPodGrouper()
		})

		Context("Deployment", func() {
			It("should return a Deployment in the objects list", func(ctx context.Context) {
				objects, err := pg.DesiredState(ctx, fakeKubeClient, kaiConfig)
				Expect(err).To(BeNil())
				Expect(len(objects)).To(BeNumerically(">", 1))

				deploymentT := test_utils.FindTypeInObjects[*appsv1.Deployment](objects)
				Expect(deploymentT).NotTo(BeNil())
				deployment := *deploymentT
				Expect(deployment).NotTo(BeNil())
				Expect(deployment.Name).To(Equal(deploymentName))
			})

			It("the deployment should keep labels from existing deployment", func(ctx context.Context) {
				objects, err := pg.DesiredState(ctx, fakeKubeClient, kaiConfig)
				Expect(err).To(BeNil())

				deploymentT := test_utils.FindTypeInObjects[*appsv1.Deployment](objects)
				Expect(deploymentT).NotTo(BeNil())
				deployment := *deploymentT
				maps.Copy(deployment.Labels, map[string]string{
					"foo": "bar",
				})
				maps.Copy(deployment.Spec.Template.Labels, map[string]string{
					"kai": "scheduler",
				})
				Expect(fakeKubeClient.Create(ctx, deployment)).To(Succeed())

				objects, err = pg.DesiredState(ctx, fakeKubeClient, kaiConfig)
				Expect(err).To(BeNil())

				deploymentT = test_utils.FindTypeInObjects[*appsv1.Deployment](objects)
				Expect(deploymentT).NotTo(BeNil())
				deployment = *deploymentT
				Expect(deployment.Labels).To(HaveKeyWithValue("foo", "bar"))
				Expect(deployment.Spec.Template.Labels).To(HaveKeyWithValue("kai", "scheduler"))
			})
		})
	})
})

func kaiConfigForPodGrouper() *kaiv1.Config {
	kaiConfig := &kaiv1.Config{}
	kaiConfig.Spec.SetDefaultsWhereNeeded()
	kaiConfig.Spec.PodGroupController.Service.Enabled = ptr.To(true)

	return kaiConfig
}
