#!/bin/bash
# Copyright 2025 NVIDIA CORPORATION
# SPDX-License-Identifier: Apache-2.0

REPO_ROOT=$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )/../..

kubectl apply -f ${REPO_ROOT}/hack/parallel_e2e_config/test-shard-2.yaml
kubectl label nodes e2e-kai-scheduler-worker3 e2e-kai-scheduler-worker4 kai.scheduler/node-pool=test-pool-2 --overwrite