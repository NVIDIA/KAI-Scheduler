# Kube-Prometheus-Stack
install prometheus operator and enable prometheus instance (and grafana if you want):
```
helm repo add prometheus-community https://prometheus-community.github.io/helm-charts
helm repo update prometheus-community
helm upgrade -i --create-namespace -n monitoring kube-prometheus-stack prometheus-community/kube-prometheus-stack --set prometheus.enabled=true --set grafana.enabled=true
```

# Service Monitors for kai services

Install a prometheus instance in kai-scheduler namespace:
```
kubectl apply -f prometheus.yaml
```

And the service monitors for the relevant services:

```
kubectl apply -f service-monitors.yaml
```