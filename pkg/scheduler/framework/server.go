// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package framework

import (
	"fmt"
	"net/http"
)

type PluginServer struct {
	registeredPlugins map[string]func(http.ResponseWriter, *http.Request)
	mux               *http.ServeMux
}

func newPluginServer(mux *http.ServeMux) *PluginServer {
	return &PluginServer{
		registeredPlugins: map[string]func(http.ResponseWriter, *http.Request){},
		mux:               mux,
	}
}

func (ps *PluginServer) registerPlugin(path string, handler func(http.ResponseWriter, *http.Request)) error {
	if ps.mux == nil {
		return fmt.Errorf("server not initialized")
	}

	if _, ok := ps.registeredPlugins[path]; ok {
		return fmt.Errorf("plugin %s already registered", path)
	}

	ps.registeredPlugins[path] = handler
	ps.mux.HandleFunc(path, handler)
	return nil
}
