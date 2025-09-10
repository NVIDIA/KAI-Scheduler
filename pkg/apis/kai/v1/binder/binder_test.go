// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package binder

import (
	"context"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestBinder(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Binder type suite")
}

var _ = Describe("Binder", func() {
	It("Set Defaults", func(ctx context.Context) {
		binder := &Binder{}
		binder.SetDefaultsWhereNeeded(nil)
		Expect(*binder.Service.Enabled).To(Equal(false))
		Expect(*binder.Service.Image.Name).To(Equal("binder"))
	})
	It("Set Defaults With Replica Count", func(ctx context.Context) {
		binder := &Binder{}
		var replicaCount int32
		replicaCount = 3
		binder.SetDefaultsWhereNeeded(&replicaCount)
		Expect(*binder.Replicas).To(Equal(int32(3)))
	})
})
