// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package gpusharing

import (
	"context"
	"fmt"
	"strings"

	"golang.org/x/exp/slices"
	v1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/NVIDIA/KAI-scheduler/pkg/apis/scheduling/v1alpha2"
	"github.com/NVIDIA/KAI-scheduler/pkg/binder/common/gpusharingconfigmap"
	"github.com/NVIDIA/KAI-scheduler/pkg/common/resources"

	"github.com/NVIDIA/KAI-scheduler/pkg/binder/common"
	gpurequesthandler "github.com/NVIDIA/KAI-scheduler/pkg/binder/plugins/gpusharing/gpu-request"
	"github.com/NVIDIA/KAI-scheduler/pkg/binder/plugins/state"
)

const (
	fractionContainerIndex = 0
	CdiDeviceNameBase      = "k8s.device-plugin.nvidia.com/gpu=%s"
)

type GPUSharing struct {
	kubeClient             client.Client
	gpuDevicePluginUsesCdi bool
	gpuSharingEnabled      bool
}

func New(kubeClient client.Client, gpuDevicePluginUsesCdi bool, gpuSharingEnabled bool) *GPUSharing {
	return &GPUSharing{
		kubeClient:             kubeClient,
		gpuDevicePluginUsesCdi: gpuDevicePluginUsesCdi,
		gpuSharingEnabled:      gpuSharingEnabled,
	}
}

func (p *GPUSharing) Name() string {
	return "gpusharing"
}

func (p *GPUSharing) Validate(pod *v1.Pod) error {
	if !p.gpuSharingEnabled && resources.RequestsGPUFraction(pod) {
		return fmt.Errorf(
			"attempting to create a pod %s/%s with gpu sharing request, while GPU sharing is disabled",
			pod.Namespace, pod.Name,
		)
	}
	return gpurequesthandler.ValidateGpuRequests(pod)
}

func (p *GPUSharing) Mutate(pod *v1.Pod) error {
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
	capabilitiesConfigMapName := gpusharingconfigmap.SetGpuCapabilitiesConfigMapName(pod, containerRef)
	directEnvVarsMapName, err := gpusharingconfigmap.ExtractDirectEnvVarsConfigMapName(pod, containerRef)
	if err != nil {
		return err
	}

	common.AddVisibleDevicesEnvVars(containerRef.Container, capabilitiesConfigMapName)
	common.SetConfigMapVolume(pod, capabilitiesConfigMapName)
	common.AddDirectEnvVarsConfigMapSource(containerRef.Container, directEnvVarsMapName)

	return nil
}

func (p *GPUSharing) PreBind(
	ctx context.Context, pod *v1.Pod, _ *v1.Node, bindRequest *v1alpha2.BindRequest, state *state.BindingState,
) error {
	if !common.IsSharedGPUAllocation(bindRequest) {
		return nil
	}

	reservedGPUIds := slices.Clone(state.ReservedGPUIds)
	if p.gpuDevicePluginUsesCdi {
		for index, gpuIndex := range reservedGPUIds {
			reservedGPUIds[index] = fmt.Sprintf(CdiDeviceNameBase, gpuIndex)
		}
	}

	containerRef := &gpusharingconfigmap.PodContainerRef{
		Container: &pod.Spec.Containers[fractionContainerIndex],
		Index:     fractionContainerIndex,
		Type:      gpusharingconfigmap.RegularContainer,
	}
	err := p.createCapabilitiesConfigMapIfMissing(ctx, pod, containerRef)
	if err != nil {
		return fmt.Errorf("failed to create capabilities configmap: %w", err)
	}

	err = p.createDirectEnvMapIfMissing(ctx, pod, containerRef)
	if err != nil {
		return fmt.Errorf("failed to create env configmap: %w", err)
	}

	nVisibleDevicesStr := strings.Join(reservedGPUIds, ",")
	err = common.SetNvidiaVisibleDevices(ctx, p.kubeClient, pod, containerRef, nVisibleDevicesStr)
	if err != nil {
		return err
	}

	numOfGPUDevices := fmt.Sprintf("%v", bindRequest.Spec.ReceivedGPU.Portion)
	return common.SetNumOfGPUDevices(ctx, p.kubeClient, pod, containerRef, numOfGPUDevices)
}

func (p *GPUSharing) createCapabilitiesConfigMapIfMissing(ctx context.Context, pod *v1.Pod,
	containerRef *gpusharingconfigmap.PodContainerRef) error {
	capabilitiesConfigMapName, err := gpusharingconfigmap.ExtractCapabilitiesConfigMapName(pod, containerRef)
	if err != nil {
		return fmt.Errorf("failed to get capabilities configmap name: %w", err)
	}
	err = gpusharingconfigmap.UpsertJobConfigMap(ctx, p.kubeClient, pod, capabilitiesConfigMapName, map[string]string{})
	return err
}

func (p *GPUSharing) createDirectEnvMapIfMissing(ctx context.Context, pod *v1.Pod,
	containerRef *gpusharingconfigmap.PodContainerRef) error {
	directEnvVarsMapName, err := gpusharingconfigmap.ExtractDirectEnvVarsConfigMapName(pod, containerRef)
	if err != nil {
		return err
	}
	directEnvVars := make(map[string]string)
	return gpusharingconfigmap.UpsertJobConfigMap(ctx, p.kubeClient, pod, directEnvVarsMapName, directEnvVars)
}

func (p *GPUSharing) PostBind(
	context.Context, *v1.Pod, *v1.Node, *v1alpha2.BindRequest, *state.BindingState,
) {
}
