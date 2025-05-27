/*
Copyright 2025 NVIDIA CORPORATION
SPDX-License-Identifier: Apache-2.0
*/
package rd

import (
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	"github.com/NVIDIA/KAI-scheduler/pkg/common/constants"
	"github.com/NVIDIA/KAI-scheduler/test/e2e/modules/constant"
	"github.com/NVIDIA/KAI-scheduler/test/e2e/modules/utils"
)

const DeploymentAppLabel = "deployment-app-name"

func CreateDeploymentObject(namespace, queueName string) *appsv1.Deployment {
	matchLabelValue := utils.GenerateRandomK8sName(10)

	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      utils.GenerateRandomK8sName(10),
			Namespace: namespace,
			Labels: map[string]string{
				constants.AppLabelName: "engine-e2e",
				DeploymentAppLabel:     matchLabelValue,
			},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: ptr.To(int32(1)),
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					constants.AppLabelName: "engine-e2e",
					DeploymentAppLabel:     matchLabelValue,
				},
			},
			Template: v1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						constants.AppLabelName: "engine-e2e",
						DeploymentAppLabel:     matchLabelValue,
						"runai/queue":          queueName,
					},
				},
				Spec: v1.PodSpec{
					Containers: []v1.Container{
						{
							Image: "ubuntu",
							Name:  "ubuntu-container",
							Args: []string{
								"sleep",
								"infinity",
							},
							SecurityContext: DefaultSecurityContext(),
						},
					},
					TerminationGracePeriodSeconds: ptr.To(int64(0)),
					SchedulerName:                 constant.SchedulerName,
					Tolerations: []v1.Toleration{
						{
							Key:      "nvidia.com/gpu",
							Operator: v1.TolerationOpExists,
							Effect:   v1.TaintEffectNoSchedule,
						},
					},
				},
			},
		},
	}
}
