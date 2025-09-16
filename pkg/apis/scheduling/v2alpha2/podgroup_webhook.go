// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package v2alpha2

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// log is for logging in this package.
var logger = logf.Log.WithName("podgroup-validation")

func (p *PodGroup) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(p).
		WithValidator(&PodGroup{}).
		Complete()
}

func (_ *PodGroup) ValidateCreate(_ context.Context, obj runtime.Object) (admission.Warnings, error) {
	podGroup, ok := obj.(*PodGroup)
	if !ok {
		return nil, fmt.Errorf("expected a PodGroup but got a %T", obj)
	}
	logger.Info("validate create", "namespace", podGroup.Namespace, "name", podGroup.Name)

	return nil, nil
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (_ *PodGroup) ValidateUpdate(_ context.Context, _ runtime.Object, newObj runtime.Object) (admission.Warnings, error) {
	podGroup, ok := newObj.(*PodGroup)
	if !ok {
		return nil, fmt.Errorf("expected a PodGroup but got a %T", newObj)
	}
	logger.Info("validate update", "namespace", podGroup.Namespace, "name", podGroup.Name)

	return nil, nil
}

func (_ *PodGroup) ValidateDelete(_ context.Context, obj runtime.Object) (admission.Warnings, error) {
	podGroup, ok := obj.(*PodGroup)
	if !ok {
		return nil, fmt.Errorf("expected a PodGroup but got a %T", obj)
	}
	logger.Info("validate delete", "namespace", podGroup.Namespace, "name", podGroup.Name)
	return nil, nil
}
