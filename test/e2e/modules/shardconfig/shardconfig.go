/*
Copyright 2025 NVIDIA CORPORATION
SPDX-License-Identifier: Apache-2.0
*/
package shardconfig

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
)

// ShardConfig stores configuration for a specific schedulingShard
type ShardConfig struct {
	ShardName           string
	PartitionLabelKey   string // e.g., "kai.scheduler/node-pool"
	PartitionLabelValue string // e.g., "test-pool-2"

	labelSelector       labels.Selector
	labelSelectorString string
}

// Package-level shard configuration (nil for default/process 1)
var currentShardConfig *ShardConfig = nil

// GetCurrentShardConfig returns the current shard configuration (nil if using default shard)
func GetCurrentShardConfig() *ShardConfig {
	return currentShardConfig
}

// SetCurrentShardConfig sets the current shard configuration
func SetCurrentShardConfig(config *ShardConfig) {
	currentShardConfig = config
}

func GetShardLabelSelector() labels.Selector {
	if currentShardConfig == nil || currentShardConfig.PartitionLabelKey == "" {
		Fail("Failed to get shard label selector: currentShardConfig is nil or PartitionLabelKey is empty")
		return nil
	}
	if currentShardConfig.labelSelector != nil {
		return currentShardConfig.labelSelector
	}

	operator := selection.DoesNotExist
	var vals []string
	if len(currentShardConfig.PartitionLabelValue) > 0 {
		operator = selection.Equals
		vals = []string{currentShardConfig.PartitionLabelValue}
	}

	requirement, err := labels.NewRequirement(currentShardConfig.PartitionLabelKey, operator, vals)
	if err != nil {
		Expect(err).NotTo(HaveOccurred(), "Failed to create shard label selector")
		return nil
	}
	selector := labels.NewSelector().Add(*requirement)
	currentShardConfig.labelSelector = selector
	return selector
}

func GetShardLabelSelectorString() string {
	if currentShardConfig == nil || currentShardConfig.PartitionLabelKey == "" {
		Fail("Failed to get shard label selector string: currentShardConfig is nil or PartitionLabelKey is empty")
		return ""
	}
	if currentShardConfig.labelSelectorString == "" {
		currentShardConfig.labelSelectorString = GetShardLabelSelector().String()
	}
	return currentShardConfig.labelSelectorString
}

func AddShardLabels(labels map[string]string) map[string]string {
	if currentShardConfig == nil || currentShardConfig.PartitionLabelKey == "" || currentShardConfig.PartitionLabelValue == "" {
		return labels
	}
	labels[currentShardConfig.PartitionLabelKey] = currentShardConfig.PartitionLabelValue
	return labels
}
