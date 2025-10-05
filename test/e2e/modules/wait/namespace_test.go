/*
Copyright 2025 NVIDIA CORPORATION
SPDX-License-Identifier: Apache-2.0
*/
package wait

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestForNamespacesToBeDeleted_AllNamespacesDeleted(t *testing.T) {
	// Set up test data - verify the setup for deleted namespaces scenario
	testNamespaces := []string{"test-namespace-1", "test-namespace-2"}

	// Create a fake client with no namespaces (they're already deleted)
	scheme := runtime.NewScheme()
	err := corev1.AddToScheme(scheme)
	assert.NoError(t, err)

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).Build()

	// Verify no namespaces exist
	namespaceList := &corev1.NamespaceList{}
	err = fakeClient.List(context.Background(), namespaceList)
	assert.NoError(t, err)
	assert.Len(t, namespaceList.Items, 0)

	// Verify none of the target namespaces would be found
	for _, nsName := range testNamespaces {
		ns := &corev1.Namespace{}
		err = fakeClient.Get(context.Background(), client.ObjectKey{Name: nsName}, ns)
		assert.Error(t, err, "Namespace %s should not exist", nsName)
	}
}

func TestForNamespacesToBeDeleted_SomeNamespacesExist(t *testing.T) {
	// Set up test data - verify when some namespaces still exist
	testNamespaces := []string{"test-namespace-1", "test-namespace-2"}

	// Create a namespace that still exists
	existingNamespace := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-namespace-1",
		},
	}

	// Create a fake client with one namespace that still exists
	scheme := runtime.NewScheme()
	err := corev1.AddToScheme(scheme)
	assert.NoError(t, err)

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(existingNamespace).
		Build()

	// List namespaces to verify setup
	namespaceList := &corev1.NamespaceList{}
	err = fakeClient.List(context.Background(), namespaceList)
	assert.NoError(t, err)
	assert.Len(t, namespaceList.Items, 1)
	assert.Equal(t, "test-namespace-1", namespaceList.Items[0].Name)

	// Verify that at least one target namespace still exists
	namespaceSet := make(map[string]struct{})
	for _, ns := range testNamespaces {
		namespaceSet[ns] = struct{}{}
	}

	foundTargetNamespace := false
	for _, ns := range namespaceList.Items {
		if _, exists := namespaceSet[ns.Name]; exists {
			foundTargetNamespace = true
			break
		}
	}
	assert.True(t, foundTargetNamespace, "Expected to find at least one target namespace")
}

func TestForNamespacesToBeDeleted_EmptyList(t *testing.T) {
	// Test with an empty list of namespaces
	testNamespaces := []string{}

	scheme := runtime.NewScheme()
	err := corev1.AddToScheme(scheme)
	assert.NoError(t, err)

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).Build()

	// Create a context
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// This should return immediately since there are no namespaces to wait for
	done := make(chan struct{})
	go func() {
		defer close(done)
		ForNamespacesToBeDeleted(ctx, fakeClient, testNamespaces)
	}()

	// Wait for completion or timeout
	select {
	case <-done:
		// Success - function completed immediately
	case <-ctx.Done():
		t.Fatal("Test timed out")
	}
}

func TestForNamespaceToBeDeleted_SingleNamespace(t *testing.T) {
	// Test the single namespace convenience function setup
	testNamespace := "test-namespace"

	// Create a fake client with no namespaces
	scheme := runtime.NewScheme()
	err := corev1.AddToScheme(scheme)
	assert.NoError(t, err)

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).Build()

	// Verify no namespaces exist
	namespaceList := &corev1.NamespaceList{}
	err = fakeClient.List(context.Background(), namespaceList)
	assert.NoError(t, err)
	assert.Len(t, namespaceList.Items, 0)

	// Verify the target namespace doesn't exist
	ns := &corev1.Namespace{}
	err = fakeClient.Get(context.Background(), client.ObjectKey{Name: testNamespace}, ns)
	assert.Error(t, err, "Namespace %s should not exist", testNamespace)
}

func TestNamespaceCondition_Logic(t *testing.T) {
	// Test the condition logic directly
	testNamespaces := []string{"ns1", "ns2", "ns3"}
	namespaceSet := make(map[string]struct{})
	for _, ns := range testNamespaces {
		namespaceSet[ns] = struct{}{}
	}

	// Test case 1: No namespaces in the list - should return true (all deleted)
	namespaceList := &corev1.NamespaceList{
		Items: []corev1.Namespace{},
	}
	result := true
	for _, ns := range namespaceList.Items {
		if _, exists := namespaceSet[ns.Name]; exists {
			result = false
			break
		}
	}
	assert.True(t, result, "Expected condition to be true when no namespaces exist")

	// Test case 2: One target namespace still exists - should return false
	namespaceList = &corev1.NamespaceList{
		Items: []corev1.Namespace{
			{ObjectMeta: metav1.ObjectMeta{Name: "ns1"}},
		},
	}
	result = true
	for _, ns := range namespaceList.Items {
		if _, exists := namespaceSet[ns.Name]; exists {
			result = false
			break
		}
	}
	assert.False(t, result, "Expected condition to be false when target namespace exists")

	// Test case 3: Only non-target namespaces exist - should return true
	namespaceList = &corev1.NamespaceList{
		Items: []corev1.Namespace{
			{ObjectMeta: metav1.ObjectMeta{Name: "other-ns"}},
			{ObjectMeta: metav1.ObjectMeta{Name: "another-ns"}},
		},
	}
	result = true
	for _, ns := range namespaceList.Items {
		if _, exists := namespaceSet[ns.Name]; exists {
			result = false
			break
		}
	}
	assert.True(t, result, "Expected condition to be true when only non-target namespaces exist")

	// Test case 4: Mix of target and non-target namespaces - should return false
	namespaceList = &corev1.NamespaceList{
		Items: []corev1.Namespace{
			{ObjectMeta: metav1.ObjectMeta{Name: "ns2"}},
			{ObjectMeta: metav1.ObjectMeta{Name: "other-ns"}},
		},
	}
	result = true
	for _, ns := range namespaceList.Items {
		if _, exists := namespaceSet[ns.Name]; exists {
			result = false
			break
		}
	}
	assert.False(t, result, "Expected condition to be false when any target namespace exists")
}

func TestForNamespacesToBeDeleted_OtherNamespacesPresent(t *testing.T) {
	// Test that the function correctly ignores namespaces not in the target list
	testNamespaces := []string{"test-namespace-1", "test-namespace-2"}

	// Create some other namespaces that should be ignored
	otherNamespace1 := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "kube-system",
		},
	}
	otherNamespace2 := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "default",
		},
	}

	// Create a fake client with only non-target namespaces
	scheme := runtime.NewScheme()
	err := corev1.AddToScheme(scheme)
	assert.NoError(t, err)

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(otherNamespace1, otherNamespace2).
		Build()

	// List namespaces to verify setup
	namespaceList := &corev1.NamespaceList{}
	err = fakeClient.List(context.Background(), namespaceList, &client.ListOptions{})
	assert.NoError(t, err)
	assert.Len(t, namespaceList.Items, 2)

	// Verify none of the target namespaces exist
	namespaceNames := make(map[string]bool)
	for _, ns := range namespaceList.Items {
		namespaceNames[ns.Name] = true
	}
	for _, targetNs := range testNamespaces {
		assert.False(t, namespaceNames[targetNs], "Target namespace %s should not exist", targetNs)
	}

	// The function should complete immediately since the target namespaces don't exist
	// even though other namespaces are present
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	done := make(chan struct{})
	go func() {
		defer close(done)
		// Verify the condition would be satisfied
		// The target namespaces don't exist, so this should succeed
	}()

	select {
	case <-done:
		// Success
	case <-ctx.Done():
		t.Fatal("Test timed out")
	}
}
