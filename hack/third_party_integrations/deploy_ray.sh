#!/bin/bash
set -e

#helm repo add kuberay https://ray-project.github.io/kuberay-helm/
#helm repo update
helm upgrade -i kuberay-operator --namespace ray --create-namespace kuberay/kuberay-operator --version 1.0.0