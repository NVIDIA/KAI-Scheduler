#!/bin/bash
# Copyright 2025 NVIDIA CORPORATION
# SPDX-License-Identifier: Apache-2.0
set -e

kubectl apply --server-side -k "github.com/kubernetes-sigs/jobset.git/config/default?ref=v0.2.0"

