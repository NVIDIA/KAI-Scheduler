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
	ps := &PluginServer{
		registeredPlugins: map[string]func(http.ResponseWriter, *http.Request){},
		mux:               mux,
	}
	if ps.mux != nil {
		ps.mux.HandleFunc("/", ps.handlePlugin)
	}

	return ps
}

func (ps *PluginServer) registerPlugin(path string, handler func(http.ResponseWriter, *http.Request)) error {
	if ps.mux == nil {
		return fmt.Errorf("server not initialized")
	}

	ps.registeredPlugins[path] = handler
	return nil
}

func (ps *PluginServer) handlePlugin(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path
	handler, ok := ps.registeredPlugins[path]
	if !ok {
		http.Error(w, "plugin not found", http.StatusNotFound)
		return
	}
	handler(w, r)
}
