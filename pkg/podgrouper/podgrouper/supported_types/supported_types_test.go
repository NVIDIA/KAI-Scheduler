// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package supportedtypes

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

const (
	queueLabelKey = "kai.scheduler/queue"
)

func TestSupportedTypes(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "SupportedTypes Suite")
}

var _ = Describe("SupportedTypes", func() {
	Context("Exact Match Tests", func() {
		var (
			kubeClient client.Client
			supported  SupportedTypes
		)

		BeforeEach(func() {
			kubeClient = fake.NewFakeClient()
			supported = NewSupportedTypes(kubeClient, false, false, queueLabelKey)
		})

		It("should return func and true for exact GVK match", func() {
			gvk := metav1.GroupVersionKind{
				Group:   "kubeflow.org",
				Version: "v1",
				Kind:    "TFJob",
			}
			funcVal, found := supported.GetPodGroupMetadataFunc(gvk)
			Expect(found).To(BeTrue())
			Expect(funcVal).NotTo(BeNil())
		})

		It("should return nil and false for non-existent GVK", func() {
			gvk := metav1.GroupVersionKind{
				Group:   "non-existent-group",
				Version: "v1",
				Kind:    "NonExistentKind",
			}
			funcVal, found := supported.GetPodGroupMetadataFunc(gvk)
			Expect(found).To(BeFalse())
			Expect(funcVal).To(BeNil())
		})
	})

	Context("Wildcard Version Tests", func() {
		var (
			kubeClient client.Client
			supported  SupportedTypes
		)

		BeforeEach(func() {
			kubeClient = fake.NewFakeClient()
			supported = NewSupportedTypes(kubeClient, false, false, queueLabelKey)
		})

		It("should successfully retrieve with any version for kind set with wildcard", func() {
			gvkWithWildcard := metav1.GroupVersionKind{
				Group:   apiGroupRunai,
				Version: "v100",
				Kind:    kindTrainingWorkload,
			}
			funcVal, found := supported.GetPodGroupMetadataFunc(gvkWithWildcard)
			Expect(found).To(BeTrue())
			Expect(funcVal).NotTo(BeNil())
		})

		It("should successfully retrieve with wildcard version for existing kinds", func() {
			gvkWithWildcard := metav1.GroupVersionKind{
				Group:   apiGroupRunai,
				Version: "*",
				Kind:    kindTrainingWorkload,
			}
			funcVal, found := supported.GetPodGroupMetadataFunc(gvkWithWildcard)
			Expect(found).To(BeTrue())
			Expect(funcVal).NotTo(BeNil())
		})

		It("should return nil and false for non-existent kind with wildcard version", func() {
			gvkWithWildcard := metav1.GroupVersionKind{
				Group:   "non-existent-group",
				Version: "*",
				Kind:    "NonExistentKind",
			}
			funcVal, found := supported.GetPodGroupMetadataFunc(gvkWithWildcard)
			Expect(found).To(BeFalse())
			Expect(funcVal).To(BeNil())
		})
	})
})
