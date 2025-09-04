#!/bin/bash
# Copyright 2025 NVIDIA CORPORATION
# SPDX-License-Identifier: Apache-2.0


CLUSTER_NAME=${CLUSTER_NAME:-e2e-kai-scheduler}

REPO_ROOT=$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )/..
KIND_CONFIG=${REPO_ROOT}/hack/e2e-kind-config.yaml
GOPATH=${HOME}/go
GOBIN=${GOPATH}/bin

# Parse named parameters
TEST_THIRD_PARTY_INTEGRATIONS="false"
LOCAL_IMAGES_BUILD="false"
PRESERVE_CLUSTER="false"

while [[ $# -gt 0 ]]; do
  case $1 in
    --test-third-party-integrations)
      TEST_THIRD_PARTY_INTEGRATIONS="true"
      shift
      ;;
    --local-images-build)
      LOCAL_IMAGES_BUILD="true"
      shift
      ;;
    --preserve-cluster)
      PRESERVE_CLUSTER="true"
      shift
      ;;
    -h|--help)
      echo "Usage: $0 [--test-third-party-integrations] [--local-images-build]"
      echo "  --test-third-party-integrations: Install third party operators for compatibility testing"
      echo "  --local-images-build: Build and use local images instead of pulling from registry"
      echo "  --preserve-cluster: Keep the kind cluster after running the test suite"
      exit 0
      ;;
    *)
      echo "Unknown option $1"
      echo "Use --help for usage information"
      exit 1
      ;;
  esac
done

if [ "$LOCAL_IMAGES_BUILD" = "true" ]; then
      echo "OMRIC: Building local images"
      cd ${REPO_ROOT}
      PACKAGE_VERSION=0.0.0-$(git rev-parse --short HEAD)
      make build VERSION=$PACKAGE_VERSION
      
      helm package ./deployments/kai-scheduler -d ./charts --app-version $PACKAGE_VERSION --version $PACKAGE_VERSION
      
      cd ${REPO_ROOT}/hack
  else
      PACKAGE_VERSION=0.0.0-$(git rev-parse --short origin/main)
  fi

for i in {1..100}; do
  echo "Running attempt $i/100"
  kind create cluster --config ${KIND_CONFIG} --name $CLUSTER_NAME

  # Install the fake-gpu-operator to provide a fake GPU resources for the e2e tests
  helm upgrade -i gpu-operator oci://ghcr.io/run-ai/fake-gpu-operator/fake-gpu-operator --namespace gpu-operator --create-namespace --version 0.0.62 \
      --values ${REPO_ROOT}/hack/fake-gpu-operator-values.yaml --wait

  # install third party operators to check the compatibility with the kai-scheduler
  if [ "$TEST_THIRD_PARTY_INTEGRATIONS" = "true" ]; then
      ${REPO_ROOT}/hack/third_party_integrations/deploy_ray.sh
      ${REPO_ROOT}/hack/third_party_integrations/deploy_kubeflow.sh
      ${REPO_ROOT}/hack/third_party_integrations/deploy_knative.sh
      ${REPO_ROOT}/hack/third_party_integrations/deploy_lws.sh
  fi

  if [ "$LOCAL_IMAGES_BUILD" = "true" ]; then
      echo "OMRIC: Loading local images"
      cd ${REPO_ROOT}
      PACKAGE_VERSION=0.0.0-$(git rev-parse --short HEAD)
      for image in $(docker images --format '{{.Repository}}:{{.Tag}}' | grep $PACKAGE_VERSION); do
          kind load docker-image $image --name $CLUSTER_NAME
      done
      ls
      helm upgrade -i kai-scheduler ./charts/kai-scheduler-$PACKAGE_VERSION.tgz  -n kai-scheduler --create-namespace --set "global.gpuSharing=true" --wait
      cd ${REPO_ROOT}/hack
  else
      PACKAGE_VERSION=0.0.0-$(git rev-parse --short origin/main)
      helm upgrade -i kai-scheduler oci://ghcr.io/nvidia/kai-scheduler/kai-scheduler -n kai-scheduler --create-namespace --set "global.gpuSharing=true" --version "$PACKAGE_VERSION"
  fi

  # Allow all the pods in the fake-gpu-operator and kai-scheduler to start
  sleep 30

  # Install ginkgo if it's not installed
  if [ ! -f ${GOBIN}/ginkgo ]; then
      echo "Installing ginkgo"
      GOBIN=${GOBIN} go install github.com/onsi/ginkgo/v2/ginkgo@v2.23.3
  fi

  if ! ${GOBIN}/ginkgo -r -focus "PodGroup Conditions Jobs Over Queue Limit NonPreemptible Job" --trace -vv ${REPO_ROOT}/test/e2e/suites/api/crds/podgroup; then
      PRESERVE_CLUSTER="true"
      kubectl logs -nkai-scheduler deploy/kai-scheduler > ~/flaky-podgroup-condition/kai-scheduler.log
      kubectl logs -nkai-scheduler deploy/pod-grouper > ~/flaky-podgroup-condition/grouper.log
      kubectl get podgroups.v2alpha2.scheduling.run.ai -A -oyaml > ~/flaky-podgroup-condition/podgroups.yaml
      break
  fi

  if [ "$PRESERVE_CLUSTER" != "true" ]; then
      kind delete cluster --name $CLUSTER_NAME
  fi
done