// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package config

import (
	kaiv1 "github.com/NVIDIA/KAI-scheduler/pkg/apis/kai/v1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
)

var _ = Describe("test secrets generation", Ordered, func() {
	var kaiConfig = &kaiv1.Config{}
	BeforeEach(func() {
		kaiConfig = &kaiv1.Config{
			Spec: kaiv1.ConfigSpec{
				Global: &kaiv1.GlobalConfig{},
			},
		}
	})
	Context("Generate secrets", func() {

		It("Generate secrets with ImagePullSecretOnly", func() {
			secret := "test_secret"
			kaiConfig.Spec.Global.ImagesPullSecret = &secret
			res := GetGlobalImagePullSecrets(kaiConfig.Spec.Global)
			Expect(len(res)).To(Equal(1))
			Expect(res[0]).To(Equal(v1.LocalObjectReference{Name: "test_secret"}))
		})
		It("Generate secrets with ImagePullSecret And Additional secrets", func() {
			secret := "test_secret"
			kaiConfig.Spec.Global.ImagesPullSecret = &secret
			kaiConfig.Spec.Global.AdditionalImagePullSecrets = []string{"additional1", "additional2"}
			res := GetGlobalImagePullSecrets(kaiConfig.Spec.Global)
			expected := []v1.LocalObjectReference{
				{Name: "additional1"},
				{Name: "additional2"},
				{Name: "test_secret"},
			}
			Expect(len(res)).To(Equal(3))
			Expect(res).To(Equal(expected))
		})
		It("Generate secrets when ImagePullSecret is not set And Additional secrets set ", func() {
			kaiConfig.Spec.Global.AdditionalImagePullSecrets = []string{"additional1", "additional2"}
			res := GetGlobalImagePullSecrets(kaiConfig.Spec.Global)
			expected := []v1.LocalObjectReference{
				{Name: "additional1"},
				{Name: "additional2"},
			}
			Expect(len(res)).To(Equal(2))
			Expect(res).To(Equal(expected))
		})
		It("Empty secrets", func() {
			res := GetGlobalImagePullSecrets(kaiConfig.Spec.Global)
			Expect(len(res)).To(Equal(0))
		})
	})
})
