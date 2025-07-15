// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package flags

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"testing"
)

func TestCommonFlags(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "common flags tests")
}

var _ = Describe("StringMapFlag", func() {
	It("parses an empty string as an empty map", func() {
		var m StringMapFlag
		Expect(m.Set("")).To(Succeed())
		Expect(m.Get()).To(BeEmpty())
	})

	It("parses a single key=value pair", func() {
		var m StringMapFlag
		Expect(m.Set("foo=bar")).To(Succeed())
		Expect(m.Get()).To(HaveKeyWithValue("foo", "bar"))
		Expect(len(m.Get())).To(Equal(1))
	})

	It("parses multiple key=value pairs", func() {
		var m StringMapFlag
		Expect(m.Set("foo=bar,baz=qux")).To(Succeed())
		Expect(m.Get()).To(HaveKeyWithValue("foo", "bar"))
		Expect(m.Get()).To(HaveKeyWithValue("baz", "qux"))
		Expect(len(m.Get())).To(Equal(2))
	})

	It("overwrites duplicate keys with the last value", func() {
		var m StringMapFlag
		Expect(m.Set("foo=bar,foo=baz")).To(Succeed())
		Expect(m.Get()).To(HaveKeyWithValue("foo", "baz"))
		Expect(len(m.Get())).To(Equal(1))
	})

	It("returns an error for invalid input", func() {
		var m StringMapFlag
		err := m.Set("foo,bar=baz")
		Expect(err).To(HaveOccurred())
	})

	It("String() returns the correct string representation", func() {
		var m StringMapFlag
		err := m.Set("foo=bar,baz=qux")
		Expect(err).ToNot(HaveOccurred())
		str := m.String()
		// Accept either order
		Expect([]string{str, reversePairs(str)}).To(ContainElement("foo=bar,baz=qux"))
	})
})

// Helper to reverse the order of pairs in a comma-separated string
func reversePairs(s string) string {
	pairs := []rune(s)
	for i, j := 0, len(pairs)-1; i < j; i, j = i+1, j-1 {
		pairs[i], pairs[j] = pairs[j], pairs[i]
	}
	return string(pairs)
}
