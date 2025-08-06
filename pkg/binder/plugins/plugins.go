// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package plugins

import (
	"context"
	"fmt"

	v1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/NVIDIA/KAI-scheduler/pkg/apis/scheduling/v1alpha2"

	"github.com/NVIDIA/KAI-scheduler/pkg/binder/plugins/state"
)

type BinderPlugins struct {
	plugins []Plugin
}

func New() *BinderPlugins {
	return &BinderPlugins{
		plugins: []Plugin{},
	}
}

func (bp *BinderPlugins) RegisterPlugin(plugin Plugin) {
	bp.plugins = append(bp.plugins, plugin)
}

func (bp *BinderPlugins) PreBind(ctx context.Context, pod *v1.Pod, host *v1.Node, bindRequest *v1alpha2.BindRequest,
	state *state.BindingState) error {
	for _, p := range bp.plugins {
		err := p.PreBind(ctx, pod, host, bindRequest, state)
		if err != nil {
			logger := log.FromContext(context.Background())
			logger.Error(err, "PreBind plugin failed for pod",
				"plugin", p.Name(), "namespace", pod.Namespace, "name", pod.Name)
			return fmt.Errorf("plugin %s failed in PreBind: %v", p.Name(), err)
		}
	}
	return nil
}

func (bp *BinderPlugins) PostBind(ctx context.Context, pod *v1.Pod, host *v1.Node, bindRequest *v1alpha2.BindRequest,
	state *state.BindingState) {
	for _, p := range bp.plugins {
		p.PostBind(ctx, pod, host, bindRequest, state)
	}
}
