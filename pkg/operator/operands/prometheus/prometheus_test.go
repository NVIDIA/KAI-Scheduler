// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package prometheus

import (
	"context"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	kaiv1 "github.com/NVIDIA/KAI-scheduler/pkg/apis/kai/v1"
	kaiprometheus "github.com/NVIDIA/KAI-scheduler/pkg/apis/kai/v1/prometheus"
	test_utils "github.com/NVIDIA/KAI-scheduler/pkg/operator/operands/common/test_utils"

	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestPrometheus(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Prometheus operand Suite")
}

func createFakeClientWithScheme() client.Client {
	testScheme := runtime.NewScheme()
	Expect(kaiv1.AddToScheme(testScheme)).To(Succeed())
	Expect(monitoringv1.AddToScheme(testScheme)).To(Succeed())

	return fake.NewClientBuilder().WithScheme(testScheme).Build()
}

var _ = Describe("Prometheus", func() {
	Describe("DesiredState", func() {
		var (
			fakeKubeClient client.Client
			prometheus     *Prometheus
			kaiConfig      *kaiv1.Config
		)

		BeforeEach(func(ctx context.Context) {
			fakeKubeClient = createFakeClientWithScheme()
			prometheus = &Prometheus{}
			kaiConfig = kaiConfigForPrometheus()
		})

		Context("when Prometheus is disabled", func() {
			It("should return empty objects list", func(ctx context.Context) {
				kaiConfig.Spec.Prometheus.Enabled = ptr.To(false)
				objects, err := prometheus.DesiredState(ctx, fakeKubeClient, kaiConfig)
				Expect(err).To(BeNil())
				Expect(len(objects)).To(Equal(0))
			})

			It("should return empty objects list when config is nil", func(ctx context.Context) {
				kaiConfig.Spec.Prometheus = nil
				objects, err := prometheus.DesiredState(ctx, fakeKubeClient, kaiConfig)
				Expect(err).To(BeNil())
				Expect(len(objects)).To(Equal(0))
			})
		})

		Context("when Prometheus is enabled", func() {
			BeforeEach(func(ctx context.Context) {
				// Add Prometheus CRD to fake client to simulate Prometheus Operator being installed
				crd := &metav1.PartialObjectMetadata{
					TypeMeta: metav1.TypeMeta{
						Kind:       "CustomResourceDefinition",
						APIVersion: "apiextensions.k8s.io/v1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name: "prometheuses.monitoring.coreos.com",
					},
				}
				Expect(fakeKubeClient.Create(ctx, crd)).To(Succeed())
			})

			It("should return Prometheus object when Prometheus Operator is installed", func(ctx context.Context) {
				objects, err := prometheus.DesiredState(ctx, fakeKubeClient, kaiConfig)
				Expect(err).To(BeNil())
				Expect(len(objects)).To(Equal(1))

				prometheusObj := test_utils.FindTypeInObjects[*monitoringv1.Prometheus](objects)
				Expect(prometheusObj).NotTo(BeNil())
				Expect((*prometheusObj).Name).To(Equal(mainResourceName))
				Expect((*prometheusObj).Namespace).To(Equal(kaiConfig.Spec.Namespace))
			})

			It("should return existing Prometheus object if it already exists", func(ctx context.Context) {
				// Create an existing Prometheus instance
				existingPrometheus := &monitoringv1.Prometheus{
					TypeMeta: metav1.TypeMeta{
						Kind:       "Prometheus",
						APIVersion: "monitoring.coreos.com/v1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:      mainResourceName,
						Namespace: kaiConfig.Spec.Namespace,
						Labels: map[string]string{
							"existing": "label",
						},
					},
				}
				Expect(fakeKubeClient.Create(ctx, existingPrometheus)).To(Succeed())

				objects, err := prometheus.DesiredState(ctx, fakeKubeClient, kaiConfig)
				Expect(err).To(BeNil())
				Expect(len(objects)).To(Equal(1))

				prometheusObj := test_utils.FindTypeInObjects[*monitoringv1.Prometheus](objects)
				Expect(prometheusObj).NotTo(BeNil())
				Expect((*prometheusObj).Name).To(Equal(mainResourceName))
				Expect((*prometheusObj).Labels).To(HaveKeyWithValue("existing", "label"))
			})
		})

		Context("when Prometheus Operator is not installed", func() {
			It("should return empty objects list", func(ctx context.Context) {
				objects, err := prometheus.DesiredState(ctx, fakeKubeClient, kaiConfig)
				Expect(err).To(BeNil())
				Expect(len(objects)).To(Equal(0))
			})
		})
	})

	Describe("CheckPrometheusOperatorInstalled", func() {
		var (
			fakeKubeClient client.Client
		)

		BeforeEach(func(ctx context.Context) {
			fakeKubeClient = createFakeClientWithScheme()
		})

		Context("when Prometheus CRD exists", func() {
			It("should return true", func(ctx context.Context) {
				crd := &metav1.PartialObjectMetadata{
					TypeMeta: metav1.TypeMeta{
						Kind:       "CustomResourceDefinition",
						APIVersion: "apiextensions.k8s.io/v1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name: "prometheuses.monitoring.coreos.com",
					},
				}
				Expect(fakeKubeClient.Create(ctx, crd)).To(Succeed())

				installed, err := CheckPrometheusOperatorInstalled(ctx, fakeKubeClient)
				Expect(err).To(BeNil())
				Expect(installed).To(BeTrue())
			})
		})

		Context("when Prometheus CRD does not exist", func() {
			It("should return false", func(ctx context.Context) {
				installed, err := CheckPrometheusOperatorInstalled(ctx, fakeKubeClient)
				Expect(err).To(BeNil())
				Expect(installed).To(BeFalse())
			})
		})
	})

	Describe("Operand methods", func() {
		var (
			fakeKubeClient client.Client
			prometheus     *Prometheus
		)

		BeforeEach(func(ctx context.Context) {
			fakeKubeClient = createFakeClientWithScheme()
			prometheus = &Prometheus{
				namespace: "test-namespace",
			}
		})

		Context("IsDeployed", func() {
			It("should return true when all objects exist", func(ctx context.Context) {
				// Create a Prometheus object
				prometheusObj := &monitoringv1.Prometheus{
					TypeMeta: metav1.TypeMeta{
						Kind:       "Prometheus",
						APIVersion: "monitoring.coreos.com/v1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:      mainResourceName,
						Namespace: "test-namespace",
					},
				}
				Expect(fakeKubeClient.Create(ctx, prometheusObj)).To(Succeed())

				prometheus.lastDesiredState = []client.Object{prometheusObj}

				deployed, err := prometheus.IsDeployed(ctx, fakeKubeClient)
				Expect(err).To(BeNil())
				Expect(deployed).To(BeTrue())
			})

			It("should return false when objects do not exist", func(ctx context.Context) {
				prometheusObj := &monitoringv1.Prometheus{
					TypeMeta: metav1.TypeMeta{
						Kind:       "Prometheus",
						APIVersion: "monitoring.coreos.com/v1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:      mainResourceName,
						Namespace: "test-namespace",
					},
				}

				prometheus.lastDesiredState = []client.Object{prometheusObj}

				deployed, err := prometheus.IsDeployed(ctx, fakeKubeClient)
				Expect(err).To(BeNil())
				Expect(deployed).To(BeFalse())
			})
		})

		Context("IsAvailable", func() {
			It("should return true when all controllers are available", func(ctx context.Context) {
				// Create a Prometheus object with proper status conditions
				prometheusObj := &monitoringv1.Prometheus{
					TypeMeta: metav1.TypeMeta{
						Kind:       "Prometheus",
						APIVersion: "monitoring.coreos.com/v1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:      mainResourceName,
						Namespace: "test-namespace",
					},
					Status: monitoringv1.PrometheusStatus{
						Conditions: []monitoringv1.Condition{
							{
								Type:   monitoringv1.ConditionType("Available"),
								Status: monitoringv1.ConditionTrue,
							},
						},
					},
				}
				Expect(fakeKubeClient.Create(ctx, prometheusObj)).To(Succeed())

				prometheus.lastDesiredState = []client.Object{prometheusObj}

				available, err := prometheus.IsAvailable(ctx, fakeKubeClient)
				Expect(err).To(BeNil())
				Expect(available).To(BeTrue())
			})

			It("should return false when Prometheus is not available", func(ctx context.Context) {
				// Create a Prometheus object with unavailable status
				prometheusObj := &monitoringv1.Prometheus{
					TypeMeta: metav1.TypeMeta{
						Kind:       "Prometheus",
						APIVersion: "monitoring.coreos.com/v1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:      mainResourceName,
						Namespace: "test-namespace",
					},
					Status: monitoringv1.PrometheusStatus{
						Conditions: []monitoringv1.Condition{
							{
								Type:   monitoringv1.ConditionType("Available"),
								Status: monitoringv1.ConditionFalse,
							},
						},
					},
				}
				Expect(fakeKubeClient.Create(ctx, prometheusObj)).To(Succeed())

				prometheus.lastDesiredState = []client.Object{prometheusObj}

				available, err := prometheus.IsAvailable(ctx, fakeKubeClient)
				Expect(err).To(BeNil())
				Expect(available).To(BeFalse())
			})

			It("should return false when Prometheus object does not exist", func(ctx context.Context) {
				// Create a Prometheus object in desired state but don't create it in the cluster
				prometheusObj := &monitoringv1.Prometheus{
					TypeMeta: metav1.TypeMeta{
						Kind:       "Prometheus",
						APIVersion: "monitoring.coreos.com/v1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:      mainResourceName,
						Namespace: "test-namespace",
					},
				}
				prometheus.lastDesiredState = []client.Object{prometheusObj}

				available, err := prometheus.IsAvailable(ctx, fakeKubeClient)
				Expect(err).ToNot(BeNil())
				Expect(available).To(BeFalse())
			})
		})

		Context("Name", func() {
			It("should return correct operand name", func(ctx context.Context) {
				name := prometheus.Name()
				Expect(name).To(Equal("KAI-prometheus"))
			})
		})
	})
})

var _ = Describe("prometheusForKAIConfig", func() {
	var (
		fakeKubeClient client.Client
		kaiConfig      *kaiv1.Config
	)

	BeforeEach(func(ctx context.Context) {
		fakeKubeClient = createFakeClientWithScheme()
		kaiConfig = kaiConfigForPrometheus()
	})

	Context("when Prometheus is disabled", func() {
		It("should return empty objects list", func(ctx context.Context) {
			kaiConfig.Spec.Prometheus.Enabled = ptr.To(false)
			objects, err := prometheusForKAIConfig(ctx, fakeKubeClient, kaiConfig)
			Expect(err).To(BeNil())
			Expect(len(objects)).To(Equal(0))
		})

		It("should return empty objects list when config is nil", func(ctx context.Context) {
			kaiConfig.Spec.Prometheus = nil
			objects, err := prometheusForKAIConfig(ctx, fakeKubeClient, kaiConfig)
			Expect(err).To(BeNil())
			Expect(len(objects)).To(Equal(0))
		})

		It("should return empty objects list when enabled is nil", func(ctx context.Context) {
			kaiConfig.Spec.Prometheus.Enabled = nil
			objects, err := prometheusForKAIConfig(ctx, fakeKubeClient, kaiConfig)
			Expect(err).To(BeNil())
			Expect(len(objects)).To(Equal(0))
		})
	})

	Context("when Prometheus is enabled", func() {
		BeforeEach(func(ctx context.Context) {
			// Add Prometheus CRD to fake client to simulate Prometheus Operator being installed
			crd := &metav1.PartialObjectMetadata{
				TypeMeta: metav1.TypeMeta{
					Kind:       "CustomResourceDefinition",
					APIVersion: "apiextensions.k8s.io/v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "prometheuses.monitoring.coreos.com",
				},
			}
			Expect(fakeKubeClient.Create(ctx, crd)).To(Succeed())
		})

		It("should create new Prometheus object when none exists", func(ctx context.Context) {
			objects, err := prometheusForKAIConfig(ctx, fakeKubeClient, kaiConfig)
			Expect(err).To(BeNil())
			Expect(len(objects)).To(Equal(1))

			prometheusObj := test_utils.FindTypeInObjects[*monitoringv1.Prometheus](objects)
			Expect(prometheusObj).NotTo(BeNil())
			Expect((*prometheusObj).Name).To(Equal(mainResourceName))
			Expect((*prometheusObj).Namespace).To(Equal(kaiConfig.Spec.Namespace))
			Expect((*prometheusObj).Labels).To(HaveKeyWithValue("app", mainResourceName))
		})

		It("should return existing Prometheus object if it already exists", func(ctx context.Context) {
			// Create an existing Prometheus instance
			existingPrometheus := &monitoringv1.Prometheus{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Prometheus",
					APIVersion: "monitoring.coreos.com/v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      mainResourceName,
					Namespace: kaiConfig.Spec.Namespace,
					Labels: map[string]string{
						"existing": "label",
						"app":      mainResourceName,
					},
				},
			}
			Expect(fakeKubeClient.Create(ctx, existingPrometheus)).To(Succeed())

			objects, err := prometheusForKAIConfig(ctx, fakeKubeClient, kaiConfig)
			Expect(err).To(BeNil())
			Expect(len(objects)).To(Equal(1))

			prometheusObj := test_utils.FindTypeInObjects[*monitoringv1.Prometheus](objects)
			Expect(prometheusObj).NotTo(BeNil())
			Expect((*prometheusObj).Name).To(Equal(mainResourceName))
			Expect((*prometheusObj).Labels).To(HaveKeyWithValue("existing", "label"))
			Expect((*prometheusObj).Labels).To(HaveKeyWithValue("app", mainResourceName))
		})
	})

	Context("when Prometheus Operator is not installed", func() {
		It("should return empty objects list", func(ctx context.Context) {
			objects, err := prometheusForKAIConfig(ctx, fakeKubeClient, kaiConfig)
			Expect(err).To(BeNil())
			Expect(len(objects)).To(Equal(0))
		})
	})
})

func kaiConfigForPrometheus() *kaiv1.Config {
	kaiConfig := &kaiv1.Config{}
	kaiConfig.Spec.SetDefaultsWhereNeeded()
	kaiConfig.Spec.Prometheus = &kaiprometheus.Prometheus{
		Enabled: ptr.To(true),
	}
	kaiConfig.Spec.Prometheus.SetDefaultsWhereNeeded()
	kaiConfig.Spec.Namespace = "test-namespace"

	return kaiConfig
}
