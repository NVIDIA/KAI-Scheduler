#!/bin/bash
# Copyright 2025 NVIDIA CORPORATION
# SPDX-License-Identifier: Apache-2.0


CLUSTER_NAME=${CLUSTER_NAME:-e2e-kai-scheduler}

REPO_ROOT=$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )/..
KIND_CONFIG=${REPO_ROOT}/hack/e2e-kind-config.yaml
GOPATH=${HOME}/go
GOBIN=${GOPATH}/bin

TEST_THIRD_PARTY_INTEGRATIONS=${1}
if [ -z "$TEST_THIRD_PARTY_INTEGRATIONS" ]; then
    echo "TEST_THIRD_PARTY_INTEGRATIONS argument isn't provided, defaulting to false"
    TEST_THIRD_PARTY_INTEGRATIONS="false"
fi

kind create cluster --config ${KIND_CONFIG} --name $CLUSTER_NAME

# Add necessary helm repos
helm repo add fake-gpu https://runai.jfrog.io/artifactory/api/helm/fake-gpu-operator-charts-prod
helm repo add nvidia https://helm.ngc.nvidia.com/nvidia/k8s
helm repo update

# Install the fake-gpu-operator to provide a fake GPU resources for the e2e tests
helm upgrade -i gpu-operator fake-gpu/fake-gpu-operator --namespace gpu-operator --create-namespace --version 0.0.53 --set topology.nodePools.default.gpuCount=8
sleep 10 # Wait for the fake-gpu-operator to start

# install third party operators to check the compatibility with the kai-scheduler
if [ "$TEST_THIRD_PARTY_INTEGRATIONS" = "true" ]; then
    ${REPO_ROOT}/hack/third_party_integrations/deploy_ray.sh
    ${REPO_ROOT}/hack/third_party_integrations/deploy_kubeflow.sh
fi

helm upgrade -i kai-scheduler nvidia/kai-scheduler -n kai-scheduler --create-namespace --set "global.registry=nvcr.io/nvidia/k8s" --set "global.gpuSharing=true"

# Allow all the pods in the fake-gpu-operator and kai-scheduler to start
sleep 30

# Install ginkgo if it's not installed
if [ ! -f ${GOBIN}/ginkgo ]; then
    echo "Installing ginkgo"
    GOBIN=${GOBIN} go install github.com/onsi/ginkgo/v2/ginkgo@v2.23.3
fi

${GOBIN}/ginkgo -r --keep-going --randomize-all --randomize-suites --trace -vv ${REPO_ROOT}/test/e2e/suites --label-filter '!autoscale', '!scale'

kind delete cluster --name $CLUSTER_NAME
