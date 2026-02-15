// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package scheduler

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/utils/ptr"

	kaiv1 "github.com/NVIDIA/KAI-scheduler/pkg/apis/kai/v1"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/conf"
)

func TestBuildDefaultPlugins(t *testing.T) {
	spec := &kaiv1.SchedulingShardSpec{
		PlacementStrategy: &kaiv1.PlacementStrategy{
			GPU: ptr.To(binpackStrategy),
			CPU: ptr.To(binpackStrategy),
		},
	}

	plugins := buildDefaultPlugins(spec)

	// Verify base plugins are present
	for _, name := range []string{
		"predicates", "proportion", "priority", "nodeavailability",
		"resourcetype", "podaffinity", "elastic", "kubeflow", "ray",
		"subgrouporder", "taskorder", "nominatednode", "dynamicresources",
		"minruntime", "topology", "snapshot",
	} {
		p, found := plugins[name]
		require.True(t, found, "expected plugin %s to be present", name)
		assert.True(t, p.enabled, "expected plugin %s to be enabled", name)
	}

	// Verify gpupack is present for binpack strategy
	_, found := plugins["gpupack"]
	assert.True(t, found, "expected gpupack for binpack strategy")

	// Verify gpusharingorder is present for binpack strategy
	_, found = plugins["gpusharingorder"]
	assert.True(t, found, "expected gpusharingorder for binpack strategy")

	// Verify nodeplacement has placement arguments
	np := plugins["nodeplacement"]
	assert.Equal(t, map[string]string{"gpu": "binpack", "cpu": "binpack"}, np.option.Arguments)
}

func TestBuildDefaultPlugins_SpreadStrategy(t *testing.T) {
	spec := &kaiv1.SchedulingShardSpec{
		PlacementStrategy: &kaiv1.PlacementStrategy{
			GPU: ptr.To(spreadStrategy),
			CPU: ptr.To(binpackStrategy),
		},
	}

	plugins := buildDefaultPlugins(spec)

	// Verify gpuspread is present for spread strategy
	_, found := plugins["gpuspread"]
	assert.True(t, found, "expected gpuspread for spread strategy")

	// Verify gpupack is NOT present
	_, found = plugins["gpupack"]
	assert.False(t, found, "expected gpupack to be absent for spread strategy")

	// Verify gpusharingorder is NOT present for spread strategy
	_, found = plugins["gpusharingorder"]
	assert.False(t, found, "expected gpusharingorder to be absent for spread strategy")
}

func TestBuildDefaultPlugins_WithKValue(t *testing.T) {
	spec := &kaiv1.SchedulingShardSpec{
		KValue: ptr.To(1.5),
		PlacementStrategy: &kaiv1.PlacementStrategy{
			GPU: ptr.To(binpackStrategy),
			CPU: ptr.To(binpackStrategy),
		},
	}

	plugins := buildDefaultPlugins(spec)
	assert.Equal(t, map[string]string{"kValue": "1.5"}, plugins["proportion"].option.Arguments)
}

func TestBuildDefaultPlugins_WithMinRuntime(t *testing.T) {
	spec := &kaiv1.SchedulingShardSpec{
		MinRuntime: &kaiv1.MinRuntime{
			PreemptMinRuntime: ptr.To("5m"),
			ReclaimMinRuntime: ptr.To("10m"),
		},
		PlacementStrategy: &kaiv1.PlacementStrategy{
			GPU: ptr.To(binpackStrategy),
			CPU: ptr.To(binpackStrategy),
		},
	}

	plugins := buildDefaultPlugins(spec)
	assert.Equal(t, map[string]string{
		"defaultPreemptMinRuntime": "5m",
		"defaultReclaimMinRuntime": "10m",
	}, plugins["minruntime"].option.Arguments)
}

func TestBuildDefaultActions(t *testing.T) {
	spec := &kaiv1.SchedulingShardSpec{
		PlacementStrategy: &kaiv1.PlacementStrategy{
			GPU: ptr.To(binpackStrategy),
			CPU: ptr.To(binpackStrategy),
		},
	}

	actions := buildDefaultActions(spec)

	// All actions should be present for binpack strategy
	for _, name := range []string{"allocate", "consolidation", "reclaim", "preempt", "stalegangeviction"} {
		a, found := actions[name]
		require.True(t, found, "expected action %s", name)
		assert.True(t, a.enabled)
	}
}

func TestBuildDefaultActions_SpreadStrategy(t *testing.T) {
	spec := &kaiv1.SchedulingShardSpec{
		PlacementStrategy: &kaiv1.PlacementStrategy{
			GPU: ptr.To(spreadStrategy),
			CPU: ptr.To(binpackStrategy),
		},
	}

	actions := buildDefaultActions(spec)

	// consolidation should be absent when GPU is spread
	_, found := actions["consolidation"]
	assert.False(t, found, "consolidation should be absent for spread strategy")
}

func TestMergePluginOverrides_NoOverrides(t *testing.T) {
	spec := &kaiv1.SchedulingShardSpec{
		PlacementStrategy: &kaiv1.PlacementStrategy{
			GPU: ptr.To(binpackStrategy),
			CPU: ptr.To(binpackStrategy),
		},
	}

	defaults := buildDefaultPlugins(spec)
	originalCount := countEnabled(defaults)

	mergePluginOverrides(defaults, nil)

	assert.Equal(t, originalCount, countEnabled(defaults), "no overrides should not change count")
}

func TestMergePluginOverrides_DisablePlugin(t *testing.T) {
	spec := &kaiv1.SchedulingShardSpec{
		PlacementStrategy: &kaiv1.PlacementStrategy{
			GPU: ptr.To(binpackStrategy),
			CPU: ptr.To(binpackStrategy),
		},
	}

	defaults := buildDefaultPlugins(spec)
	mergePluginOverrides(defaults, map[string]kaiv1.PluginConfig{
		"elastic": {Enabled: ptr.To(false)},
	})

	assert.False(t, defaults["elastic"].enabled)

	// Verify it doesn't appear in resolved list
	resolved := resolvePlugins(defaults)
	for _, p := range resolved {
		assert.NotEqual(t, "elastic", p.Name, "disabled plugin should not appear in resolved list")
	}
}

func TestMergePluginOverrides_ChangePriority(t *testing.T) {
	spec := &kaiv1.SchedulingShardSpec{
		PlacementStrategy: &kaiv1.PlacementStrategy{
			GPU: ptr.To(binpackStrategy),
			CPU: ptr.To(binpackStrategy),
		},
	}

	defaults := buildDefaultPlugins(spec)
	mergePluginOverrides(defaults, map[string]kaiv1.PluginConfig{
		"predicates": {Priority: ptr.To(50)},
	})

	assert.Equal(t, 50, defaults["predicates"].priority)

	// predicates should now be near the end
	resolved := resolvePlugins(defaults)
	lastIdx := len(resolved) - 1
	// predicates should come after gpusharingorder (priority 100)
	// but the exact position depends on other plugin priorities
	for i, p := range resolved {
		if p.Name == "predicates" {
			assert.Greater(t, i, lastIdx/2, "predicates with low priority should be in bottom half")
			break
		}
	}
}

func TestMergePluginOverrides_OverrideArguments(t *testing.T) {
	spec := &kaiv1.SchedulingShardSpec{
		KValue: ptr.To(1.5),
		PlacementStrategy: &kaiv1.PlacementStrategy{
			GPU: ptr.To(binpackStrategy),
			CPU: ptr.To(binpackStrategy),
		},
	}

	defaults := buildDefaultPlugins(spec)
	// Verify default arguments first
	assert.Equal(t, "1.5", defaults["proportion"].option.Arguments["kValue"])

	// Override arguments (full replacement)
	mergePluginOverrides(defaults, map[string]kaiv1.PluginConfig{
		"proportion": {Arguments: map[string]string{"kValue": "3.0"}},
	})

	assert.Equal(t, map[string]string{"kValue": "3.0"}, defaults["proportion"].option.Arguments)
}

func TestMergePluginOverrides_AddCustomPlugin(t *testing.T) {
	spec := &kaiv1.SchedulingShardSpec{
		PlacementStrategy: &kaiv1.PlacementStrategy{
			GPU: ptr.To(binpackStrategy),
			CPU: ptr.To(binpackStrategy),
		},
	}

	defaults := buildDefaultPlugins(spec)
	mergePluginOverrides(defaults, map[string]kaiv1.PluginConfig{
		"myplugin": {Priority: ptr.To(1050), Arguments: map[string]string{"key": "val"}},
	})

	p, found := defaults["myplugin"]
	require.True(t, found)
	assert.True(t, p.enabled)
	assert.Equal(t, 1050, p.priority)
	assert.Equal(t, map[string]string{"key": "val"}, p.option.Arguments)

	// Verify it appears in resolved list at the right position
	resolved := resolvePlugins(defaults)
	var mypluginIdx, subgrouporderIdx int
	for i, p := range resolved {
		if p.Name == "myplugin" {
			mypluginIdx = i
		}
		if p.Name == "subgrouporder" {
			subgrouporderIdx = i
		}
	}
	// myplugin (1050) should come right after subgrouporder (1000) -> before subgrouporder in the list
	assert.Less(t, mypluginIdx, subgrouporderIdx, "myplugin should come before subgrouporder")
}

func TestMergeActionOverrides_DisableAction(t *testing.T) {
	spec := &kaiv1.SchedulingShardSpec{
		PlacementStrategy: &kaiv1.PlacementStrategy{
			GPU: ptr.To(binpackStrategy),
			CPU: ptr.To(binpackStrategy),
		},
	}

	defaults := buildDefaultActions(spec)
	mergeActionOverrides(defaults, map[string]kaiv1.ActionConfig{
		"preempt": {Enabled: ptr.To(false)},
	})

	assert.False(t, defaults["preempt"].enabled)

	actionsStr, actionNames := resolveActions(defaults)
	assert.NotContains(t, actionNames, "preempt")
	assert.NotContains(t, actionsStr, "preempt")
}

func TestMergeActionOverrides_ChangePriority(t *testing.T) {
	spec := &kaiv1.SchedulingShardSpec{
		PlacementStrategy: &kaiv1.PlacementStrategy{
			GPU: ptr.To(binpackStrategy),
			CPU: ptr.To(binpackStrategy),
		},
	}

	defaults := buildDefaultActions(spec)
	// reclaim (300) -> 600, which moves it above allocate (500)
	mergeActionOverrides(defaults, map[string]kaiv1.ActionConfig{
		"reclaim": {Priority: ptr.To(600)},
	})

	_, actionNames := resolveActions(defaults)
	var reclaimIdx, allocateIdx int
	for i, name := range actionNames {
		if name == "reclaim" {
			reclaimIdx = i
		}
		if name == "allocate" {
			allocateIdx = i
		}
	}
	assert.Less(t, reclaimIdx, allocateIdx, "reclaim with higher priority should come before allocate")
}

func TestMergeActionOverrides_AddCustomAction(t *testing.T) {
	spec := &kaiv1.SchedulingShardSpec{
		PlacementStrategy: &kaiv1.PlacementStrategy{
			GPU: ptr.To(binpackStrategy),
			CPU: ptr.To(binpackStrategy),
		},
	}

	defaults := buildDefaultActions(spec)
	mergeActionOverrides(defaults, map[string]kaiv1.ActionConfig{
		"myaction": {Priority: ptr.To(250)},
	})

	a, found := defaults["myaction"]
	require.True(t, found)
	assert.True(t, a.enabled)
	assert.Equal(t, 250, a.priority)

	_, actionNames := resolveActions(defaults)
	assert.Contains(t, actionNames, "myaction")

	// myaction (250) should be between reclaim (300) and preempt (200)
	var myactionIdx, reclaimIdx, preemptIdx int
	for i, name := range actionNames {
		switch name {
		case "myaction":
			myactionIdx = i
		case "reclaim":
			reclaimIdx = i
		case "preempt":
			preemptIdx = i
		}
	}
	assert.Less(t, reclaimIdx, myactionIdx, "reclaim should come before myaction")
	assert.Less(t, myactionIdx, preemptIdx, "myaction should come before preempt")
}

func TestMergeActionOverrides_EnableConditionallyAbsentAction(t *testing.T) {
	// Spread strategy means consolidation is absent by default
	spec := &kaiv1.SchedulingShardSpec{
		PlacementStrategy: &kaiv1.PlacementStrategy{
			GPU: ptr.To(spreadStrategy),
			CPU: ptr.To(binpackStrategy),
		},
	}

	defaults := buildDefaultActions(spec)
	_, found := defaults["consolidation"]
	require.False(t, found, "consolidation should be absent by default for spread strategy")

	// User explicitly adds consolidation
	mergeActionOverrides(defaults, map[string]kaiv1.ActionConfig{
		"consolidation": {Enabled: ptr.To(true), Priority: ptr.To(400)},
	})

	a, found := defaults["consolidation"]
	require.True(t, found)
	assert.True(t, a.enabled)

	_, actionNames := resolveActions(defaults)
	assert.Contains(t, actionNames, "consolidation")
}

func TestResolvePlugins_Ordering(t *testing.T) {
	plugins := map[string]*pluginWithPriority{
		"a": {option: conf.PluginOption{Name: "a"}, priority: 100, enabled: true},
		"b": {option: conf.PluginOption{Name: "b"}, priority: 200, enabled: true},
		"c": {option: conf.PluginOption{Name: "c"}, priority: 100, enabled: true},
		"d": {option: conf.PluginOption{Name: "d"}, priority: 300, enabled: false},
	}

	resolved := resolvePlugins(plugins)

	require.Len(t, resolved, 3, "disabled plugin should be filtered")
	assert.Equal(t, "b", resolved[0].Name, "highest priority first")
	assert.Equal(t, "a", resolved[1].Name, "alphabetical tiebreak")
	assert.Equal(t, "c", resolved[2].Name, "alphabetical tiebreak")
}

func TestResolveActions_Ordering(t *testing.T) {
	actions := map[string]*actionWithPriority{
		"x": {name: "x", priority: 50, enabled: true},
		"y": {name: "y", priority: 100, enabled: true},
		"z": {name: "z", priority: 50, enabled: true},
		"w": {name: "w", priority: 200, enabled: false},
	}

	actionsStr, actionNames := resolveActions(actions)

	require.Len(t, actionNames, 3, "disabled action should be filtered")
	assert.Equal(t, []string{"y", "x", "z"}, actionNames)
	assert.Equal(t, "y, x, z", actionsStr)
}

func countEnabled(plugins map[string]*pluginWithPriority) int {
	count := 0
	for _, p := range plugins {
		if p.enabled {
			count++
		}
	}
	return count
}
