// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"strconv"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/version"
	featureutil "k8s.io/apiserver/pkg/util/feature"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/rest"
	"k8s.io/kubernetes/pkg/features"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	expectedVersion = "v1alpha3"
)

func SetDRAFeatureGate(mgr ctrl.Manager) {
	if !IsDynamicResourcesEnabled(mgr.GetConfig()) {
		return
	}

	_ = featureutil.DefaultMutableFeatureGate.OverrideDefault(features.DynamicResourceAllocation, true)
}

func IsDynamicResourcesEnabled(restConfig *rest.Config) bool {
	logger := log.Log.WithName("feature-gates")
	// Create a DiscoveryClient
	discoveryClient, err := discovery.NewDiscoveryClientForConfig(restConfig)
	if err != nil {
		logger.Error(err, "Failed to create discovery client")
		return false
	}

	// Get API server version
	serverVersion, err := discoveryClient.ServerVersion()
	if err != nil {
		logger.Error(err, "Failed to get server version")
		return false
	}

	// Check if the API server version is compatible with DRA
	if majorVer, errMajor := strconv.Atoi(serverVersion.Major); errMajor != nil || majorVer < 1 {
		return false
	}
	if minorVer, errMinor := strconv.Atoi(serverVersion.Minor); errMinor != nil || minorVer < 26 {
		return false
	}

	// Get supported API versions
	serverGroups, err := discoveryClient.ServerGroups()
	if err != nil {
		logger.Error(err, "Failed to get server groups")
		return false
	}

	found := false
	var resourceGroup v1.APIGroup
	for _, group := range serverGroups.Groups {
		if group.Name == "resource.k8s.io" {
			resourceGroup = group
			found = true
			break
		}
	}
	if !found {
		return false
	}

	// Check if the DRA API group is supported
	for _, groupVersion := range resourceGroup.Versions {
		if version.CompareKubeAwareVersionStrings(groupVersion.Version, expectedVersion) >= 0 {
			return true
		}
	}

	return false
}
