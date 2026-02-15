// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package scheduler

import (
	"cmp"
	"slices"
	"strings"

	kaiv1 "github.com/NVIDIA/KAI-scheduler/pkg/apis/kai/v1"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/conf"
)

func resolvePlugins(plugins kaiv1.PluginConfigs) []conf.PluginOption {
	var result []conf.PluginOption
	for name, cfg := range plugins {
		if *cfg.Enabled {
			result = append(result, conf.PluginOption{
				Name:      name,
				Arguments: cfg.Arguments,
			})
		}
	}

	slices.SortFunc(result, func(a, b conf.PluginOption) int {
		if pa, pb := *plugins[a.Name].Priority, *plugins[b.Name].Priority; pa != pb {
			return pb - pa // descending
		}
		return cmp.Compare(a.Name, b.Name)
	})

	return result
}

func resolveActions(actions kaiv1.ActionConfigs) (string, []string) {
	var names []string
	for name, cfg := range actions {
		if *cfg.Enabled {
			names = append(names, name)
		}
	}

	slices.SortFunc(names, func(a, b string) int {
		if pa, pb := *actions[a].Priority, *actions[b].Priority; pa != pb {
			return pb - pa // descending
		}
		return cmp.Compare(a, b)
	})

	return strings.Join(names, ", "), names
}
