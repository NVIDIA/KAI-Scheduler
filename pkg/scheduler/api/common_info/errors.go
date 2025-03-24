// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package common_info

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/dustin/go-humanize"

	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/resource_info"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/k8s_internal"
)

const (
	ResourcesWereNotFoundMsg = "no nodes with enough resources were found"
	DefaultPodgroupError     = "Unable to schedule podgroup"
	DefaultPodError          = "Unable to schedule pod"
)

type FitError struct {
	taskNamespace   string
	taskName        string
	NodeName        string
	Reasons         []string
	DetailedReasons []string
}

func NewFitErrorWithDetailedMessage(name, namespace, nodeName string, reasons []string, detailedReasons ...string) *FitError {
	fe := &FitError{
		taskName:        name,
		taskNamespace:   namespace,
		NodeName:        nodeName,
		Reasons:         reasons,
		DetailedReasons: detailedReasons,
	}

	if len(detailedReasons) == 0 {
		fe.DetailedReasons = reasons
	}

	return fe
}

func NewFitError(name, namespace, nodeName string, message string) *FitError {
	return NewFitErrorWithDetailedMessage(name, namespace, nodeName, []string{message})
}

func NewFitErrorByReasons(name, namespace, nodeName string, err error, reasons ...string) *FitError {
	message := reasons
	if len(message) == 0 && err != nil {
		message = []string{err.Error()}
	}
	return NewFitErrorWithDetailedMessage(name, namespace, nodeName, message)
}

func NewFitErrorInsufficientResource(
	name, namespace, nodeName string,
	resourceRequested *resource_info.ResourceRequirements, usedResource, capacityResource *resource_info.Resource,
	capacityGpuMemory int64, gangSchedulingJob bool,
) *FitError {
	availableResource := capacityResource.Clone()
	availableResource.Sub(usedResource)
	var shortMessages []string
	var detailedMessages []string

	if len(resourceRequested.MIGResources) > 0 {
		for migProfile, quant := range resourceRequested.MIGResources {
			availableMigProfilesQuant := int64(0)
			capacityMigProfilesQuant := int64(0)
			if _, found := availableResource.ScalarResources[migProfile]; found {
				availableMigProfilesQuant = availableResource.ScalarResources[migProfile]
				capacityMigProfilesQuant = capacityResource.ScalarResources[migProfile]
			}
			if availableMigProfilesQuant < quant {
				detailedMessages = append(detailedMessages, k8s_internal.NewInsufficientResourceErrorScalarResources(
					migProfile,
					quant,
					usedResource.ScalarResources[migProfile],
					capacityMigProfilesQuant,
					gangSchedulingJob))
				shortMessages = append(shortMessages, fmt.Sprintf("node(s) didn't have enough of mig profile: %s",
					migProfile))
			}
		}
	} else {
		requestedGPUs := resourceRequested.GPUs()
		availableGPUs := availableResource.GPUs
		if requestedGPUs > availableGPUs {
			detailedMessages = append(detailedMessages, k8s_internal.NewInsufficientResourceError(
				"GPUs",
				generateRequestedGpuString(resourceRequested),
				strconv.FormatFloat(usedResource.GPUs, 'g', 3, 64),
				strconv.FormatFloat(capacityResource.GPUs, 'g', 3, 64),
				gangSchedulingJob))
			shortMessages = append(shortMessages, "node(s) didn't have enough resources: GPUs")
		}

		if resourceRequested.GpuMemory() > capacityGpuMemory {
			detailedMessages = append(detailedMessages, k8s_internal.NewInsufficientGpuMemoryCapacity(
				resourceRequested.GpuMemory(), capacityGpuMemory, gangSchedulingJob))
			shortMessages = append(shortMessages, "node(s) didn't have enough resources: GPU memory")
		}
	}

	requestedCPUs := int64(resourceRequested.CPUMilliCores)
	availableCPUs := int64(availableResource.CPUMilliCores)
	if requestedCPUs > availableCPUs {
		detailedMessages = append(detailedMessages, k8s_internal.NewInsufficientResourceError(
			"CPU cores",
			humanize.FtoaWithDigits(resourceRequested.CPUMilliCores/resource_info.MilliCPUToCores, 3),
			humanize.FtoaWithDigits(usedResource.CPUMilliCores/resource_info.MilliCPUToCores, 3),
			humanize.FtoaWithDigits(capacityResource.CPUMilliCores/resource_info.MilliCPUToCores, 3),
			gangSchedulingJob))
		shortMessages = append(shortMessages, "node(s) didn't have enough resources: CPU cores")
	}

	if resourceRequested.MemoryBytes > availableResource.MemoryBytes {
		detailedMessages = append(detailedMessages, k8s_internal.NewInsufficientResourceError(
			"memory",
			humanize.FtoaWithDigits(resourceRequested.MemoryBytes/resource_info.MemoryToGB, 3),
			humanize.FtoaWithDigits(usedResource.MemoryBytes/resource_info.MemoryToGB, 3),
			humanize.FtoaWithDigits(capacityResource.MemoryBytes/resource_info.MemoryToGB, 3),
			gangSchedulingJob))
		shortMessages = append(shortMessages, "node(s) didn't have enough resources: memory")
	}

	for requestedResourceName, requestedResourceQuant := range resourceRequested.ScalarResources {
		availableResourceQuant := int64(0)
		capacityResourceQuant := int64(0)
		if _, found := availableResource.ScalarResources[requestedResourceName]; found {
			availableResourceQuant = availableResource.ScalarResources[requestedResourceName]
			capacityResourceQuant = capacityResource.ScalarResources[requestedResourceName]
		}
		if availableResourceQuant < requestedResourceQuant {
			detailedMessages = append(detailedMessages, k8s_internal.NewInsufficientResourceErrorScalarResources(
				requestedResourceName,
				requestedResourceQuant,
				usedResource.ScalarResources[requestedResourceName], capacityResourceQuant,
				gangSchedulingJob))
			shortMessages = append(shortMessages, fmt.Sprintf("node(s) didn't have enough resources: %s",
				requestedResourceName))
		}
	}

	return NewFitErrorWithDetailedMessage(name, namespace, nodeName, shortMessages, detailedMessages...)
}

func generateRequestedGpuString(resourceRequested *resource_info.ResourceRequirements) string {
	var requestedGpuString string
	if resourceRequested.IsFractionalRequest() && resourceRequested.GetNumOfGpuDevices() > 1 {
		requestedGpuString = fmt.Sprintf("%d X %s", resourceRequested.GetNumOfGpuDevices(),
			strconv.FormatFloat(resourceRequested.GpuFractionalPortion(), 'g', 3, 64))
	} else {
		requestedGpuString = strconv.FormatFloat(resourceRequested.GPUs(), 'g', 3, 64)
	}
	return requestedGpuString
}

func (f *FitError) Error() string {
	return fmt.Sprintf("Pod %s/%s cannot be scheduled on node %s. reasons: %s", f.taskNamespace, f.taskName,
		f.NodeName, strings.Join(f.Reasons, ". \n"))
}

type FitErrors struct {
	nodes map[string]*FitError
	err   string
}

func NewFitErrors() *FitErrors {
	f := new(FitErrors)
	f.nodes = make(map[string]*FitError)
	return f
}

func (f *FitErrors) SetError(err string) {
	f.err = err
}

func (f *FitErrors) SetNodeError(nodeName string, err error) {
	var fe *FitError
	switch obj := err.(type) {
	case *FitError:
		obj.NodeName = nodeName
		fe = obj
	default:
		fe = NewFitError("", "", nodeName, err.Error())
	}

	f.nodes[nodeName] = fe
}

func (f *FitErrors) AddNodeErrors(errors *FitErrors) {
	for nodeName, fitError := range errors.nodes {
		f.nodes[nodeName] = fitError
	}
}

func (f *FitErrors) DetailedError() string {
	if f.err == "" {
		f.err = ResourcesWereNotFoundMsg
	}
	reasonMessages := []string{"\n" + f.err + "."}
	for _, node := range f.nodes {
		reasonMessages = append(reasonMessages,
			fmt.Sprintf("\n<%v>: %v.", node.NodeName, strings.Join(node.DetailedReasons, ", ")))
	}
	sort.Strings(reasonMessages)
	return strings.Join(reasonMessages, "")
}

func (f *FitErrors) Error() string {
	reasons := make(map[string]int)

	sortReasonsHistogram := func() []string {
		for _, node := range f.nodes {
			for _, reason := range node.Reasons {
				reasons[reason]++
			}
		}

		var reasonStrings []string
		for k, v := range reasons {
			reasonStrings = append(reasonStrings, fmt.Sprintf("%v %v", v, k))
		}
		sort.Strings(reasonStrings)
		return reasonStrings
	}
	if f.err == "" {
		f.err = ResourcesWereNotFoundMsg
	}
	reasonMsg := f.err

	nodeReasonsHistogram := sortReasonsHistogram()
	if len(nodeReasonsHistogram) > 0 {
		reasonMsg += fmt.Sprintf(": %v.", strings.Join(nodeReasonsHistogram, ". \n"))
	}
	return reasonMsg
}

type NotFoundError struct {
	Name string
}

func (e *NotFoundError) Error() string { return e.Name + ": not found" }
