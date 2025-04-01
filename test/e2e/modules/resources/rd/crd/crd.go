// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package crd

import (
	"context"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	apiextensions "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func SkipIfCrdIsNotInstalled(ctx context.Context, kubeConfig *rest.Config, name, version string) {
	installed, err := isCrdInstalled(ctx, kubeConfig, name, version)
	Expect(err).NotTo(HaveOccurred())
	if !installed {
		Skip(fmt.Sprintf("CRD %s in version %s is not installed", name, version))
	}
}

func isCrdInstalled(ctx context.Context, kubeConfig *rest.Config, name, version string) (bool, error) {
	apiExtensionSdk := apiextensions.NewForConfigOrDie(kubeConfig)

	crd, err := apiExtensionSdk.ApiextensionsV1().CustomResourceDefinitions().Get(ctx, name, v1.GetOptions{})
	if err != nil {
		return false, client.IgnoreNotFound(err)
	}

	for _, crdVersion := range crd.Spec.Versions {
		if crdVersion.Name == version {
			return true, nil
		}
	}

	return false, nil
}
