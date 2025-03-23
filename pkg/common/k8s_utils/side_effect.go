// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package k8s_utils

import (
	_ "k8s.io/cli-runtime"
	_ "k8s.io/cluster-bootstrap"
	_ "k8s.io/cri-client/pkg"
	_ "k8s.io/endpointslice"
	_ "k8s.io/externaljwt"
	_ "k8s.io/kube-aggregator/pkg/apis/apiregistration/v1"
	_ "k8s.io/kube-controller-manager"
	_ "k8s.io/kube-proxy"
	_ "k8s.io/kubectl"
	_ "k8s.io/metrics"
	_ "k8s.io/mount-utils"
	_ "k8s.io/pod-security-admission"
	_ "k8s.io/sample-apiserver/pkg/apis/wardle"
)
