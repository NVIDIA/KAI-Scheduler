/*
Copyright 2025 NVIDIA CORPORATION
SPDX-License-Identifier: Apache-2.0
*/
package prometheus_timeaware

import (
	"context"
	"os/exec"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	runtimeClient "sigs.k8s.io/controller-runtime/pkg/client"

	kaiv1 "github.com/NVIDIA/KAI-scheduler/pkg/apis/kai/v1"
	kaiprometheus "github.com/NVIDIA/KAI-scheduler/pkg/apis/kai/v1/prometheus"
	"github.com/NVIDIA/KAI-scheduler/pkg/common/constants"
	"github.com/NVIDIA/KAI-scheduler/test/e2e/modules/configurations"
	testcontext "github.com/NVIDIA/KAI-scheduler/test/e2e/modules/context"
	v1 "k8s.io/api/core/v1"
)

var _ = Describe("Prometheus Time-Aware Fairness Integration", Ordered, func() {
	var (
		testCtx        *testcontext.TestContext
		originalConfig *kaiv1.Config
		prometheusNS   string
		prometheusURL  string
	)

	BeforeAll(func(ctx context.Context) {
		testCtx = testcontext.GetConnectivity(ctx, Default)

		// install Prometheus Operator
		installPrometheusOperator(ctx, testCtx)

		// Get the current Config to save for restoration
		config := &kaiv1.Config{}
		err := testCtx.ControllerClient.Get(ctx, types.NamespacedName{
			Name: constants.DefaultKAIConfigSingeltonInstanceName,
		}, config)
		Expect(err).NotTo(HaveOccurred(), "Failed to get Config CRD")
		originalConfig = config.DeepCopy()
	})

	AfterAll(func(ctx context.Context) {
		// Restore original Config
		if originalConfig != nil {
			err := configurations.PatchKAIConfig(ctx, testCtx, func(config *kaiv1.Config) {
				config.Spec = originalConfig.Spec
			})
			Expect(err).NotTo(HaveOccurred(), "Failed to restore original Config")
		}
		uninstallPrometheusOperator(ctx, testCtx)
	})

	It("should set up managed Prometheus and validate time-aware fairness integration", func(ctx context.Context) {
		// Step 1: Enable managed Prometheus
		By("Enabling Prometheus in Config CRD")
		err := configurations.PatchKAIConfig(ctx, testCtx, func(config *kaiv1.Config) {
			if config.Spec.Prometheus == nil {
				config.Spec.Prometheus = &kaiprometheus.Prometheus{}
			}
			config.Spec.Prometheus.Enabled = ptr.To(true)
		})
		Expect(err).NotTo(HaveOccurred(), "Failed to enable Prometheus in Config CRD")

		// Step 2: Get the namespace from Config
		config := &kaiv1.Config{}
		err = testCtx.ControllerClient.Get(ctx, types.NamespacedName{
			Name: constants.DefaultKAIConfigSingeltonInstanceName,
		}, config)
		Expect(err).NotTo(HaveOccurred(), "Failed to get Config CRD")
		prometheusNS = config.Spec.Namespace
		if prometheusNS == "" {
			prometheusNS = constants.DefaultKAINamespace
		}

		// Step 3: Wait for Config to be reconciled and Prometheus CR to be created
		By("Waiting for Config to be reconciled")
		Eventually(func() bool {
			config := &kaiv1.Config{}
			err := testCtx.ControllerClient.Get(ctx, types.NamespacedName{
				Name: constants.DefaultKAIConfigSingeltonInstanceName,
			}, config)
			if err != nil {
				return false
			}
			// Check if reconciling condition indicates success or if there are any errors
			for _, condition := range config.Status.Conditions {
				if condition.Type == string(kaiv1.ConditionTypeReconciling) {
					return condition.Status == metav1.ConditionTrue
				}
			}
			return false
		}).WithTimeout(2*time.Minute).WithPolling(5*time.Second).
			Should(BeTrue(), "Config should be reconciled")

		By("Waiting for Prometheus CR to be created")
		prometheusCR := &unstructured.Unstructured{}
		prometheusCR.SetGroupVersionKind(schema.GroupVersionKind{
			Group:   "monitoring.coreos.com",
			Version: "v1",
			Kind:    "Prometheus",
		})
		var lastErr error
		Eventually(func() error {
			err := testCtx.ControllerClient.Get(ctx, types.NamespacedName{
				Name:      "kai",
				Namespace: prometheusNS,
			}, prometheusCR)
			if err != nil {
				lastErr = err
			}
			return err
		}).WithTimeout(2*time.Minute).WithPolling(5*time.Second).
			Should(Succeed(), "Prometheus CR should be created. Last error: %v", lastErr)

		// Step 4: Wait for Prometheus pods to be running
		By("Waiting for Prometheus pods to be running")
		Eventually(func() bool {
			pods := &v1.PodList{}
			labelSelector := runtimeClient.MatchingLabels{
				"app.kubernetes.io/name": "prometheus",
				"prometheus":             "kai",
			}
			err := testCtx.ControllerClient.List(ctx, pods,
				runtimeClient.InNamespace(prometheusNS),
				labelSelector)
			if err != nil {
				return false
			}

			// Check if at least one pod is running
			for _, pod := range pods.Items {
				if pod.Status.Phase == v1.PodRunning {
					// Also check if container is ready
					for _, containerStatus := range pod.Status.ContainerStatuses {
						if containerStatus.Ready {
							return true
						}
					}
				}
			}
			return false
		}).WithTimeout(5*time.Minute).WithPolling(10*time.Second).
			Should(BeTrue(), "Prometheus pod should be running and ready")

		// // Step 5: Construct Prometheus service URL
		// prometheusURL = fmt.Sprintf("http://kai.%s.svc:9090", prometheusNS)

		// // Step 6: Configure external Prometheus URL to enable connectivity monitoring
		// // This points to our managed Prometheus instance
		// By("Configuring external Prometheus URL for connectivity monitoring")
		// err = configurations.PatchKAIConfig(ctx, testCtx, func(config *kaiv1.Config) {
		// 	if config.Spec.Prometheus == nil {
		// 		config.Spec.Prometheus = &kaiprometheus.Prometheus{}
		// 	}
		// 	config.Spec.Prometheus.ExternalPrometheusUrl = ptr.To(prometheusURL)
		// 	// Ensure Prometheus is still enabled
		// 	config.Spec.Prometheus.Enabled = ptr.To(true)
		// })
		// Expect(err).NotTo(HaveOccurred(), "Failed to configure external Prometheus URL")

		// Step 7: Configure time-aware fairness TSDB connection
		By("Configuring time-aware fairness TSDB connection")
		err = configurations.PatchKAIConfig(ctx, testCtx, func(config *kaiv1.Config) {
			if config.Spec.Global == nil {
				config.Spec.Global = &kaiv1.GlobalConfig{}
			}
			if config.Spec.Global.ExternalTSDBConnection == nil {
				config.Spec.Global.ExternalTSDBConnection = &kaiv1.Connection{}
			}
			config.Spec.Global.ExternalTSDBConnection.URL = ptr.To(prometheusURL)
		})
		Expect(err).NotTo(HaveOccurred(), "Failed to configure TSDB connection")

		// Step 8: Wait for Prometheus connectivity condition
		By("Waiting for Prometheus connectivity to be verified")
		Eventually(func() bool {
			config := &kaiv1.Config{}
			err := testCtx.ControllerClient.Get(ctx, types.NamespacedName{
				Name: constants.DefaultKAIConfigSingeltonInstanceName,
			}, config)
			if err != nil {
				return false
			}

			// Check for PrometheusConnectivity condition
			for _, condition := range config.Status.Conditions {
				if condition.Type == string(kaiv1.Available) {
					return condition.Status == metav1.ConditionTrue &&
						condition.Reason == string(kaiv1.PrometheusConnected)
				}
			}
			return false
		}).WithTimeout(3*time.Minute).WithPolling(10*time.Second).
			Should(BeTrue(), "Prometheus connectivity condition should be True")

		// Step 9: Validate the condition details
		By("Validating Prometheus connectivity condition details")
		config = &kaiv1.Config{}
		err = testCtx.ControllerClient.Get(ctx, types.NamespacedName{
			Name: constants.DefaultKAIConfigSingeltonInstanceName,
		}, config)
		Expect(err).NotTo(HaveOccurred(), "Failed to get Config CRD for validation")

		found := false
		for _, condition := range config.Status.Conditions {
			if condition.Type == string(kaiv1.ConditionTypePrometheusConnectivity) {
				found = true
				Expect(condition.Status).To(Equal(metav1.ConditionTrue),
					"Prometheus connectivity status should be True")
				Expect(condition.Reason).To(Equal(string(kaiv1.PrometheusConnected)),
					"Prometheus connectivity reason should be prometheus_connected")
				Expect(condition.Message).To(ContainSubstring("External Prometheus connectivity verified"),
					"Prometheus connectivity message should indicate success")
				break
			}
		}
		Expect(found).To(BeTrue(), "PrometheusConnectivity condition should exist")

		// Step 10: Verify Prometheus Service exists
		By("Verifying Prometheus Service exists")
		service := &v1.Service{}
		err = testCtx.ControllerClient.Get(ctx, types.NamespacedName{
			Name:      "kai",
			Namespace: prometheusNS,
		}, service)
		Expect(err).NotTo(HaveOccurred(), "Prometheus Service should exist")
		Expect(service.Spec.Ports).NotTo(BeEmpty(), "Prometheus Service should have ports")

		// Verify at least one port is 9090
		has9090Port := false
		for _, port := range service.Spec.Ports {
			if port.Port == 9090 {
				has9090Port = true
				break
			}
		}
		Expect(has9090Port).To(BeTrue(), "Prometheus Service should expose port 9090")
	})
})

func installPrometheusOperator(ctx context.Context, testCtx *testcontext.TestContext) {
	By("Cleaning up previous Prometheus Operator Helm release and cluster resources")
	// Cleanup Helm release (ignore errors)
	cmd := exec.CommandContext(ctx, "helm", "uninstall", "prometheus-operator", "--namespace", "monitoring")
	cmd.Stdout = GinkgoWriter
	cmd.Stderr = GinkgoWriter
	_ = cmd.Run() // ignore error

	// Cleanup ClusterRole (ignore errors)
	exec.CommandContext(ctx, "kubectl", "delete", "clusterrole", "prometheus-operator-kube-state-metrics").Run()
	exec.CommandContext(ctx, "kubectl", "delete", "clusterrolebinding", "prometheus-operator-kube-state-metrics").Run()

	By("Installing Prometheus Operator via Helm")

	// Add helm repo
	cmd = exec.CommandContext(ctx, "helm", "repo", "add", "prometheus-community",
		"https://prometheus-community.github.io/helm-charts")
	Expect(cmd.Run()).To(Succeed())

	// Update repo
	cmd = exec.CommandContext(ctx, "helm", "repo", "update", "prometheus-community")
	Expect(cmd.Run()).To(Succeed())

	// Install only the operator (not the full stack)
	cmd = exec.CommandContext(ctx, "helm", "install", "prometheus-operator",
		"prometheus-community/kube-prometheus-stack",
		"--namespace", "monitoring",
		"--create-namespace",
		"--set", "prometheus.enabled=false",
		"--set", "grafana.enabled=false",
		"--set", "alertmanager.enabled=false",
		"--wait")

	cmd.Stdout = GinkgoWriter
	cmd.Stderr = GinkgoWriter
	Expect(cmd.Run()).To(Succeed())
	waitForPrometheusOperator(ctx, testCtx)
}

func waitForPrometheusOperator(ctx context.Context, testCtx *testcontext.TestContext) {
	By("Waiting for Prometheus Operator to be ready")
	Eventually(func() bool {
		deployments, err := testCtx.KubeClientset.AppsV1().Deployments("monitoring").
			List(ctx, metav1.ListOptions{LabelSelector: "app.kubernetes.io/name=kube-prometheus-stack-prometheus-operator"})
		if err != nil || len(deployments.Items) == 0 {
			return false
		}
		deployment := deployments.Items[0]
		return deployment.Status.ReadyReplicas >= 1
	}).WithTimeout(2*time.Minute).WithPolling(10*time.Second).
		Should(BeTrue(), "Prometheus Operator should be ready")
}

func uninstallPrometheusOperator(ctx context.Context, testCtx *testcontext.TestContext) {
	By("Uninstalling Prometheus Operator via Helm")
	cmd := exec.CommandContext(ctx, "helm", "uninstall", "prometheus-operator", "--namespace", "monitoring")
	cmd.Stdout = GinkgoWriter
	cmd.Stderr = GinkgoWriter
	_ = cmd.Run() // ignore error

}
