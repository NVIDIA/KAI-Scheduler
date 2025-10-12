// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package prometheus

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// Helper functions
func stringPtr(s string) *string {
	return &s
}

func TestPrometheus(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Prometheus Suite")
}

var _ = Describe("TSDB CalculateStorageSize", func() {
	var (
		ctx        context.Context
		fakeClient client.Client
		scheme     *runtime.Scheme
	)

	BeforeEach(func() {
		ctx = context.Background()
		logger := log.FromContext(ctx)
		ctx = log.IntoContext(ctx, logger)
		scheme = runtime.NewScheme()
	})

	Context("with 1000 queues and 5 nodepools", func() {
		BeforeEach(func() {
			// Create fake client with 5 SchedulingShards
			var objects []client.Object
			for i := 0; i < 5; i++ {
				shard := &unstructured.Unstructured{}
				shard.SetGroupVersionKind(schema.GroupVersionKind{
					Group:   "kai.scheduler",
					Version: "v1",
					Kind:    "SchedulingShard",
				})
				shard.SetName(fmt.Sprintf("shard-%d", i))
				objects = append(objects, shard)
			}

			// Create fake client with 1000 Queues
			for i := 0; i < 1000; i++ {
				queue := &unstructured.Unstructured{}
				queue.SetGroupVersionKind(schema.GroupVersionKind{
					Group:   "scheduling.run.ai",
					Version: "v2",
					Kind:    "Queue",
				})
				queue.SetName(fmt.Sprintf("queue-%d", i))
				objects = append(objects, queue)
			}

			fakeClient = fake.NewClientBuilder().WithScheme(scheme).WithObjects(objects...).Build()
		})

		It("should calculate correct storage size for large scale deployment", func() {
			tsdb := &Prometheus{
				RetentionPeriod: stringPtr("2w"),
				SampleInterval:  stringPtr("1m"),
			}
			tsdb.SetDefaultsWhereNeeded()

			size, err := tsdb.CalculateStorageSize(ctx, fakeClient)

			Expect(err).ToNot(HaveOccurred())
			Expect(size).To(ContainSubstring("Gi"))

			sizeStr := strings.TrimSuffix(size, "Gi")
			sizeValue, parseErr := strconv.Atoi(sizeStr)
			Expect(parseErr).ToNot(HaveOccurred())

			// Expected: 0.94 Gi, rounded to 1 Gi
			expectedMinGi := 1
			Expect(sizeValue).To(Equal(expectedMinGi))
		})

		It("should handle different retention periods correctly", func() {
			tsdb := &Prometheus{
				RetentionPeriod: stringPtr("1d"),
				SampleInterval:  stringPtr("30s"),
			}
			tsdb.SetDefaultsWhereNeeded()

			size, err := tsdb.CalculateStorageSize(ctx, fakeClient)

			Expect(err).ToNot(HaveOccurred())
			Expect(size).To(ContainSubstring("Gi"))

			sizeStr := strings.TrimSuffix(size, "Gi")
			sizeValue, parseErr := strconv.Atoi(sizeStr)
			Expect(parseErr).ToNot(HaveOccurred())

			// Expected: 0.067 Gi, rounded to 1 Gi
			expectedMinGi := 1
			Expect(sizeValue).To(Equal(expectedMinGi))
		})
	})

	Context("with 0 queues and 0 nodepools", func() {
		BeforeEach(func() {
			// Create fake client with no objects
			fakeClient = fake.NewClientBuilder().WithScheme(scheme).Build()
		})

		It("should use default values and calculate minimal storage", func() {
			tsdb := &Prometheus{
				RetentionPeriod: stringPtr("2w"),
				SampleInterval:  stringPtr("1m"),
			}
			tsdb.SetDefaultsWhereNeeded()

			size, err := tsdb.CalculateStorageSize(ctx, fakeClient)

			Expect(err).ToNot(HaveOccurred())
			Expect(size).To(ContainSubstring("Gi"))

			sizeStr := strings.TrimSuffix(size, "Gi")
			sizeValue, parseErr := strconv.Atoi(sizeStr)
			Expect(parseErr).ToNot(HaveOccurred())

			// Expected: 0.00019 Gi, rounded to 1 Gi
			expectedMinGi := 1
			Expect(sizeValue).To(Equal(expectedMinGi))
		})

		It("should handle empty cluster gracefully", func() {
			tsdb := &Prometheus{
				RetentionPeriod: stringPtr("1w"),
				SampleInterval:  stringPtr("5m"),
			}
			tsdb.SetDefaultsWhereNeeded()

			size, err := tsdb.CalculateStorageSize(ctx, fakeClient)

			Expect(err).ToNot(HaveOccurred())
			Expect(size).To(ContainSubstring("Gi"))

			sizeStr := strings.TrimSuffix(size, "Gi")
			sizeValue, parseErr := strconv.Atoi(sizeStr)
			Expect(parseErr).ToNot(HaveOccurred())

			// Expected: 0.000019 Gi, rounded to 1 Gi
			expectedMinGi := 1
			Expect(sizeValue).To(Equal(expectedMinGi))
		})
	})

	Context("edge cases", func() {
		BeforeEach(func() {
			fakeClient = fake.NewClientBuilder().WithScheme(scheme).Build()
		})

		It("should handle nil TSDB gracefully", func() {
			var tsdb *Prometheus
			// This should not panic
			Expect(func() { tsdb.SetDefaultsWhereNeeded() }).ToNot(Panic())
		})

		It("should handle invalid duration formats", func() {
			tsdb := &Prometheus{
				RetentionPeriod: stringPtr("invalid"),
				SampleInterval:  stringPtr("1m"),
			}
			tsdb.SetDefaultsWhereNeeded()

			size, err := tsdb.CalculateStorageSize(ctx, fakeClient)

			Expect(err).To(HaveOccurred())
			Expect(size).To(Equal("30Gi")) // Fallback value
		})
	})
})
