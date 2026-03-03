#!/bin/bash
# Copyright 2025 NVIDIA CORPORATION
# SPDX-License-Identifier: Apache-2.0

# This script runs upgrade e2e tests for the kai-scheduler.
# It sets up a kind cluster, installs the previous minor release from OCI,
# then runs upgrade tests that helm-upgrade to the current version.

set -e

CLUSTER_NAME=${CLUSTER_NAME:-e2e-kai-scheduler}

REPO_ROOT=$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )/..
KIND_CONFIG=${REPO_ROOT}/hack/e2e-kind-config.yaml
GOPATH=${HOME}/go
GOBIN=${GOPATH}/bin

: ${KIND_K8S_TAG:="v1.34.0"}
: ${KIND_IMAGE:="kindest/node:${KIND_K8S_TAG}"}

LOCAL_IMAGES_BUILD="false"
PRESERVE_CLUSTER="false"

while [[ $# -gt 0 ]]; do
  case $1 in
    --local-images-build)
      LOCAL_IMAGES_BUILD="true"
      shift
      ;;
    --preserve-cluster)
      PRESERVE_CLUSTER="true"
      shift
      ;;
    -h|--help)
      echo "Usage: $0 [--local-images-build] [--preserve-cluster]"
      echo "  --local-images-build: Build and use local images for the upgrade target"
      echo "  --preserve-cluster: Keep the kind cluster after running the test suite"
      echo ""
      echo "Environment variables:"
      echo "  UPGRADE_FROM_VERSION: Override the version to upgrade from (e.g. v0.12.0)"
      echo "  PACKAGE_VERSION: Override the target version to upgrade to"
      exit 0
      ;;
    *)
      echo "Unknown option $1"
      echo "Use --help for usage information"
      exit 1
      ;;
  esac
done

# resolve_previous_minor_version resolves the latest release of the previous
# minor version. For example, if current is v0.13.x, it finds the latest v0.12.x.
# It uses the git branch name for version branches (v*.*) or the latest release
# for the main branch.
resolve_previous_minor_version() {
    local current_branch
    current_branch=$(git rev-parse --abbrev-ref HEAD 2>/dev/null || echo "main")

    local current_minor=""

    # Check if the branch name looks like a version branch (e.g. v0.13, release/v0.13)
    if [[ "$current_branch" =~ v([0-9]+)\.([0-9]+) ]]; then
        current_minor="${BASH_REMATCH[1]}.${BASH_REMATCH[2]}"
    else
        # On main branch: get the latest release to determine current minor
        local latest_release
        latest_release=$(curl -sf "https://api.github.com/repos/NVIDIA/KAI-Scheduler/releases?per_page=100" | jq -r '.[].tag_name' | grep -E '^v[0-9]+\.[0-9]+\.[0-9]+$' | sort -V | tail -1)
        if [[ -n "$latest_release" && "$latest_release" != "null" && "$latest_release" =~ v([0-9]+)\.([0-9]+) ]]; then
            current_minor="${BASH_REMATCH[1]}.${BASH_REMATCH[2]}"
        fi
    fi

    if [ -z "$current_minor" ]; then
        echo ""
        return
    fi

    local major minor
    major=$(echo "$current_minor" | cut -d. -f1)
    minor=$(echo "$current_minor" | cut -d. -f2)

    if [ "$minor" -eq 0 ]; then
        echo ""
        return
    fi

    local previous_minor="${major}.$((minor - 1))"

    # Find the latest release matching the previous minor version
    local previous_release
    previous_release=$(curl -sf "https://api.github.com/repos/NVIDIA/KAI-Scheduler/releases?per_page=100" | \
        jq -r '.[].tag_name' | \
        grep -E "^v${previous_minor}\.[0-9]+$" | \
        sort -V | tail -1)

    echo "$previous_release"
}

# Resolve the version to upgrade from
if [ -z "$UPGRADE_FROM_VERSION" ]; then
    echo "Resolving previous minor version to upgrade from..."
    UPGRADE_FROM_VERSION=$(resolve_previous_minor_version)
    if [ -z "$UPGRADE_FROM_VERSION" ]; then
        echo "Could not resolve a previous minor release. Skipping upgrade tests."
        exit 0
    fi
fi
echo "Upgrade from version: $UPGRADE_FROM_VERSION"

# Create kind cluster
kind create cluster \
    --config ${KIND_CONFIG} \
    --image ${KIND_IMAGE} \
    --name $CLUSTER_NAME

# Deploy local image registry
echo "Deploying local image registry..."
kubectl apply -f ${REPO_ROOT}/hack/local_registry.yaml
kubectl wait --for=condition=available --timeout=60s deployment/registry -n kube-registry

# Install the fake-gpu-operator
helm upgrade -i gpu-operator oci://ghcr.io/run-ai/fake-gpu-operator/fake-gpu-operator --namespace gpu-operator --create-namespace \
    --version 0.0.71 --values ${REPO_ROOT}/hack/fake-gpu-operator-values.yaml --wait

# Deploy Prometheus Operator
echo "Deploying Prometheus Operator..."
helm repo add prometheus-community https://prometheus-community.github.io/helm-charts --force-update
helm repo update prometheus-community
helm install prometheus prometheus-community/kube-prometheus-stack --namespace monitoring --create-namespace \
    --set "alertmanager.enabled=false" \
    --set "grafana.enabled=false" \
    --set "prometheus.enabled=false" \
    --wait

# Install the previous version of kai-scheduler from OCI registry
echo "Installing kai-scheduler $UPGRADE_FROM_VERSION from OCI registry..."
helm upgrade -i kai-scheduler oci://ghcr.io/nvidia/kai-scheduler/kai-scheduler -n kai-scheduler --create-namespace \
    --set "global.gpuSharing=true" --wait --version "$UPGRADE_FROM_VERSION"

# Create RBAC for fake-gpu-operator status updates
kubectl create clusterrole pods-patcher --verb=patch --resource=pods
kubectl create rolebinding fake-status-updater --clusterrole=pods-patcher --serviceaccount=gpu-operator:status-updater -n kai-resource-reservation

echo "Previous version $UPGRADE_FROM_VERSION installed. Building upgrade target..."

# Build the upgrade target (current version)
if [ -z "$PACKAGE_VERSION" ]; then
    GIT_REV=$(git rev-parse --short HEAD | sed 's/^0*//')
    PACKAGE_VERSION=0.0.0-$GIT_REV
fi

if [ "$LOCAL_IMAGES_BUILD" = "true" ]; then
    cd ${REPO_ROOT}
    echo "Building docker images with version $PACKAGE_VERSION..."
    make build DOCKER_REPO_BASE=localhost:30100 VERSION=$PACKAGE_VERSION

    # Start port-forward to local registry
    kubectl port-forward -n kube-registry deploy/registry 30100:5000 &
    PORT_FORWARD_PID=$!
    trap "kill $PORT_FORWARD_PID 2>/dev/null || true" EXIT
    sleep 2

    # Push images to local registry
    echo "Pushing images to local registry..."
    for image in $(docker images --format '{{.Repository}}:{{.Tag}}' | grep $PACKAGE_VERSION); do
        docker push $image
    done

    cd ${REPO_ROOT}
fi

# Package the new helm chart
helm package ./deployments/kai-scheduler -d ./charts --app-version $PACKAGE_VERSION --version $PACKAGE_VERSION
export UPGRADE_CHART_PATH=${REPO_ROOT}/charts/kai-scheduler-$PACKAGE_VERSION.tgz

echo "Upgrade chart path: $UPGRADE_CHART_PATH"

# Install ginkgo if it's not installed
if [ ! -f ${GOBIN}/ginkgo ]; then
    echo "Installing ginkgo"
    GOBIN=${GOBIN} go install github.com/onsi/ginkgo/v2/ginkgo@v2.25.3
fi

echo "Running upgrade tests..."
${GOBIN}/ginkgo -r --keep-going --trace -vv --label-filter 'upgrade' ${REPO_ROOT}/test/e2e/suites/upgrade

# Cleanup
rm -rf ${REPO_ROOT}/charts/kai-scheduler-$PACKAGE_VERSION.tgz

if [ "$PRESERVE_CLUSTER" != "true" ]; then
    kind delete cluster --name $CLUSTER_NAME
fi
