// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

// +kubebuilder:object:generate:=true
package prometheus

import (
	"k8s.io/utils/ptr"
)

type Prometheus struct {
	// Enabled defines whether a Prometheus instance should be deployed
	// +kubebuilder:validation:Optional
	Enabled *bool `json:"enabled,omitempty"`
}

func (p *Prometheus) SetDefaultsWhereNeeded() {

	if p.Enabled == nil {
		p.Enabled = ptr.To(false)
	}

}
