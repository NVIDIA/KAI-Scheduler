In v0.6.0 release the following changes were made:

## Resource Reservation Namespace
The name of the resource reservation namespace was changed from `runai-reservation` to `kai-resource-reservation`. 
In order to safely migrate from the previous namespace, GPU sharing workloads must be deleted before the upgrade and reservation pods should not be found in `runai-reservation` namespace.
The following command should result without any existing pods:
```
kubectl get pods -n runai-reservation
```

## Scheduling Queue Label Key
The label key for a scheduling queue was changed from `runai/queue` to `kai.scheduler/queue`.
In order to adopt the new label key, all workloads must have the new label with the name of the respective queue before the upgrade.
The following command should result without any existing pods:
```
kubectl get pods -A -l 'runai/queue'
```

Docs and examples have been updated to reflect these changes.
If adopting these changes is not possible at current time, the following flag can be added to the `helm upgrade` command. This will configure KAI Scheduler to use the old values until the cluster is updated to the new values:
```
--values ./docs/backwardcompatibility/v0.6.0/values.yaml
```
