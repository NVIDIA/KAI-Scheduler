# Kubernetes Workload API Integration

## Introduction
Kubernetes v1.35 introduces the **Workload API (KEP-4671)**, a new standard for defining group scheduling requirements natively. This design extends KAI Scheduler to natively support this API, implementing a translation layer that maps standard `Workload` definitions to KAI's internal `PodGroup` mechanism. This ensures seamless scheduling for any application using the new Kubernetes standard while preserving KAI's advanced queuing and quota capabilities.

### Kubernetes Workload API Overview
The Workload API introduces a standard way to group pods for scheduling. It consists of two main components:

1.  **Workload Resource (`scheduling.k8s.io/v1alpha1`)**:
    A namespace-scoped resource that defines one or more "PodGroups," each with a specific scheduling policy (e.g., `gang` vs. `basic`).
    ```yaml
    kind: Workload
    spec:
      podGroups:
        - name: training-workers
          policy:
            gang:
              minCount: 4  # Minimum pods required to start
    ```

2.  **Pod Specification (`spec.workloadRef`)**:
    A new field in the Pod spec that explicitly links a Pod to a Workload and a specific group within it.
    ```yaml
    spec:
      workloadRef:
        name: my-workload         # References the Workload resource above
        podGroup: training-workers # References the specific group name
        podGroupReplicaKey: group-a # (Optional) Splits one group definition into multiple instances
    ```

## Design Plan

### 1. Precedence Rule
The system will be updated to check for the new Kubernetes **Workload Reference** (`spec.workloadRef`) on a Pod *before* checking for standard owners (like Jobs or ReplicaSets). 

*   **Logic**: If a Pod links to a Workload, that relationship becomes the primary source of truth for scheduling.
*   **Legacy Fallback**: If no Workload reference exists, the system falls back to existing logic (checking `OwnerReferences`).

### 2. Grouping Strategy
We will map the Kubernetes concept of a "Gang" directly to a KAI **PodGroup**. A unique KAI PodGroup will be created for every distinct combination of:

1.  The **Workload** resource (the parent).
2.  The specific **PodGroup Name** defined inside that Workload.
3.  The **Replica Key** (`podGroupReplicaKey`), if provided by the Pod.

This ensures that different replicas or different groups within the same Workload are scheduled independently as distinct gangs.

### 3. Policy & Attribute Translation
We will implement a new **Plugin** in the Pod Grouper architecture responsible for translating Workload API fields into KAI scheduling constraints:

*   **Gang Policy**: The `minCount` from the Workload API translates directly to the KAI `MinMember` requirement.
*   **Basic Policy**: We propose two options for handling "basic" policy (non-gang) workloads. The implementation choice may depend on scalability requirements and specific use cases.
    *   **Option A: Unified Group (Default)**: Map the entire Workload group to a single KAI PodGroup with **`MinMember: 1`**. This is scalable (1 CR per group) and allows centralized quota management, while still permitting pods to schedule independently.
    *   **Option B: Isolated Groups**: Map *each individual Pod* to its own dedicated KAI PodGroup (also with `MinMember: 1`). This provides maximum isolation (treating every pod as a distinct job) but significantly increases object overhead (`N` CRs for `N` pods).
*   **Queues & Priority**: These are resolved from the Workload resource first (e.g. labels), falling back to Pod labels if necessary.

### 4. Error Handling & Instant Recovery
If a Pod references a Workload or PodGroup that does not exist, strict validation is enforced:

*   **Pending State**: The Pod remains **Pending** and no KAI PodGroup is created. It is never scheduled as a standalone task.
*   **Instant Recovery**: To prevent long exponential backoff delays (up to ~1m) in the controller, we will implement a **Watcher** on `Workload` resources. As soon as a missing Workload is created, the watcher immediately triggers reconciliation for any pending Pods referencing it, ensuring instant scheduling.
