/*
Copyright 2025 NVIDIA CORPORATION
SPDX-License-Identifier: Apache-2.0
*/
package capacity

import (
	"context"
	"fmt"

	"github.com/NVIDIA/KAI-scheduler/pkg/common/constants"
	"github.com/NVIDIA/KAI-scheduler/pkg/common/k8s_utils"
	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	resourceapi "k8s.io/api/resource/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/dynamic-resource-allocation/cel"
)

// SkipIfInsufficientDynamicResources skips the test if there aren't enough nodes with the required devices.
func SkipIfInsufficientDynamicResources(clientset kubernetes.Interface, deviceClassName string, nodes, devicePerNode int) {
	devicesByNode := ListDevicesByNode(clientset, deviceClassName)

	fittingNodes := 0
	for _, devices := range devicesByNode {
		if devices >= devicePerNode {
			fittingNodes++
		}
	}

	if fittingNodes < nodes {
		ginkgo.Skip(fmt.Sprintf("Expected at least %d nodes with %d devices of deviceClass %s, found %d",
			nodes, devicePerNode, deviceClassName, fittingNodes))
	}
}

// ListDevicesByNode counts devices per node that match the given DeviceClass selectors (CEL).
// If the DeviceClass doesn't exist, the test is skipped. If it has no selectors, devices are counted by driver name only (same as previous behavior).
func ListDevicesByNode(clientset kubernetes.Interface, deviceClassName string) map[string]int {
	resourceSlices, err := clientset.ResourceV1().ResourceSlices().List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			ginkgo.Skip("DRA is not enabled in the cluster, skipping")
		}
		gomega.Expect(err).NotTo(gomega.HaveOccurred(), "Failed to list resource slices")
	}

	deviceClass, err := clientset.ResourceV1().DeviceClasses().Get(context.TODO(), deviceClassName, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			ginkgo.Skip(fmt.Sprintf("DeviceClass %s not found, skipping", deviceClassName))
		}
		gomega.Expect(err).NotTo(gomega.HaveOccurred(), fmt.Sprintf("Failed to get device class %s", deviceClassName))
	}

	features := k8s_utils.GetK8sFeatures()
	celCache := cel.NewCache(10, cel.Features{EnableConsumableCapacity: features.EnableConsumableCapacity})

	var compiledSelectors []cel.CompilationResult
	for _, selector := range deviceClass.Spec.Selectors {
		if selector.CEL != nil && selector.CEL.Expression != "" {
			compiled := celCache.GetOrCompile(selector.CEL.Expression)
			if compiled.Error != nil {
				gomega.Expect(compiled.Error).To(gomega.BeNil(), "Failed to compile CEL expression: %s", selector.CEL.Expression)
			}
			compiledSelectors = append(compiledSelectors, compiled)
		}
	}

	devicesByNode := map[string]int{}
	ctx := context.Background()

	for _, slice := range resourceSlices.Items {
		if slice.Spec.NodeName == nil || *slice.Spec.NodeName == "" {
			continue
		}

		nodeName := *slice.Spec.NodeName
		if _, ok := devicesByNode[nodeName]; !ok {
			devicesByNode[nodeName] = 0
		}

		for _, device := range slice.Spec.Devices {
			if len(compiledSelectors) == 0 {
				if slice.Spec.Driver == deviceClassName {
					devicesByNode[nodeName]++
				}
				continue
			}

			celDevice := convertToCelDevice(slice.Spec.Driver, device)
			// DeviceClass selectors are AND'ed: each selector must match.
			matches := true
			for _, compiled := range compiledSelectors {
				matched, _, err := compiled.DeviceMatches(ctx, celDevice)
				if err != nil {
					ginkgo.GinkgoWriter.Printf("Warning: Failed to evaluate CEL expression for device %s: %v\n", device.Name, err)
					matches = false
					break
				}
				if !matched {
					matches = false
					break
				}
			}

			if matches {
				devicesByNode[nodeName]++
			}
		}
	}

	return devicesByNode
}

// convertToCelDevice converts a ResourceSlice Device to cel.Device format.
func convertToCelDevice(driver string, device resourceapi.Device) cel.Device {
	attributes := make(map[resourceapi.QualifiedName]resourceapi.DeviceAttribute)
	capacity := make(map[resourceapi.QualifiedName]resourceapi.DeviceCapacity)

	for k, v := range device.Attributes {
		attributes[k] = v
	}

	for k, v := range device.Capacity {
		capacity[k] = v
	}

	return cel.Device{
		Driver:                   driver,
		Attributes:               attributes,
		Capacity:                 capacity,
		AllowMultipleAllocations: device.AllowMultipleAllocations,
	}
}

func CleanupResourceClaims(ctx context.Context, clientset kubernetes.Interface, namespace string) {
	err := clientset.ResourceV1().ResourceClaims(namespace).
		DeleteCollection(ctx, metav1.DeleteOptions{}, metav1.ListOptions{
			LabelSelector: fmt.Sprintf("%s=engine-e2e", constants.AppLabelName),
		})
	if err != nil {
		if !errors.IsNotFound(err) {
			gomega.Expect(err).To(gomega.Succeed(), "Failed to delete resource claim")
		}
	}
}
