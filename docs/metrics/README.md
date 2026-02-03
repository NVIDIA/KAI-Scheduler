# Kube-Prometheus-Stack
install prometheus operator and enable prometheus instance (and grafana if you want):
```
helm repo add prometheus-community https://prometheus-community.github.io/helm-charts
helm repo update prometheus-community
helm upgrade -i --create-namespace -n monitoring kube-prometheus-stack prometheus-community/kube-prometheus-stack --values kube-prometheus-values.yaml
```

# Service Monitors for kai services

Install a prometheus instance and the relevant service monitors in kai-scheduler namespace:

```sh
kubectl apply -f prometheus.yaml
kubectl apply -f service-monitors.yaml
```

To enable the prometheus as a grafana datasource, if desired, apply grafana-datasource.yaml:

```sh
kubectl apply -f grafana-datasource.yaml
```

# Prometheus Adapter

Prometheus Adapter exposes Prometheus metrics as Kubernetes custom metrics API, enabling HPA to scale based on custom metrics.

## Installation

Add the Prometheus Adapter Helm repository:
```bash
helm repo add prometheus-community https://prometheus-community.github.io/helm-charts
helm repo update prometheus-community
```

Install Prometheus Adapter with custom metric rules:
```bash
helm upgrade -i --create-namespace -n monitoring prometheus-adapter prometheus-community/prometheus-adapter --values prometheus-adapter-values.yaml
```