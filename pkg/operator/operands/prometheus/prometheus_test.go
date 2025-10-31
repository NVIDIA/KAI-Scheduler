// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package prometheus

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	kaiv1 "github.com/NVIDIA/KAI-scheduler/pkg/apis/kai/v1"
	kaiprometheus "github.com/NVIDIA/KAI-scheduler/pkg/apis/kai/v1/prometheus"
	"github.com/NVIDIA/KAI-scheduler/pkg/operator/operands/common"
	test_utils "github.com/NVIDIA/KAI-scheduler/pkg/operator/operands/common/test_utils"

	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	corev1 "k8s.io/api/core/v1"                                                // nolint:unused
	rbacv1 "k8s.io/api/rbac/v1"                                                // nolint:unused
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1" // nolint:unused
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
	Expect(corev1.AddToScheme(testScheme)).To(Succeed())
	Expect(rbacv1.AddToScheme(testScheme)).To(Succeed())
	Expect(apiextensionsv1.AddToScheme(testScheme)).To(Succeed())

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
			It("should return ServiceAccount only", func(ctx context.Context) {
				kaiConfig.Spec.Prometheus.Enabled = ptr.To(false)
				objects, err := prometheus.DesiredState(ctx, fakeKubeClient, kaiConfig)
				Expect(err).To(BeNil())
				Expect(len(objects)).To(Equal(1)) // ServiceAccount only
			})

			It("should return ServiceAccount only when config is nil", func(ctx context.Context) {
				kaiConfig.Spec.Prometheus = nil
				objects, err := prometheus.DesiredState(ctx, fakeKubeClient, kaiConfig)
				Expect(err).To(BeNil())
				Expect(len(objects)).To(Equal(1)) // ServiceAccount only
			})
		})

		Context("when Prometheus is enabled", func() {
			BeforeEach(func(ctx context.Context) {
				// Add Prometheus CRD to fake client to simulate Prometheus Operator being installed
				prometheusCRD := &metav1.PartialObjectMetadata{
					TypeMeta: metav1.TypeMeta{
						Kind:       "CustomResourceDefinition",
						APIVersion: "apiextensions.k8s.io/v1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name: "prometheuses.monitoring.coreos.com",
					},
				}
				Expect(fakeKubeClient.Create(ctx, prometheusCRD)).To(Succeed())

				// Add ServiceMonitor CRD to fake client to simulate Prometheus Operator being installed
				serviceMonitorCRD := &metav1.PartialObjectMetadata{
					TypeMeta: metav1.TypeMeta{
						Kind:       "CustomResourceDefinition",
						APIVersion: "apiextensions.k8s.io/v1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name: "servicemonitors.monitoring.coreos.com",
					},
				}
				Expect(fakeKubeClient.Create(ctx, serviceMonitorCRD)).To(Succeed())
			})

			It("should return Prometheus object when Prometheus Operator is installed", func(ctx context.Context) {
				objects, err := prometheus.DesiredState(ctx, fakeKubeClient, kaiConfig)
				Expect(err).To(BeNil())
				Expect(len(objects)).To(Equal(3)) // ServiceAccount, Prometheus, 1 ServiceMonitor

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
				Expect(len(objects)).To(Equal(3)) // ServiceAccount, Prometheus, 1 ServiceMonitor

				prometheusObj := test_utils.FindTypeInObjects[*monitoringv1.Prometheus](objects)
				Expect(prometheusObj).NotTo(BeNil())
				Expect((*prometheusObj).Name).To(Equal(mainResourceName))
				Expect((*prometheusObj).Labels).To(HaveKeyWithValue("existing", "label"))
			})
		})

		Context("when Prometheus Operator is not installed", func() {
			It("should return ServiceAccount only", func(ctx context.Context) {
				objects, err := prometheus.DesiredState(ctx, fakeKubeClient, kaiConfig)
				Expect(err).To(BeNil())
				Expect(len(objects)).To(Equal(1)) // ServiceAccount only
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
				prometheusCRD := &metav1.PartialObjectMetadata{
					TypeMeta: metav1.TypeMeta{
						Kind:       "CustomResourceDefinition",
						APIVersion: "apiextensions.k8s.io/v1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name: "prometheuses.monitoring.coreos.com",
					},
				}
				Expect(fakeKubeClient.Create(ctx, prometheusCRD)).To(Succeed())

				serviceMonitorCRD := &metav1.PartialObjectMetadata{
					TypeMeta: metav1.TypeMeta{
						Kind:       "CustomResourceDefinition",
						APIVersion: "apiextensions.k8s.io/v1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name: "servicemonitors.monitoring.coreos.com",
					},
				}
				Expect(fakeKubeClient.Create(ctx, serviceMonitorCRD)).To(Succeed())

				installed, err := common.CheckPrometheusCRDsAvailable(ctx, fakeKubeClient, "prometheus", "serviceMonitor")
				Expect(err).To(BeNil())
				Expect(installed).To(BeTrue())
			})
		})

		Context("when Prometheus CRD does not exist", func() {
			It("should return false", func(ctx context.Context) {
				installed, err := common.CheckPrometheusCRDsAvailable(ctx, fakeKubeClient, "prometheus", "serviceMonitor")
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
			prometheusCRD := &metav1.PartialObjectMetadata{
				TypeMeta: metav1.TypeMeta{
					Kind:       "CustomResourceDefinition",
					APIVersion: "apiextensions.k8s.io/v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "prometheuses.monitoring.coreos.com",
				},
			}
			Expect(fakeKubeClient.Create(ctx, prometheusCRD)).To(Succeed())

			// Add ServiceMonitor CRD to fake client to simulate Prometheus Operator being installed
			serviceMonitorCRD := &metav1.PartialObjectMetadata{
				TypeMeta: metav1.TypeMeta{
					Kind:       "CustomResourceDefinition",
					APIVersion: "apiextensions.k8s.io/v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "servicemonitors.monitoring.coreos.com",
				},
			}
			Expect(fakeKubeClient.Create(ctx, serviceMonitorCRD)).To(Succeed())
		})

		It("should create new Prometheus object when none exists", func(ctx context.Context) {
			objects, err := prometheusForKAIConfig(ctx, fakeKubeClient, kaiConfig)
			Expect(err).To(BeNil())
			Expect(len(objects)).To(Equal(1)) // Prometheus only

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
			Expect(len(objects)).To(Equal(1)) // Prometheus only

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

	Context("when external Prometheus URL is provided", func() {
		BeforeEach(func(ctx context.Context) {
			// Add ServiceMonitor CRD to fake client to simulate Prometheus Operator being installed
			serviceMonitorCRD := &metav1.PartialObjectMetadata{
				TypeMeta: metav1.TypeMeta{
					Kind:       "CustomResourceDefinition",
					APIVersion: "apiextensions.k8s.io/v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "servicemonitors.monitoring.coreos.com",
				},
			}
			Expect(fakeKubeClient.Create(ctx, serviceMonitorCRD)).To(Succeed())
		})

		It("should return ServiceMonitors only when external Prometheus URL is valid", func(ctx context.Context) {
			kaiConfig.Spec.Prometheus.ExternalPrometheusUrl = ptr.To("http://prometheus.example.com:9090")

			objects, err := prometheusForKAIConfig(ctx, fakeKubeClient, kaiConfig)

			// The function skips Prometheus CR creation and only creates ServiceMonitors
			Expect(err).To(BeNil())
			Expect(objects).NotTo(BeNil())
			Expect(len(objects)).To(Equal(1)) // 1 ServiceMonitor

			serviceMonitor := test_utils.FindTypeInObjects[*monitoringv1.ServiceMonitor](objects)
			Expect(serviceMonitor).NotTo(BeNil())
			Expect((*serviceMonitor).Name).To(Equal("queuecontroller"))
		})

		It("should return empty objects list when ServiceMonitors are disabled", func(ctx context.Context) {
			kaiConfig.Spec.Prometheus.ExternalPrometheusUrl = ptr.To("http://prometheus.example.com:9090")
			kaiConfig.Spec.Prometheus.ServiceMonitor.Enabled = ptr.To(false)

			objects, err := prometheusForKAIConfig(ctx, fakeKubeClient, kaiConfig)

			// When ServiceMonitors are disabled, function returns empty list
			Expect(err).To(BeNil())
			Expect(objects).NotTo(BeNil())
			Expect(len(objects)).To(Equal(0))
		})

		It("should return empty objects list when ServiceMonitor CRD is not available", func(ctx context.Context) {
			kaiConfig.Spec.Prometheus.ExternalPrometheusUrl = ptr.To("http://prometheus.example.com:9090")

			// Remove ServiceMonitor CRD
			serviceMonitorCRD := &metav1.PartialObjectMetadata{
				TypeMeta: metav1.TypeMeta{
					Kind:       "CustomResourceDefinition",
					APIVersion: "apiextensions.k8s.io/v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "servicemonitors.monitoring.coreos.com",
				},
			}
			Expect(fakeKubeClient.Delete(ctx, serviceMonitorCRD)).To(Succeed())

			objects, err := prometheusForKAIConfig(ctx, fakeKubeClient, kaiConfig)

			// When CRD is not available, function returns empty list
			Expect(err).To(BeNil())
			Expect(objects).NotTo(BeNil())
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

var _ = Describe("External Prometheus Validation", func() {
	Context("validateExternalPrometheusConnection", func() {
		var (
			ctx    context.Context
			server *httptest.Server
		)

		BeforeEach(func() {
			ctx = context.Background()
		})

		AfterEach(func() {
			if server != nil {
				server.Close()
			}
		})

		It("should successfully validate connection to a working Prometheus instance", func() {
			// Create a test server that responds to /api/v1/status/config
			server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/api/v1/status/config" {
					w.WriteHeader(http.StatusOK)
					w.Write([]byte(`{"status":"success"}`))
				} else {
					w.WriteHeader(http.StatusNotFound)
				}
			}))

			err := validateExternalPrometheusConnection(ctx, server.URL)
			Expect(err).To(BeNil())
		})

		It("should fail validation for invalid URL", func() {
			err := validateExternalPrometheusConnection(ctx, "://invalid-url")
			Expect(err).NotTo(BeNil())
			Expect(err.Error()).To(ContainSubstring("invalid Prometheus URL"))
		})

		It("should fail validation when Prometheus returns error status", func() {
			server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
			}))

			err := validateExternalPrometheusConnection(ctx, server.URL)
			Expect(err).NotTo(BeNil())
			Expect(err.Error()).To(ContainSubstring("status code 500"))
		})

		It("should fail validation when connection cannot be established", func() {
			err := validateExternalPrometheusConnection(ctx, "http://nonexistent.example.com:9090")
			Expect(err).NotTo(BeNil())
			Expect(err.Error()).To(ContainSubstring("failed to connect"))
		})

		It("should add http scheme when missing", func() {
			server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/api/v1/status/config" {
					w.WriteHeader(http.StatusOK)
					w.Write([]byte(`{"status":"success"}`))
				} else {
					w.WriteHeader(http.StatusNotFound)
				}
			}))

			// Remove the scheme from the URL
			urlWithoutScheme := server.URL[7:] // Remove "http://"
			err := validateExternalPrometheusConnection(ctx, urlWithoutScheme)
			Expect(err).To(BeNil())
		})
	})
})
