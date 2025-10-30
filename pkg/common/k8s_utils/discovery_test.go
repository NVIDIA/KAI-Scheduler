// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package k8s_utils

import (
	"context"
	"testing"

	nodev1 "k8s.io/api/node/v1"
	"k8s.io/apimachinery/pkg/api/meta"

	ocpconf "github.com/openshift/api/config/v1"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
)

func TestIsOpenshift_ClusterVersionRegisteredAndExists(t *testing.T) {
	scheme := runtime.NewScheme()
	utilruntime.Must(ocpconf.AddToScheme(scheme))

	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(&ocpconf.ClusterVersion{
		ObjectMeta: metav1.ObjectMeta{
			Name: "cluster",
		},
	}).Build()

	isOpenshift, err := IsOpenshift(context.Background(), c)
	if err != nil {
		t.Fatalf("Failed to check if cluster is openshift: %v", err)
	}

	assert.NoError(t, err)
	assert.True(t, isOpenshift)
}

func TestIsOpenshift_ClusterVersionNotRegistered(t *testing.T) {
	scheme := runtime.NewScheme()
	utilruntime.Must(ocpconf.AddToScheme(scheme))

	c := fake.NewClientBuilder().WithScheme(scheme).WithInterceptorFuncs(interceptor.Funcs{
		List: func(ctx context.Context, client client.WithWatch, list client.ObjectList, opts ...client.ListOption) error {
			return &meta.NoResourceMatchError{}
		},
	}).Build()

	isOpenshift, err := IsOpenshift(context.Background(), c)
	if err != nil {
		t.Fatalf("Failed to check if cluster is openshift: %v", err)
	}

	assert.NoError(t, err)
	assert.False(t, isOpenshift)
}

func TestRuntimeClassExists_True(t *testing.T) {
	scheme := runtime.NewScheme()
	utilruntime.Must(nodev1.AddToScheme(scheme))

	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(&nodev1.RuntimeClass{
		ObjectMeta: metav1.ObjectMeta{
			Name: "runtime-class",
		},
	}).Build()

	exists, err := RuntimeClassExists(context.Background(), c, "runtime-class")
	if err != nil {
		t.Fatalf("Failed to check if runtime class exists: %v", err)
	}

	assert.NoError(t, err)
	assert.True(t, exists)
}

func TestRuntimeClassExists_False(t *testing.T) {
	scheme := runtime.NewScheme()
	utilruntime.Must(nodev1.AddToScheme(scheme))

	c := fake.NewClientBuilder().WithScheme(scheme).Build()

	exists, err := RuntimeClassExists(context.Background(), c, "runtime-class")
	if err != nil {
		t.Fatalf("Failed to check if runtime class exists: %v", err)
	}

	assert.NoError(t, err)
	assert.False(t, exists)
}
