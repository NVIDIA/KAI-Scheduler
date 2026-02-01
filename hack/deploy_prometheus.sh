#!/bin/bash
# Copyright 2025 NVIDIA CORPORATION
# SPDX-License-Identifier: Apache-2.0

set -e

REPO_ROOT=$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )/..

echo "Adding Prometheus community Helm repo..."
helm repo add prometheus-community https://prometheus-community.github.io/helm-charts
helm repo update

echo "Installing Prometheus"
helm upgrade -i prometheus prometheus-community/kube-prometheus-stack \
    --namespace monitoring \
    --create-namespace \
    --wait

echo "Applying Prometheus configuration..."
kubectl apply -f ${REPO_ROOT}/docs/metrics/prometheus.yaml

echo "Creating ServiceMonitors for KAI components..."
kubectl apply -f ${REPO_ROOT}/docs/metrics/service-monitors.yaml

echo "Installing prometheus-adapter..."
helm upgrade -i prometheus-adapter prometheus-community/prometheus-adapter \
    --namespace monitoring \
    --values ${REPO_ROOT}/docs/metrics/prometheus-adapter-values.yaml \
    --wait

echo "Waiting for prometheus-adapter to be ready..."
kubectl rollout status deployment/prometheus-adapter -n monitoring --timeout=120s

sleep 10

echo "Verifying custom metrics."
kubectl get --raw "/apis/custom.metrics.k8s.io/v1beta1/namespaces/kai-scheduler/pods/*/controller_runtime_webhook_requests_per_second" | jq .
kubectl get --raw "/apis/custom.metrics.k8s.io/v1beta1/namespaces/kai-scheduler/pods/*/cpu_utilization" | jq .

echo "Prometheus installation complete."