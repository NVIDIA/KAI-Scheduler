// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package pod_info

import (
	"fmt"
	"strconv"
	"strings"

	v1 "k8s.io/api/core/v1"
	clientcache "k8s.io/client-go/tools/cache"

	commonconstants "github.com/NVIDIA/KAI-scheduler/pkg/common/constants"
	"github.com/NVIDIA/KAI-scheduler/pkg/common/resources"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/bindrequest_info"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/common_info"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/pod_status"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/resource_info"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/storageclaim_info"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/log"
)

const (
	GpuMemoryAnnotationName            = "gpu-memory"
	GPUGroup                           = "runai-gpu-group"
	ReceivedResourceTypeAnnotationName = "received-resource-type"
	WholeGpuIndicator                  = "-2"

	ResourceReservationPodPrefix     = "runai-reservation"
	ResourceReservationAppLabelValue = ResourceReservationPodPrefix
)

type ResourceRequestType string

const (
	RequestTypeGpuMemory   ResourceRequestType = "GpuMemory"
	RequestTypeMigInstance ResourceRequestType = "MigInstance"
	RequestTypeFraction    ResourceRequestType = "Fraction"
	RequestTypeRegular     ResourceRequestType = "Regular"
)

type ResourceReceivedType string

const (
	ReceivedTypeMigInstance ResourceReceivedType = "MigInstance"
	ReceivedTypeFraction    ResourceReceivedType = "Fraction"
	ReceivedTypeRegular     ResourceReceivedType = "Regular"
	ReceivedTypeNone        ResourceReceivedType = ""
)

type PodsMap map[common_info.PodID]*PodInfo

type PodInfo struct {
	UID common_info.PodID
	Job common_info.PodGroupID

	Name      string
	Namespace string

	ResourceRequestType  ResourceRequestType
	ResourceReceivedType ResourceReceivedType

	// ResReq are the minimal resources that needed to launch a pod. (includes init containers resources)
	ResReq           *resource_info.ResourceRequirements
	AcceptedResource *resource_info.ResourceRequirements

	schedulingConstraintsSignature common_info.SchedulingConstraintsSignature

	GPUGroups []string

	NodeName        string
	Status          pod_status.PodStatus
	IsVirtualStatus bool
	IsLegacyMIGtask bool

	BindRequest *bindrequest_info.BindRequestInfo

	ResourceClaimInfo bindrequest_info.ResourceClaimInfo

	// OwnedStorageClaims are StorageClaims that are owned exclusively by the pod, and we can count on them being deleted
	// if the pod is evicted
	ownedStorageClaims map[storageclaim_info.Key]*storageclaim_info.StorageClaimInfo

	// storageClaims are all storage claims used by the pod, with any status and any ownership situation. Not mutually exclusive
	// with ownedStorageClaims
	storageClaims map[storageclaim_info.Key]*storageclaim_info.StorageClaimInfo

	Pod *v1.Pod
}

func (pi *PodInfo) GetAllStorageClaims() map[storageclaim_info.Key]*storageclaim_info.StorageClaimInfo {
	return pi.storageClaims
}

func (pi *PodInfo) GetDeletedStorageClaimsNames() string {
	var deletedClaims []*storageclaim_info.StorageClaimInfo
	for _, claim := range pi.storageClaims {
		if claim.HasDeletedOwner() {
			deletedClaims = append(deletedClaims, claim)
		}
	}

	result := make([]string, len(deletedClaims))
	for _, claim := range deletedClaims {
		result = append(result, string(claim.Key))
	}

	return strings.Join(result, ", ")
}

func (pi *PodInfo) GetUnboundOrReleasingStorageClaimsByStorageClass() map[common_info.StorageClassID][]*storageclaim_info.StorageClaimInfo {
	result := map[common_info.StorageClassID][]*storageclaim_info.StorageClaimInfo{}

	for _, claim := range pi.GetAllStorageClaims() {
		if claim.Phase != v1.ClaimPending {
			continue
		}
		result[claim.StorageClass] = append(result[claim.StorageClass], claim)
	}

	// If a pod is pending virtually - it means that it was just evicted virtually. So all the owned storage claims are about
	// to be deleted - which means we need to consider them as pending.
	if pi.IsVirtualStatus && !pod_status.AllocatedStatus(pi.Status) {
		for _, claim := range pi.GetOwnedStorageClaims() {
			result[claim.StorageClass] = append(result[claim.StorageClass], claim)
		}
	}

	return result
}

func (pi *PodInfo) GetOwnedStorageClaims() map[storageclaim_info.Key]*storageclaim_info.StorageClaimInfo {
	return pi.ownedStorageClaims
}

func (pi *PodInfo) UpsertStorageClaim(claimInfo *storageclaim_info.StorageClaimInfo) {
	if claimInfo.PodOwnerReference != nil && claimInfo.PodOwnerReference.PodID == pi.UID {
		pi.ownedStorageClaims[claimInfo.Key] = claimInfo

		if pi.Pod != nil && pi.Pod.DeletionTimestamp == nil {
			claimInfo.MarkOwnerAlive()
		}
	}
	pi.storageClaims[claimInfo.Key] = claimInfo
}

func NewTaskInfo(pod *v1.Pod) *PodInfo {
	return NewTaskInfoWithBindRequest(pod, nil)
}

func NewTaskInfoWithBindRequest(pod *v1.Pod, bindRequest *bindrequest_info.BindRequestInfo) *PodInfo {
	initResreq := getPodResourceRequest(pod)

	nodeName := pod.Spec.NodeName
	if nodeName == "" && bindRequest != nil {
		nodeName = bindRequest.BindRequest.Spec.SelectedNode
	}

	podInfo := &PodInfo{
		UID:                            common_info.PodID(pod.UID),
		Job:                            getPodGroupID(pod),
		Name:                           pod.Name,
		Namespace:                      pod.Namespace,
		NodeName:                       nodeName,
		Status:                         getTaskStatus(pod, bindRequest),
		IsVirtualStatus:                false,
		IsLegacyMIGtask:                false,
		Pod:                            pod,
		ResReq:                         initResreq,
		AcceptedResource:               resource_info.EmptyResourceRequirements(),
		GPUGroups:                      []string{},
		ResourceRequestType:            RequestTypeRegular,
		ResourceReceivedType:           ReceivedTypeNone,
		BindRequest:                    bindRequest,
		schedulingConstraintsSignature: "",
		storageClaims:                  map[storageclaim_info.Key]*storageclaim_info.StorageClaimInfo{},
		ownedStorageClaims:             map[storageclaim_info.Key]*storageclaim_info.StorageClaimInfo{},
	}

	podInfo.updatePodAdditionalFields(bindRequest)
	return podInfo
}

func (pi *PodInfo) Clone() *PodInfo {
	return &PodInfo{
		UID:                  pi.UID,
		Job:                  pi.Job,
		Name:                 pi.Name,
		Namespace:            pi.Namespace,
		NodeName:             pi.NodeName,
		Status:               pi.Status,
		Pod:                  pi.Pod,
		ResReq:               pi.ResReq.Clone(),
		AcceptedResource:     pi.AcceptedResource.Clone(),
		GPUGroups:            pi.GPUGroups,
		ResourceClaimInfo:    pi.ResourceClaimInfo.Clone(),
		ResourceRequestType:  pi.ResourceRequestType,
		ResourceReceivedType: pi.ResourceReceivedType,
		IsVirtualStatus:      pi.IsVirtualStatus,
		IsLegacyMIGtask:      pi.IsLegacyMIGtask,
		storageClaims:        pi.storageClaims,
		ownedStorageClaims:   pi.ownedStorageClaims,
	}
}

func (pi PodInfo) String() string {
	return fmt.Sprintf("Pod (%v:%v/%v): job %v, status %v, resreq %v",
		pi.UID, pi.Namespace, pi.Name, pi.Job, pi.Status, pi.ResReq)
}

func (pi *PodInfo) IsMigProfileRequest() bool {
	return pi.ResourceRequestType == RequestTypeMigInstance
}

func (pi *PodInfo) IsMigProfileAllocation() bool {
	return pi.ResourceReceivedType == ReceivedTypeMigInstance
}

func (pi *PodInfo) IsFractionRequest() bool {
	return pi.ResourceRequestType == RequestTypeFraction
}

func (pi *PodInfo) IsFractionAllocation() bool {
	return pi.ResourceReceivedType == ReceivedTypeFraction
}

func (pi *PodInfo) IsFractionCandidate() bool {
	return pi.ResourceRequestType == RequestTypeFraction || pi.ResourceRequestType == RequestTypeGpuMemory
}

func (pi *PodInfo) IsMigCandidate() bool {
	return pi.ResourceRequestType == RequestTypeMigInstance
}

func (pi *PodInfo) IsMemoryRequest() bool {
	return pi.ResourceRequestType == RequestTypeGpuMemory
}

func (pi *PodInfo) IsRegularGPURequest() bool {
	return pi.ResourceRequestType == RequestTypeRegular
}

func (pi *PodInfo) IsSharedGPURequest() bool {
	return pi.IsFractionRequest() || pi.IsMemoryRequest()
}

func (pi *PodInfo) IsSharedGPUAllocation() bool {
	return pi.IsFractionAllocation()
}

func (pi *PodInfo) IsCPUOnlyRequest() bool {
	return !pi.IsRequireAnyKindOfGPU()
}

func (pi *PodInfo) IsRequireAnyKindOfGPU() bool {
	return pi.ResReq.GPUs() > 0 || pi.IsMemoryRequest() || pi.IsMigProfileRequest()
}

func (pi *PodInfo) IsKaiUtilityPod() bool {
	return pi.IsResourceReservationTask() || pi.IsScaleAdjustTask()
}

func (pi *PodInfo) IsScaleAdjustTask() bool {
	appName, found := pi.Pod.Labels[commonconstants.AppLabelName]
	return found && appName == commonconstants.ScalingPodAppLabelValue
}

func (pi *PodInfo) IsResourceReservationTask() bool {
	appName, found := pi.Pod.Labels[commonconstants.AppLabelName]
	return found && appName == ResourceReservationAppLabelValue
}

func (pi *PodInfo) GetSchedulingConstraintsSignature() common_info.SchedulingConstraintsSignature {
	if pi.schedulingConstraintsSignature == "" {
		pi.schedulingConstraintsSignature = schedulingConstraintsSignature(pi.Pod, pi.storageClaims)
	}
	return pi.schedulingConstraintsSignature
}

// PodKey returns the string key of a pod.
func PodKey(pod *v1.Pod) common_info.PodID {
	if key, err := clientcache.MetaNamespaceKeyFunc(pod); err != nil {
		return common_info.PodID(fmt.Sprintf("%v/%v", pod.Namespace, pod.Name))
	} else {
		return common_info.PodID(key)
	}
}

func getPodGroupID(pod *v1.Pod) common_info.PodGroupID {
	if gn, found := pod.Annotations[commonconstants.PodGroupAnnotationForPod]; found && len(gn) != 0 {
		return common_info.PodGroupID(gn)
	}

	return ""
}

func getPodResourceRequest(pod *v1.Pod) *resource_info.ResourceRequirements {
	result := getPodResourceWithoutInitContainers(pod)

	// take max_resource(sum_pod, any_init_container)
	for _, container := range pod.Spec.InitContainers {
		err := result.SetMaxResource(resource_info.RequirementsFromResourceList(container.Resources.Requests))
		if err != nil {
			log.InfraLogger.Errorf("Failed to calculate pod required resources for pod %s/%s. Error: %s",
				pod.Namespace, pod.Name, err.Error())
		}
	}

	return result
}

// getPodResourceWithoutInitContainers returns Pod's resource request, it does not contain
// init containers' resource request.
func getPodResourceWithoutInitContainers(pod *v1.Pod) *resource_info.ResourceRequirements {
	podResourcesList := v1.ResourceList{}
	for _, container := range pod.Spec.Containers {
		for key := range container.Resources.Requests {
			resourceSum := podResourcesList[key]
			resourceSum.Add(container.Resources.Requests[key])
			podResourcesList[key] = resourceSum
		}
	}

	return resource_info.RequirementsFromResourceList(podResourcesList)
}

func getTaskStatus(pod *v1.Pod, bindRequest *bindrequest_info.BindRequestInfo) pod_status.PodStatus {
	switch pod.Status.Phase {
	case v1.PodRunning:
		if pod.DeletionTimestamp != nil {
			return pod_status.Releasing
		}

		return pod_status.Running
	case v1.PodPending:
		if pod.DeletionTimestamp != nil {
			return pod_status.Releasing
		}

		if len(pod.Spec.NodeName) != 0 {
			return pod_status.Bound
		}

		if bindRequest != nil {

			return pod_status.Binding
		}

		return pod_status.Pending
	case v1.PodUnknown:
		return pod_status.Unknown
	case v1.PodSucceeded:
		return pod_status.Succeeded
	case v1.PodFailed:
		return pod_status.Failed
	}

	return pod_status.Unknown
}

func (pi *PodInfo) updatePodAdditionalFields(bindRequest *bindrequest_info.BindRequestInfo) {
	if len(pi.Job) == 0 {
		return
	}

	if bindRequest != nil && len(bindRequest.BindRequest.Spec.SelectedGPUGroups) > 0 {
		pi.GPUGroups = bindRequest.BindRequest.Spec.SelectedGPUGroups
	} else {
		pi.GPUGroups = resources.GetGpuGroups(pi.Pod)
	}

	if bindRequest != nil && len(bindRequest.BindRequest.Spec.ReceivedResourceType) > 0 {
		pi.ResourceReceivedType = ResourceReceivedType(bindRequest.BindRequest.Spec.ReceivedResourceType)
	} else {
		resourceReceivedType, found := pi.Pod.Annotations[ReceivedResourceTypeAnnotationName]
		if found && resourceReceivedType != string(ReceivedTypeNone) {
			pi.ResourceReceivedType = ResourceReceivedType(resourceReceivedType)
		}
	}

	gpuMemory, err := strconv.ParseInt(pi.Pod.Annotations[GpuMemoryAnnotationName], 10, 64)
	if err == nil && gpuMemory > 0 {
		pi.ResReq.GpuResourceRequirement =
			*resource_info.NewGpuResourceRequirementWithGpus(0, gpuMemory)
		pi.ResourceRequestType = RequestTypeGpuMemory
	}

	gpuFractionString := pi.Pod.Annotations[common_info.GPUFraction]
	gpuFraction, GPUFractionErr := strconv.ParseFloat(gpuFractionString, 64)
	if !(gpuFraction <= 0 || gpuFraction > 1 || GPUFractionErr != nil) {
		pi.ResReq.GpuResourceRequirement = *resource_info.NewGpuResourceRequirementWithGpus(gpuFraction, 0)
		pi.ResourceRequestType = RequestTypeFraction
	}

	if pi.ResourceRequestType == RequestTypeFraction || pi.ResourceRequestType == RequestTypeGpuMemory {
		numFractionDevicesStr, found := pi.Pod.Annotations[commonconstants.GpuFractionsNumDevices]
		if found && numFractionDevicesStr != "" {
			numFractionDevices, numFractionDevicesErr := strconv.ParseInt(numFractionDevicesStr, 10, 64)
			if numFractionDevicesErr == nil {
				pi.ResReq.GpuResourceRequirement = *resource_info.NewGpuResourceRequirementWithMultiFraction(
					numFractionDevices, gpuFraction, gpuMemory)
			}
		}
	}

	pi.updateLegacyMigResourceRequestFromAnnotations()
	if len(pi.ResReq.MigResources()) > 0 {
		pi.ResourceRequestType = RequestTypeMigInstance
	}
}

// updateLegacyMigResourceRequestFromAnnotations updates the mig resource request of legacy MIG pods
func (pi *PodInfo) updateLegacyMigResourceRequestFromAnnotations() {
	for annotationName, annotationValue := range pi.Pod.Annotations {
		if resource_info.IsMigResource(v1.ResourceName(annotationName)) {
			value, err := strconv.ParseInt(annotationValue, 10, 64)
			if err != nil {
				log.InfraLogger.V(2).Infof("Could not parse pod annotation of mig resource as int64. Annotation: %v, Value: %v", annotationName, annotationValue)
				continue
			}
			migResources := map[v1.ResourceName]int64{
				v1.ResourceName(annotationName): value,
			}
			pi.ResReq.GpuResourceRequirement = *resource_info.NewGpuResourceRequirementWithMig(migResources)
			pi.IsLegacyMIGtask = true
		}
	}
}

func (pi *PodInfo) ShouldAllocate(isRealAllocation bool) bool {
	return pi.Status == pod_status.Pending ||
		(!isRealAllocation && pi.Status == pod_status.Releasing && pi.IsVirtualStatus)
}
