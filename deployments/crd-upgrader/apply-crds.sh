#!/bin/bash
# Copyright 2025 NVIDIA CORPORATION
# SPDX-License-Identifier: Apache-2.0

migrate_queues_from_v2alpha2() {
  if kubectl get crd queues.scheduling.run.ai &>/dev/null; then
    if kubectl get crd queues.scheduling.run.ai -o jsonpath='{.status.storedVersions}' | grep -q 'v2alpha2'; then
      echo "Migrating queues from v2alpha2"
      kubectl get queues.scheduling.run.ai --all-namespaces -o json | kubectl replace -f -
      kubectl patch crd queues.scheduling.run.ai -p '{"status":{"storedVersions":[]}}' --subresource=status
      echo "Queues migrated from v2alpha2"
    fi
  fi
}

migrate_queues_from_v2alpha2
# Using --force-conflicts to claim ownership of the CRDs from helm
kubectl apply --server-side=true --force-conflicts -f /internal-crds
