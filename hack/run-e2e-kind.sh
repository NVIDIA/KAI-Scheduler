#!/bin/bash
# Copyright 2025 NVIDIA CORPORATION
# SPDX-License-Identifier: Apache-2.0


CLUSTER_NAME=${CLUSTER_NAME:-e2e-kai-scheduler}

REPO_ROOT=$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )/..
KIND_CONFIG=${REPO_ROOT}/hack/e2e-kind-config.yaml
GOPATH=${HOME}/go
GOBIN=${GOPATH}/bin

NVCR_SECRET_FILE_PATH=${1}
if [ -z "$NVCR_SECRET_FILE_PATH" ]; then
    echo "Must a path to an appropriate secret file, that contains the credentials for the nvstaging-runai helm and docker image repository"
    exit 1
fi

kind create cluster --config ${KIND_CONFIG} --name $CLUSTER_NAME

kubectl create namespace kai-scheduler
# Set an appropriate secret to allow the kube-ai system pods to pull from nvstaging-runai and pull test images for the e2e tests
kubectl apply -f ${NVCR_SECRET_FILE_PATH} -n kai-scheduler


# Install the fake-gpu-operator to provide a fake GPU resources for the e2e tests
helm upgrade -i gpu-operator fake-gpu-operator/fake-gpu-operator --namespace gpu-operator --create-namespace --version 0.0.53 --set topology.nodePools.default.gpuCount=8

# install third party operators to check the compatibility with the kai-scheduler
${REPO_ROOT}/hack/third_party_integrations/deploy_ray.sh
${REPO_ROOT}/hack/third_party_integrations/deploy_kubeflow.sh

helm upgrade -i kai-scheduler nvstaging-runai/kai-scheduler -n kai-scheduler --create-namespace --set "global.imagePullSecrets[0].name=nvcr-secret" --set "global.gpuSharing=true" --set "global.registry=nvcr.io/nvstaging/runai" --version v0.2.0

# Allow all the pods in the fake-gpu-operator and kai-scheduler to start
sleep 30

# Install ginkgo if it's not installed
if [ ! -f ${GOBIN}/ginkgo ]; then
    echo "Installing ginkgo"
    GOBIN=${GOBIN} go install github.com/onsi/ginkgo/v2/ginkgo@v2.22.2
fi

${GOBIN}/ginkgo -r --keep-going --randomize-all --randomize-suites --trace -vv ${REPO_ROOT}/test/e2e/suites/integrations/third_party --label-filter '!autoscale', '!scale'

kind delete cluster --name $CLUSTER_NAME
