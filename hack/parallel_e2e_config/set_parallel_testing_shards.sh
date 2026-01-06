#!/bin/bash
# Copyright 2025 NVIDIA CORPORATION
# SPDX-License-Identifier: Apache-2.0

REPO_ROOT=$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )/../..

# Get all nodes containing "worker" in their name
ALL_WORKER_NODES=$(kubectl get nodes -o jsonpath='{.items[*].metadata.name}' | tr ' ' '\n' | grep 'worker')
if [ -z "$ALL_WORKER_NODES" ]; then
  echo "Error: No 'worker' nodes found"
  exit 1
fi

# Count total worker nodes
TOTAL_WORKERS=$(echo "$ALL_WORKER_NODES" | wc -l | tr -d ' ')
echo "Found $TOTAL_WORKERS worker node(s)"

# Calculate half (rounded up)
HALF_WORKERS=$(( ($TOTAL_WORKERS + 1) / 2 ))
echo "Selecting $HALF_WORKERS node(s) to label (half of $TOTAL_WORKERS)"

# Select half of the worker nodes
NODES_TO_LABEL=$(echo "$ALL_WORKER_NODES" | head -n $HALF_WORKERS | tr '\n' ' ')

if [ -z "$NODES_TO_LABEL" ]; then
  echo "Error: Failed to select nodes to label"
  exit 1
fi

echo "Labeling nodes: $NODES_TO_LABEL for scheduling shard test-pool-2"
kubectl label nodes $NODES_TO_LABEL kai.scheduler/node-pool=test-pool-2 --overwrite

echo "Create the scheduling shard test-shard-2.yaml"
kubectl apply -f ${REPO_ROOT}/hack/parallel_e2e_config/test-shard-2.yaml