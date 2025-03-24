/*
Copyright 2025 NVIDIA CORPORATION
SPDX-License-Identifier: Apache-2.0
*/
package rd

import (
	v1 "k8s.io/api/core/v1"
	"k8s.io/utils/ptr"
)

// DefaultSecurityContext Set container security context that is valid by default openshift clusters
func DefaultSecurityContext() *v1.SecurityContext {
	return &v1.SecurityContext{
		AllowPrivilegeEscalation: ptr.To(false),
		RunAsNonRoot:             ptr.To(true),
		RunAsUser:                ptr.To(int64(1000)),
		Capabilities: &v1.Capabilities{
			Add:  []v1.Capability{},
			Drop: []v1.Capability{},
		},
		SeccompProfile: &v1.SeccompProfile{
			Type: v1.SeccompProfileTypeRuntimeDefault,
		},
	}
}
