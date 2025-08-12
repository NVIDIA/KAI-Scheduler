// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package usagedb

import (
	"fmt"
	"maps"

	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/cache/usagedb/api"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/cache/usagedb/fake"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/log"
)

type GetClientFn func(connectionString string) (api.Interface, error)

func GetClient(config *api.UsageDBConfig) (api.Interface, error) {
	if config == nil {
		return nil, nil
	}

	if config.ClientType == "" {
		return nil, fmt.Errorf("client type cannot be empty")
	}

	clientMap := map[string]GetClientFn{
		"fake": fake.NewFakeClient,
	}

	client, ok := clientMap[config.ClientType]
	if !ok {
		return nil, fmt.Errorf("unknown client type: %s, supported types: %v", config.ClientType, maps.Keys(clientMap))
	}

	log.InfraLogger.V(3).Infof("getting usage db client of type: %s, connection string: %s", config.ClientType, config.ConnectionString)

	return client(config.ConnectionString)
}
