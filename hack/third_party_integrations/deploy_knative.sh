#!/bin/bash
# Copyright 2025 NVIDIA CORPORATION
# SPDX-License-Identifier: Apache-2.0
set -e

kubectl apply -f https://github.com/knative/operator/releases/download/knative-v1.17.5/operator.yaml
cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: Namespace
metadata:
  name: knative-serving
---
apiVersion: operator.knative.dev/v1beta1
kind: KnativeServing
metadata:
  name: knative-serving
  namespace: knative-serving
EOF
sleep 20
kubectl apply -f https://github.com/knative/serving/releases/download/knative-v1.15.2/serving-core.yaml
kubectl patch configmap/config-autoscaler --namespace knative-serving --type merge --patch '{"data":{"enable-scale-to-zero":"true"}}'
kubectl patch configmap/config-features --namespace knative-serving --type merge --patch '{"data":{"kubernetes.podspec-schedulername":"enabled","kubernetes.podspec-affinity":"enabled","kubernetes.podspec-tolerations":"enabled","kubernetes.podspec-volumes-emptydir":"enabled","kubernetes.podspec-securitycontext":"enabled","kubernetes.containerspec-addcapabilities":"enabled","kubernetes.podspec-persistent-volume-claim":"enabled","kubernetes.podspec-persistent-volume-write":"enabled","multi-container":"enabled","kubernetes.podspec-init-containers":"enabled"}}'
echo "Knative installed"
