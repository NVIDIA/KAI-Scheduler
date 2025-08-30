#!/bin/bash
# Copyright 2025 NVIDIA CORPORATION
# SPDX-License-Identifier: Apache-2.0


# Using --force-conflicts to claim ownership of the CRDs from helm
kubectl apply --server-side=true --force-conflicts -f /internal-crds

# Check and apply external CRDs only if they don't already exist in the cluster
for crd_file in /external-crds/*.yaml; do
    if [ -f "$crd_file" ]; then
        # Extract CRD name from the file
        crd_name=$(grep "^  name:" "$crd_file" | head -1 | awk '{print $2}')
        if [ -n "$crd_name" ]; then
            # Check if CRD already exists in the cluster
            if kubectl get crd "$crd_name" >/dev/null 2>&1; then
                echo "CRD $crd_name already exists in cluster, skipping..."
            else
                echo "Applying CRD $crd_name..."
                kubectl apply --server-side=true -f "$crd_file"
            fi
        else
            echo "Warning: Could not extract CRD name from $crd_file"
        fi
    fi
done