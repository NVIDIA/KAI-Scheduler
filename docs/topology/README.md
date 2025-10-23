# Topology Aware Scheduling
KAI Scheduler incorporates topology awareness and schedules workloads with consideration for the physical placement of nodes.

## Topological Information
Topology information is derived from both Kubernetes node labels and the Kueue Topology CRD:
```yaml
apiVersion: kueue.x-k8s.io/v1alpha1
kind: Topology
metadata:
  name: "cluster-topology"
spec:
  levels:
  - nodeLabel: "cloud.provider.com/topology-block"
  - nodeLabel: "cloud.provider.com/topology-rack"
  - nodeLabel: "kubernetes.io/hostname"
```
A topology definition must include at least one level with a label selector. Based on this configuration, KAI Scheduler organizes nodes into hierarchical domains aligned with the specified labels. 

## Topology Constraints
Workloads that require location-aware scheduling can declare topology constraints via annotations, using either required or preferred placement.

To enforce strict rack-level locality, a workload can be annotated as follows:
```yaml
kai.scheduler/topology: "cluster-topology"
kai.scheduler/topology-required-placement: "cloud.provider.com/topology-rack"
```
With a required placement, the scheduler will not place the workload on nodes lacking the specified label.

For soft affinity, the annotation can specify a preferred placement:
```yaml
kai.scheduler/topology: "cluster-topology"
kai.scheduler/topology-preferred-placement: "cloud.provider.com/topology-rack"
```
In this case, the scheduler will first target nodes matching the label. If no suitable nodes are available, it will fall back to a higher-level topology domain. 

You can also combine required and preferred placement in a single workload. In this model, the required constraint sets the upper boundary for eligible domains, while the preferred constraint guides the scheduler toward a more granular placement within that boundary. 
For example, a workload can require block-level placement while preferring rack-level locality, allowing tighter affinity without expanding the scheduling scope to the entire cluster.
Such constraint can be expressed as:
```yaml
kai.scheduler/topology: "cluster-topology"
kai.scheduler/topology-required-placement: "cloud.provider.com/topology-block"
kai.scheduler/topology-preferred-placement: "cloud.provider.com/topology-rack"
```