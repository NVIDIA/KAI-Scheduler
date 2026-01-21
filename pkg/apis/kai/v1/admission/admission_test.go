// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package admission

import (
	"context"
	"testing"

	"github.com/NVIDIA/KAI-scheduler/pkg/common/constants"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/utils/ptr"
)

func TestAdmission(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Admission type suite")
}

var _ = Describe("Admission", func() {
	It("Set Defaults", func(ctx context.Context) {
		admission := &Admission{}
		var replicaCount int32
		replicaCount = 1
		admission.SetDefaultsWhereNeeded(&replicaCount)
		Expect(*admission.Service.Enabled).To(Equal(true))
		Expect(*admission.Service.Image.Name).To(Equal("admission"))
		Expect(*admission.Replicas).To(Equal(int32(1)))
		Expect(*admission.GPUPodRuntimeClassName).To(Equal(constants.DefaultRuntimeClassName))
	})

	It("Set Defaults with replica count", func(ctx context.Context) {
		admission := &Admission{}
		var replicaCount int32
		replicaCount = 3
		admission.SetDefaultsWhereNeeded(&replicaCount)
		Expect(*admission.Replicas).To(Equal(int32(3)))
	})

	It("Set Defaults for Autoscaling", func(ctx context.Context) {
		admission := &Admission{}
		var replicaCount int32
		replicaCount = 1
		admission.SetDefaultsWhereNeeded(&replicaCount)

		Expect(admission.Autoscaling).NotTo(BeNil())
		Expect(*admission.Autoscaling.Enabled).To(Equal(false))
		Expect(*admission.Autoscaling.MinReplicas).To(Equal(int32(1)))
		Expect(*admission.Autoscaling.MaxReplicas).To(Equal(int32(5)))
		Expect(*admission.Autoscaling.AverageRequestsPerPod).To(Equal(int32(100)))
	})

	It("Set Autoscaling enabled with custom values", func(ctx context.Context) {
		admission := &Admission{
			Autoscaling: &Autoscaling{
				Enabled:               ptr.To(true),
				MinReplicas:           ptr.To(int32(2)),
				MaxReplicas:           ptr.To(int32(10)),
				AverageRequestsPerPod: ptr.To(int32(150)),
			},
		}
		var replicaCount int32
		replicaCount = 1
		admission.SetDefaultsWhereNeeded(&replicaCount)

		Expect(*admission.Autoscaling.Enabled).To(Equal(true))
		Expect(*admission.Autoscaling.MinReplicas).To(Equal(int32(2)))
		Expect(*admission.Autoscaling.MaxReplicas).To(Equal(int32(10)))
		Expect(*admission.Autoscaling.AverageRequestsPerPod).To(Equal(int32(150)))
	})

	It("Autoscaling disabled keeps replicas from config", func(ctx context.Context) {
		admission := &Admission{
			Replicas: ptr.To(int32(3)),
			Autoscaling: &Autoscaling{
				Enabled: ptr.To(false),
			},
		}
		var replicaCount int32
		replicaCount = 1
		admission.SetDefaultsWhereNeeded(&replicaCount)

		Expect(*admission.Autoscaling.Enabled).To(Equal(false))
		Expect(*admission.Replicas).To(Equal(int32(3)))
	})
})

var _ = Describe("Webhook", func() {
	It("Set Defaults", func(ctx context.Context) {
		webhook := &Webhook{}
		webhook.SetDefaultsWhereNeeded()

		Expect(*webhook.Port).To(Equal(443))
		Expect(*webhook.TargetPort).To(Equal(9443))
		Expect(*webhook.ProbePort).To(Equal(8081))
		Expect(*webhook.MetricsPort).To(Equal(8080))
	})

	It("Preserves custom values", func(ctx context.Context) {
		webhook := &Webhook{
			Port:        ptr.To(8443),
			TargetPort:  ptr.To(8888),
			ProbePort:   ptr.To(8082),
			MetricsPort: ptr.To(9090),
		}
		webhook.SetDefaultsWhereNeeded()

		Expect(*webhook.Port).To(Equal(8443))
		Expect(*webhook.TargetPort).To(Equal(8888))
		Expect(*webhook.ProbePort).To(Equal(8082))
		Expect(*webhook.MetricsPort).To(Equal(9090))
	})
})

var _ = Describe("Autoscaling", func() {
	It("Set Defaults", func(ctx context.Context) {
		autoscaling := &Autoscaling{}
		autoscaling.SetDefaultsWhereNeeded()

		Expect(*autoscaling.Enabled).To(Equal(false))
		Expect(*autoscaling.MinReplicas).To(Equal(int32(1)))
		Expect(*autoscaling.MaxReplicas).To(Equal(int32(5)))
		Expect(*autoscaling.AverageRequestsPerPod).To(Equal(int32(100)))
	})

	It("Preserves custom values", func(ctx context.Context) {
		autoscaling := &Autoscaling{
			Enabled:               ptr.To(true),
			MinReplicas:           ptr.To(int32(3)),
			MaxReplicas:           ptr.To(int32(20)),
			AverageRequestsPerPod: ptr.To(int32(200)),
		}
		autoscaling.SetDefaultsWhereNeeded()

		Expect(*autoscaling.Enabled).To(Equal(true))
		Expect(*autoscaling.MinReplicas).To(Equal(int32(3)))
		Expect(*autoscaling.MaxReplicas).To(Equal(int32(20)))
		Expect(*autoscaling.AverageRequestsPerPod).To(Equal(int32(200)))
	})
})
