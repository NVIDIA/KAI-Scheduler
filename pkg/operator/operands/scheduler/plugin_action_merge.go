// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package scheduler

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	kaiv1 "github.com/NVIDIA/KAI-scheduler/pkg/apis/kai/v1"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/conf"
)

type pluginWithPriority struct {
	option   conf.PluginOption
	priority int
	enabled  bool
}

type actionWithPriority struct {
	name     string
	priority int
	enabled  bool
}

// Default priorities preserve the current hardcoded ordering.
// Higher priority = runs first. Spaced by 100.
var defaultPluginPriorities = map[string]int{
	"predicates":       1900,
	"proportion":       1800,
	"priority":         1700,
	"nodeavailability": 1600,
	"resourcetype":     1500,
	"podaffinity":      1400,
	"elastic":          1300,
	"kubeflow":         1200,
	"ray":              1100,
	"subgrouporder":    1000,
	"taskorder":        900,
	"nominatednode":    800,
	"dynamicresources": 700,
	"minruntime":       600,
	"topology":         500,
	"snapshot":         400,
	"gpupack":          300,
	"gpuspread":        300,
	"nodeplacement":    200,
	"gpusharingorder":  100,
}

var defaultActionPriorities = map[string]int{
	"allocate":          500,
	"consolidation":    400,
	"reclaim":          300,
	"preempt":          200,
	"stalegangeviction": 100,
}

// buildDefaultPlugins creates the default plugin set based on shard spec,
// matching the current hardcoded logic in configMapForShard.
func buildDefaultPlugins(spec *kaiv1.SchedulingShardSpec) map[string]*pluginWithPriority {
	plugins := make(map[string]*pluginWithPriority)

	// Base plugins (always present)
	basePlugins := []string{
		"predicates", "proportion", "priority", "nodeavailability",
		"resourcetype", "podaffinity", "elastic", "kubeflow", "ray",
		"subgrouporder", "taskorder", "nominatednode", "dynamicresources",
		"minruntime", "topology", "snapshot",
	}

	for _, name := range basePlugins {
		plugins[name] = &pluginWithPriority{
			option:   conf.PluginOption{Name: name},
			priority: defaultPluginPriorities[name],
			enabled:  true,
		}
	}

	// proportion: set kValue argument if configured
	if spec.KValue != nil {
		plugins["proportion"].option.Arguments = map[string]string{
			"kValue": strconv.FormatFloat(*spec.KValue, 'f', -1, 64),
		}
	}

	// minruntime: set arguments if configured
	if spec.MinRuntime != nil && (spec.MinRuntime.PreemptMinRuntime != nil || spec.MinRuntime.ReclaimMinRuntime != nil) {
		minRuntimeArgs := make(map[string]string)
		if spec.MinRuntime.PreemptMinRuntime != nil {
			minRuntimeArgs["defaultPreemptMinRuntime"] = *spec.MinRuntime.PreemptMinRuntime
		}
		if spec.MinRuntime.ReclaimMinRuntime != nil {
			minRuntimeArgs["defaultReclaimMinRuntime"] = *spec.MinRuntime.ReclaimMinRuntime
		}
		plugins["minruntime"].option.Arguments = minRuntimeArgs
	}

	// Placement strategy determines gpu plugin
	placementArguments := calculatePlacementArguments(spec.PlacementStrategy)
	gpuPluginName := fmt.Sprintf("gpu%s", strings.Replace(placementArguments[gpuResource], "bin", "", 1))
	plugins[gpuPluginName] = &pluginWithPriority{
		option:   conf.PluginOption{Name: gpuPluginName},
		priority: defaultPluginPriorities[gpuPluginName],
		enabled:  true,
	}

	// nodeplacement with placement arguments
	plugins["nodeplacement"] = &pluginWithPriority{
		option: conf.PluginOption{
			Name:      "nodeplacement",
			Arguments: placementArguments,
		},
		priority: defaultPluginPriorities["nodeplacement"],
		enabled:  true,
	}

	// gpusharingorder only with binpack GPU strategy
	if placementArguments[gpuResource] == binpackStrategy {
		plugins["gpusharingorder"] = &pluginWithPriority{
			option:   conf.PluginOption{Name: "gpusharingorder"},
			priority: defaultPluginPriorities["gpusharingorder"],
			enabled:  true,
		}
	}

	return plugins
}

// buildDefaultActions creates the default action set based on shard spec.
func buildDefaultActions(spec *kaiv1.SchedulingShardSpec) map[string]*actionWithPriority {
	actions := make(map[string]*actionWithPriority)

	// Always present
	for _, name := range []string{"allocate", "reclaim", "preempt", "stalegangeviction"} {
		actions[name] = &actionWithPriority{
			name:     name,
			priority: defaultActionPriorities[name],
			enabled:  true,
		}
	}

	// consolidation: only if neither GPU nor CPU is spread
	placementArguments := calculatePlacementArguments(spec.PlacementStrategy)
	if placementArguments[gpuResource] != spreadStrategy && placementArguments[cpuResource] != spreadStrategy {
		actions["consolidation"] = &actionWithPriority{
			name:     "consolidation",
			priority: defaultActionPriorities["consolidation"],
			enabled:  true,
		}
	}

	return actions
}

// mergePluginOverrides applies user overrides to the default plugin map.
func mergePluginOverrides(defaults map[string]*pluginWithPriority, overrides map[string]kaiv1.PluginConfig) {
	for name, override := range overrides {
		existing, found := defaults[name]
		if !found {
			// New plugin: default priority 0, enabled true
			existing = &pluginWithPriority{
				option:   conf.PluginOption{Name: name},
				priority: 0,
				enabled:  true,
			}
			defaults[name] = existing
		}

		if override.Enabled != nil {
			existing.enabled = *override.Enabled
		}
		if override.Priority != nil {
			existing.priority = *override.Priority
		}
		if override.Arguments != nil {
			existing.option.Arguments = override.Arguments
		}
	}
}

// mergeActionOverrides applies user overrides to the default action map.
func mergeActionOverrides(defaults map[string]*actionWithPriority, overrides map[string]kaiv1.ActionConfig) {
	for name, override := range overrides {
		existing, found := defaults[name]
		if !found {
			// New action: default priority 0, enabled true
			existing = &actionWithPriority{
				name:     name,
				priority: 0,
				enabled:  true,
			}
			defaults[name] = existing
		}

		if override.Enabled != nil {
			existing.enabled = *override.Enabled
		}
		if override.Priority != nil {
			existing.priority = *override.Priority
		}
	}
}

// resolvePlugins sorts by priority descending, filters disabled, and returns the plugin list.
// Alphabetical tiebreak for stability.
func resolvePlugins(plugins map[string]*pluginWithPriority) []conf.PluginOption {
	var enabled []*pluginWithPriority
	for _, p := range plugins {
		if p.enabled {
			enabled = append(enabled, p)
		}
	}

	sort.Slice(enabled, func(i, j int) bool {
		if enabled[i].priority != enabled[j].priority {
			return enabled[i].priority > enabled[j].priority
		}
		return enabled[i].option.Name < enabled[j].option.Name
	})

	result := make([]conf.PluginOption, len(enabled))
	for i, p := range enabled {
		result[i] = p.option
	}
	return result
}

// resolveActions sorts by priority descending, filters disabled, and returns
// the comma-separated actions string and the action name slice.
// Alphabetical tiebreak for stability.
func resolveActions(actions map[string]*actionWithPriority) (string, []string) {
	var enabled []*actionWithPriority
	for _, a := range actions {
		if a.enabled {
			enabled = append(enabled, a)
		}
	}

	sort.Slice(enabled, func(i, j int) bool {
		if enabled[i].priority != enabled[j].priority {
			return enabled[i].priority > enabled[j].priority
		}
		return enabled[i].name < enabled[j].name
	})

	names := make([]string, len(enabled))
	for i, a := range enabled {
		names[i] = a.name
	}

	return strings.Join(names, ", "), names
}
