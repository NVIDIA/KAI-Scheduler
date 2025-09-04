// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package admission

import (
	"context"
	"fmt"
	"strconv"

	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/types"

	admissionv1 "k8s.io/api/admissionregistration/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	kaiv1 "github.com/NVIDIA/KAI-scheduler/pkg/apis/kai/v1"
	kaiv1admission "github.com/NVIDIA/KAI-scheduler/pkg/apis/kai/v1/admission"
	generate "github.com/NVIDIA/KAI-scheduler/pkg/operator/cert-utils"
	"github.com/NVIDIA/KAI-scheduler/pkg/operator/operands/common"
)

const (
	mainResourceName                         = "admission"
	mutatorWebhookKAIAdmissionResourceName   = "mutating-kai-admission"
	validatorWebhookKAIAdmissionResourceName = "validating-kai-admission"
	kaiAdmissionWebhookSecretName            = "kai-admission-webhook-tls-secret"
	certKey                                  = "tls.crt"
	keyKey                                   = "tls.key"
)

func deploymentForKAIConfig(
	ctx context.Context, runtimeClient client.Reader, kaiConfig *kaiv1.Config,
) ([]client.Object, error) {

	config := kaiConfig.Spec.Admission
	deployment, err := common.DeploymentForKAIConfig(ctx, runtimeClient, kaiConfig, config.Service, mainResourceName)
	if err != nil {
		return nil, err
	}

	deployment.Spec.Strategy.Type = appsv1.RecreateDeploymentStrategyType
	deployment.Spec.Strategy.RollingUpdate = nil
	deployment.Spec.Replicas = config.Replicas
	deployment.Spec.Template.Spec.Containers[0].Args = buildArgsList(kaiConfig, config)
	deployment.Spec.Template.Spec.Containers[0].VolumeMounts = []v1.VolumeMount{
		{
			Name:      "cert",
			ReadOnly:  true,
			MountPath: "/tmp/k8s-webhook-server/serving-certs",
		},
	}
	deployment.Spec.Template.Spec.Containers[0].Ports = []v1.ContainerPort{
		{
			Name:          "webhook",
			ContainerPort: int32(*config.Webhook.TargetPort),
		},
	}
	deployment.Spec.Template.Spec.Containers[0].ReadinessProbe = &v1.Probe{
		ProbeHandler: v1.ProbeHandler{
			HTTPGet: &v1.HTTPGetAction{
				Path: "/readyz",
				Port: intstr.IntOrString{
					Type:   intstr.Int,
					IntVal: int32(*config.Webhook.ProbePort),
				},
			},
		},
		InitialDelaySeconds: 5,
	}
	deployment.Spec.Template.Spec.Volumes = []v1.Volume{
		{
			Name: "cert",
			VolumeSource: v1.VolumeSource{
				Secret: &v1.SecretVolumeSource{
					SecretName:  kaiAdmissionWebhookSecretName,
					DefaultMode: ptr.To(int32(420)),
				},
			},
		},
	}

	return []client.Object{deployment}, nil
}

func serviceAccountForKAIConfig(
	ctx context.Context, runtimeClient client.Reader, kaiConfig *kaiv1.Config,
) ([]client.Object, error) {
	sa, err := common.ObjectForKAIConfig(ctx, runtimeClient, &v1.ServiceAccount{}, mainResourceName,
		kaiConfig.Spec.Namespace)
	if err != nil {
		return nil, err
	}
	sa.(*v1.ServiceAccount).TypeMeta = metav1.TypeMeta{
		Kind:       "ServiceAccount",
		APIVersion: "v1",
	}
	return []client.Object{sa}, err
}

func serviceForKAIConfig(
	ctx context.Context, runtimeClient client.Reader, kaiConfig *kaiv1.Config,
) ([]client.Object, error) {
	serviceObj, err := common.ObjectForKAIConfig(ctx, runtimeClient, &v1.Service{}, mainResourceName,
		kaiConfig.Spec.Namespace)
	if err != nil {
		return nil, err
	}

	service := serviceObj.(*v1.Service)
	service.TypeMeta = metav1.TypeMeta{
		Kind:       "Service",
		APIVersion: "v1",
	}
	config := kaiConfig.Spec.Admission

	service.Spec.Ports = []v1.ServicePort{
		{
			Name:       "webhook",
			Port:       int32(*config.Webhook.Port),
			Protocol:   v1.ProtocolTCP,
			TargetPort: intstr.FromInt(*config.Webhook.TargetPort),
		},
		{
			Name:       "http-metrics",
			Port:       int32(*config.Webhook.MetricsPort),
			Protocol:   v1.ProtocolTCP,
			TargetPort: intstr.FromInt(*config.Webhook.MetricsPort),
		},
	}
	service.Spec.Selector = map[string]string{
		"app": mainResourceName,
	}

	service.Spec.SessionAffinity = v1.ServiceAffinityNone
	service.Spec.Type = v1.ServiceTypeClusterIP

	return []client.Object{service}, nil
}

func buildWebhookSelectors(kaiConfig *kaiv1.Config) (namespaceSelector *metav1.LabelSelector, objectSelector *metav1.LabelSelector) {
	config := kaiConfig.Spec.Admission

	namespaceSelector = &metav1.LabelSelector{
		MatchExpressions: func() []metav1.LabelSelectorRequirement {
			requirements := []metav1.LabelSelectorRequirement{
				{
					Key:      "kubernetes.io/metadata.name",
					Operator: metav1.LabelSelectorOpNotIn,
					Values:   []string{"kube-system", kaiConfig.Spec.Namespace},
				},
			}

			if config.QueueLabelSelector != nil && *config.QueueLabelSelector {
				requirements = append(requirements,
					metav1.LabelSelectorRequirement{
						Key:      *kaiConfig.Spec.Global.QueueLabelKey,
						Operator: metav1.LabelSelectorOpExists,
					},
				)
			}
			return requirements
		}(),
		MatchLabels: func() map[string]string {
			if len(kaiConfig.Spec.Global.NamespaceLabelSelector) > 0 {
				return kaiConfig.Spec.Global.NamespaceLabelSelector
			}
			return nil
		}(),
	}

	if len(kaiConfig.Spec.Global.PodLabelSelector) > 0 {
		objectSelector = &metav1.LabelSelector{
			MatchLabels: kaiConfig.Spec.Global.PodLabelSelector,
		}
	}

	return namespaceSelector, objectSelector
}

func buildWebhookClientConfig(kaiConfig *kaiv1.Config, secret *v1.Secret, webhookPath string) admissionv1.WebhookClientConfig {
	return admissionv1.WebhookClientConfig{
		Service: &admissionv1.ServiceReference{
			Namespace: kaiConfig.Spec.Namespace,
			Name:      mainResourceName,
			Path:      ptr.To(webhookPath),
		},
		CABundle: secret.Data[certKey],
	}
}

func mutatingWCForKAIConfig(
	ctx context.Context, runtimeClient client.Reader, kaiConfig *kaiv1.Config,
	secret *v1.Secret, webhookName string,
) ([]client.Object, error) {
	mutatingWebhookConfiguration := &admissionv1.MutatingWebhookConfiguration{}
	err := runtimeClient.Get(ctx, types.NamespacedName{Name: mutatorWebhookKAIAdmissionResourceName}, mutatingWebhookConfiguration)
	if client.IgnoreNotFound(err) != nil {
		return nil, err
	}
	mutatingWebhookConfiguration.Name = mutatorWebhookKAIAdmissionResourceName

	if mutatingWebhookConfiguration.Labels == nil {
		mutatingWebhookConfiguration.Labels = map[string]string{}
	}
	mutatingWebhookConfiguration.Labels["app"] = mainResourceName

	namespaceSelector, objectSelector := buildWebhookSelectors(kaiConfig)
	clientConfig := buildWebhookClientConfig(kaiConfig, secret, "/mutate--v1-pod")

	mutatingWebhookConfiguration.Webhooks = []admissionv1.MutatingWebhook{
		{
			Name:                    webhookName,
			AdmissionReviewVersions: []string{"v1"},
			SideEffects:             common.PtrFrom(admissionv1.SideEffectClassNone),
			NamespaceSelector:       namespaceSelector,
			ObjectSelector:          objectSelector,
			FailurePolicy:           common.PtrFrom(admissionv1.Fail),
			ClientConfig:            clientConfig,
			Rules: []admissionv1.RuleWithOperations{
				{
					Operations: []admissionv1.OperationType{admissionv1.Create},
					Rule: admissionv1.Rule{
						APIGroups:   []string{""},
						APIVersions: []string{"v1"},
						Resources:   []string{"pods"},
						Scope:       common.PtrFrom(admissionv1.NamespacedScope),
					},
				},
			},
			ReinvocationPolicy: common.PtrFrom(admissionv1.IfNeededReinvocationPolicy),
		},
	}

	return []client.Object{mutatingWebhookConfiguration}, nil
}

func validatingWCForKAIConfig(
	ctx context.Context, runtimeClient client.Reader, kaiConfig *kaiv1.Config,
	secret *v1.Secret, webhookName string,
) ([]client.Object, error) {
	validatingWebhookConfiguration := &admissionv1.ValidatingWebhookConfiguration{}
	err := runtimeClient.Get(ctx, types.NamespacedName{Name: validatorWebhookKAIAdmissionResourceName},
		validatingWebhookConfiguration)
	if client.IgnoreNotFound(err) != nil {
		return nil, err
	}
	validatingWebhookConfiguration.Name = validatorWebhookKAIAdmissionResourceName

	if validatingWebhookConfiguration.Labels == nil {
		validatingWebhookConfiguration.Labels = map[string]string{}
	}
	validatingWebhookConfiguration.Labels["app"] = mainResourceName

	namespaceSelector, objectSelector := buildWebhookSelectors(kaiConfig)
	clientConfig := buildWebhookClientConfig(kaiConfig, secret, "/validate--v1-pod")

	validatingWebhookConfiguration.Webhooks = []admissionv1.ValidatingWebhook{
		{
			Name:                    webhookName,
			AdmissionReviewVersions: []string{"v1"},
			SideEffects:             common.PtrFrom(admissionv1.SideEffectClassNone),
			NamespaceSelector:       namespaceSelector,
			ObjectSelector:          objectSelector,
			FailurePolicy:           common.PtrFrom(admissionv1.Fail),
			ClientConfig:            clientConfig,
			Rules: []admissionv1.RuleWithOperations{
				{
					Operations: []admissionv1.OperationType{admissionv1.Create, admissionv1.Update},
					Rule: admissionv1.Rule{
						APIGroups:   []string{""},
						APIVersions: []string{"v1"},
						Resources:   []string{"pods"},
						Scope:       common.PtrFrom(admissionv1.NamespacedScope),
					},
				},
			},
		},
	}

	return []client.Object{validatingWebhookConfiguration}, nil
}

func upsertKAIAdmissionCertSecret(ctx context.Context, runtimeClient client.Reader, kaiConfig *kaiv1.Config) (
	error, *v1.Secret, string) {
	secretObj, err := common.ObjectForKAIConfig(ctx, runtimeClient, &v1.Secret{},
		kaiAdmissionWebhookSecretName, kaiConfig.Spec.Namespace)
	if err != nil {
		return err, nil, ""
	}

	secret := secretObj.(*v1.Secret)
	secret.TypeMeta = metav1.TypeMeta{
		Kind:       "Secret",
		APIVersion: "v1",
	}
	webhookName := calculateServiceUrl(mainResourceName, kaiConfig.Spec.Namespace)
	if err = updateSelfSigned(secret, webhookName); err != nil {
		return err, nil, ""
	}
	return err, secret, webhookName
}

func updateSelfSigned(secret *v1.Secret, serviceUrl string) error {
	if _, found := secret.Data[certKey]; found {
		return nil
	}
	cert, key, err := generate.GenerateSelfSignedCert(serviceUrl, []string{serviceUrl})
	if err != nil {
		return err
	}
	secret.Data = map[string][]byte{
		certKey: cert,
		keyKey:  key,
	}
	return nil
}

func calculateServiceUrl(serviceName, namespace string) string {
	return fmt.Sprintf("%s.%s.svc", serviceName, namespace)
}

func buildArgsList(kaiConfig *kaiv1.Config, config *kaiv1admission.Admission) []string {
	args := []string{
		"--scheduler-name",
		*kaiConfig.Spec.Global.SchedulerName,
		"--webhook-addr",
		strconv.Itoa(*config.Webhook.TargetPort),
		"--health-probe-bind-address",
		fmt.Sprintf(":%d", *config.Webhook.ProbePort),
		"--metrics-bind-address",
		fmt.Sprintf(":%d", *config.Webhook.MetricsPort),
	}

	if config.GPUSharing != nil && *config.GPUSharing {
		args = append(args, "--gpu-sharing-enabled=true")
	}

	if config.Replicas != nil && *config.Replicas > 1 {
		args = append(args, "--leader-elect")
	}

	common.AddK8sClientConfigToArgs(config.Service.K8sClientConfig, args)

	return args
}
