#!/bin/bash
# Copyright 2025 NVIDIA CORPORATION
# SPDX-License-Identifier: Apache-2.0


# Using --force-conflicts to claim ownership of the CRDs from helm
kubectl apply --server-side=true --force-conflicts -f /internal-crds

# Do not apply external crd if an equivelent crd is already installed
kubectl apply --server-side=true -f /external-crds