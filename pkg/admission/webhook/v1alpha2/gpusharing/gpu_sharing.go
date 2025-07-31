// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package gpusharing

import (
	"context"
	"fmt"

	v1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/NVIDIA/KAI-scheduler/pkg/binder/common/gpusharingconfigmap"
	"github.com/NVIDIA/KAI-scheduler/pkg/common/resources"

	"github.com/NVIDIA/KAI-scheduler/pkg/binder/common"
	gpurequesthandler "github.com/NVIDIA/KAI-scheduler/pkg/binder/plugins/gpusharing/gpu-request"
)

const (
	fractionContainerIndex = 0
	CdiDeviceNameBase      = "k8s.device-plugin.nvidia.com/gpu=%s"
)

type AdmissionGPUSharing struct {
	kubeClient             client.Client
	gpuDevicePluginUsesCdi bool
	gpuSharingEnabled      bool
}

func New(kubeClient client.Client, gpuDevicePluginUsesCdi bool, gpuSharingEnabled bool) *AdmissionGPUSharing {
	return &AdmissionGPUSharing{
		kubeClient:             kubeClient,
		gpuDevicePluginUsesCdi: gpuDevicePluginUsesCdi,
		gpuSharingEnabled:      gpuSharingEnabled,
	}
}

func (p *AdmissionGPUSharing) Name() string {
	return "gpusharing"
}

func (p *AdmissionGPUSharing) Validate(pod *v1.Pod) error {
	logger := log.FromContext(context.Background())
	logger.Info("gpusharing validating pod", "pod", pod.Name, "plugin", p.Name())
	if !p.gpuSharingEnabled && resources.RequestsGPUFraction(pod) {
		return fmt.Errorf(
			"attempting to create a pod %s/%s with gpu sharing request, while GPU sharing is disabled",
			pod.Namespace, pod.Name,
		)
	}
	return gpurequesthandler.ValidateGpuRequests(pod)
}

func (p *AdmissionGPUSharing) Mutate(pod *v1.Pod) error {
	if len(pod.Spec.Containers) == 0 {
		return nil
	}

	if !resources.RequestsGPUFraction(pod) {
		return nil
	}

	containerRef := &gpusharingconfigmap.PodContainerRef{
		Container: &pod.Spec.Containers[fractionContainerIndex],
		Index:     fractionContainerIndex,
		Type:      gpusharingconfigmap.RegularContainer,
	}
	logger := log.FromContext(context.Background())
	logger.Info("gpusharing mutating pod", "pod", pod.Name, "plugin", p.Name(), "container", containerRef, "index", containerRef.Index)
	capabilitiesConfigMapName := gpusharingconfigmap.SetGpuCapabilitiesConfigMapName(pod, containerRef)
	directEnvVarsMapName, err := gpusharingconfigmap.ExtractDirectEnvVarsConfigMapName(pod, containerRef)
	if err != nil {
		return err
	}

	common.AddGPUSharingEnvVars(containerRef.Container, capabilitiesConfigMapName)
	common.SetConfigMapVolume(pod, capabilitiesConfigMapName)
	common.AddDirectEnvVarsConfigMapSource(containerRef.Container, directEnvVarsMapName)

	return nil
}
