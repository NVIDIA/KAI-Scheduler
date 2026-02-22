# Generic Pod-Grouping

## Overview
In KAI, a wide range of third party workloads are supported via the pod-grouper. Plugins are written in Go, allowing custom pod-group creation logic according to each framework' needs: for example, correctly sub-grouping master/workers in pytorch/ray, interpreting topology requirements for grove, etc.

While this allows extensive support for many types, adding support for new frameworks or updating support for new features is bottle-necked by the KAI development cycle. In addition, the logic is coupled per KAI version, so improvements for users are tied to version upgrades.

In this design doc we will explore the range of features supported by the pod-grouper, and demonstrate how they can be implemented by a declarative API, allowing faster iterations and custom support per use case.

## User Stories

## Supported features

Per type of workload (defined by the kubernetes GVK of the topmost owner, or the highest supported member of the owner chain), a cluster-wide declarative API should allow the admin to define how all instances of this type will be "grouped" for scheduling purposes, meaning:

- How the podgroup and subgroups structure will look (flat, master/workers, or a complex hierarchy)
- Gang scheduling requirements at all levels (global and subgroup)
- Topology constraints (preferred and/or required, global and subgroup) 

**Resource Interface** (temporary name):
For the purpose of this document, we will define an instance of this api as a "Resource Interface", or RI. An RI references a specific GVK (for example, PytorchJob v1), and contains instructions on how to build podgroups for it.

## High level technical requirements and design

### Pre-defined hierarchy

It is important for the scheduler and pod-grouper to be able to determine all the relevant scheduling constraints (minMember definitions, topology requirements) at podgroup creation time: if the podgroup is created when not all constraints are known, it might cause a partial allocation of pods which then becomes invalid, causing unnecessary allocations and evictions.

### Podgrouper plugin vs RI

Defining grouping instructions for types with existing podgrouper implementations could introduce ambiguous scenarios, so we need to define which logic should take priority. It can be argued that an RI resource with scheduling definition is more intentional/explicit than the default pod-grouper plugin, and should take precedence. It might also make sense to allow this behavior to be toggled per workload type or even per workload.

The following is a proposal for how to resolve those. Phases 2-3 are only a suggestion for future versions, to be revisited only if actual use cases arise. 

**Phase 1: Use RI by default and revert to podgrouper**

If exists, use the RI to interpret scheduling restrictions. Otherwise, use pod-grouper plugin. 

**Phase 2: Global configuration, default-priorityclass style**

Same as 1, but allow explicit cluster-wide override by the admin, like we do for default priorities per type.

**Phase 3: workload-level override**

Extend options 1 & 2, but also allow a workload instance to override those with annotations

### Inferring subgroup scheduling constraints

Each subgroup in the podgroup tree can have it's own scheduling constraints (minSubgroups, minPods, topology requirements). RI needs to be able to allow us to interpret those. The RI can contain instructions per subgroup-node on where to find the subgroup's scheduling constraints. 

This is a fictitious example that assumes that ray has topology requirements in it's spec:
```yaml
    gangScheduling:
      podGroups:
        - name: cluster
          members:
            - componentName: head
              groupByKeyPaths:
                - .metadata.labels["ray.io/cluster"]
              requiredTopologyConstraintsPath: ".spec.headGroupSpec.topologyRequirements.Required"
              preferredTopologyConstraintsPath: ".spec.headGroupSpec.topologyRequirements.Preferred"
            - componentName: worker
              groupByKeyPaths:
                - .metadata.labels["ray.io/cluster"]
              requiredTopologyConstraintsPath: ".spec.workerGroupSpecs[].topologyRequirements.Required"
              preferredTopologyConstraintsPath: ".spec.workerGroupSpecs[].topologyRequirements.Preferred"
              minReplicasPath: .spec.workerGroupSpecs[].minReplicas
```

If a workload CRD doesn't have these definitions in the spec, annotations could be used: `requiredTopologyConstraintsPath: ".metadata.annotations.headGroupTopology.Required"`

**Common annotations**

It might make sense, as the need arises, to define common annotations that can be put on the workload object and interpreted by the podgrouper - after all, the same definitions of minPods/minSubgroups and topology can be populated to any subgroup. This is in line with current patterns like priority and preemptiblity annotations. Annotations can be defined as:

```yaml
metadata:
    annotations:
        kai.scheduler/topology-preferred/.root.workers: rack
        kai.scheduler/topology-preferred/.root.masters: node
```

Where .spec.workers refer to the path in the subgroup.