// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package pod_group_controller

import (
	"context"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"testing"
)

func TestPodGrouper(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "PodGrouper type suite")
}

var _ = Describe("PodGrouper", func() {
	It("Set Defaults", func(ctx context.Context) {
		podGrouper := &PodGroupController{}
		podGrouper.SetDefaultsWhereNeeded(nil)
		Expect(*podGrouper.Enabled).To(Equal(true))
		Expect(*podGrouper.Image.Name).To(Equal(imageName))
	})
	It("Set Defaults With replicas", func(ctx context.Context) {
		podGrouper := &PodGroupController{}
		var replicaCount int32
		replicaCount = 3
		podGrouper.SetDefaultsWhereNeeded(&replicaCount)
		Expect(*podGrouper.Replicas).To(Equal(int32(3)))
	})
})
