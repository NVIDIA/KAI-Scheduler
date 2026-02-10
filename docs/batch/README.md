# Batch and Gang Scheduling

## Overview

KAI Scheduler provides sophisticated workload scheduling with support for both batch scheduling and gang scheduling. The scheduler automatically detects the workload type and applies the appropriate scheduling strategy through the PodGrouper component, which creates PodGroup custom resources to coordinate pod scheduling.

## Definitions

### Batch Scheduling

Batch scheduling allows pods within a workload to be scheduled independently. Each pod is scheduled as resources become available, without waiting for other pods in the same workload. This is the default behavior for standard Kubernetes Jobs where individual pods can make progress independently.

In KAI Scheduler, batch-scheduled workloads are created with `minMember=1` in their PodGroup, meaning only one pod needs to be schedulable for the workload to start.

### Gang Scheduling

Gang scheduling ensures that either all pods in a workload are scheduled together, or none are scheduled until sufficient resources become available. This "all-or-nothing" approach prevents resource deadlocks and ensures distributed workloads can start simultaneously.

Gang scheduling is essential for:
- Distributed machine learning training (PyTorch, TensorFlow, MPI)
- Parallel computing workloads that require inter-pod communication
- Applications where partial scheduling would waste resources or cause deadlocks

In KAI Scheduler, gang-scheduled workloads have `minMember` set to the total number of required replicas, ensuring all pods are scheduled atomically.

## How It Works

The PodGrouper component automatically creates PodGroup custom resources for incoming workloads. Each workload type has a specialized plugin that determines the appropriate grouping logic:

- **Standard Jobs**: Create PodGroups with `minMember=1` (batch scheduling)
- **Distributed Training Jobs**: Create PodGroups with `minMember=<total replicas>` (gang scheduling)
- **JobSets**: Create one or multiple PodGroups depending on startup policy

For technical details on the PodGrouper architecture and plugin system, see [Pod Grouper Technical Details](../developer/pod-grouper.md).

## Supported Workload Types

### Standard Kubernetes Job

Standard Kubernetes Jobs run batch workloads where pods can be scheduled independently.

**Scheduling Behavior:** Batch scheduling (pods scheduled independently)

**External Requirements:** None (native Kubernetes resource)

**Example:**

```yaml
# Copyright 2025 NVIDIA CORPORATION
# SPDX-License-Identifier: Apache-2.0

apiVersion: batch/v1
kind: Job
metadata:
  name: batch-job-example
  labels:
    kai.scheduler/queue: default-queue
spec:
  parallelism: 2
  completions: 2
  template:
    spec:
      schedulerName: kai-scheduler
      restartPolicy: OnFailure
      containers:
        - name: worker
          image: ubuntu:latest
          command: ["/bin/bash"]
          args:
            - -c
            - |
              echo "Starting batch job worker"
              echo "Processing task $HOSTNAME"
              sleep 30
              echo "Task completed successfully"
          resources:
            requests:
              cpu: "100m"
              memory: "128Mi"
```

Apply: `kubectl apply -f docs/batch/examples/job.yaml`

### PyTorchJob (Kubeflow Training Operator)

PyTorchJob enables distributed PyTorch training across multiple GPUs and nodes using the Kubeflow Training Operator.

**Scheduling Behavior:** Gang scheduling (all pods scheduled together)

**External Requirements:**
- Kubeflow Training Operator v1
- Installation: `kubectl apply -k "github.com/kubeflow/training-operator/manifests/overlays/standalone?ref=v1.8.1"`
- Documentation: https://www.kubeflow.org/docs/components/training/

**Example:**

```yaml
# Copyright 2025 NVIDIA CORPORATION
# SPDX-License-Identifier: Apache-2.0

apiVersion: "kubeflow.org/v1"
kind: "PyTorchJob"
metadata:
  name: "pytorch-dist-mnist"
  labels:
    kai.scheduler/queue: default-queue
spec:
  pytorchReplicaSpecs:
    Master:
      replicas: 1
      restartPolicy: OnFailure
      template:
        spec:
          schedulerName: kai-scheduler
          containers:
            - name: pytorch
              image: ghcr.io/kubeflow/training-v1/pytorch-dist-mnist:latest
              args: ["--backend", "nccl", "--batch-size", "2048", "--epochs", "900"]
              resources:
                limits:
                  nvidia.com/gpu: "1"
    Worker:
      replicas: 2
      restartPolicy: OnFailure
      template:
        spec:
          schedulerName: kai-scheduler
          containers:
            - name: pytorch
              image: ghcr.io/kubeflow/training-v1/pytorch-dist-mnist:latest
              args: ["--backend", "nccl", "--batch-size", "2048", "--epochs", "900"]
              resources:
                limits:
                  nvidia.com/gpu: "1"
```

Apply: `kubectl apply -f docs/batch/examples/pytorchjob.yaml`

This example trains a distributed MNIST model using NCCL backend with 1 master and 2 worker pods, each with 1 GPU. All 3 pods will be scheduled together via gang scheduling.

### MPIJob (Kubeflow Training Operator)

MPIJob enables distributed training and HPC workloads using the Message Passing Interface (MPI) protocol.

**Scheduling Behavior:** Gang scheduling (all pods scheduled together)

**External Requirements:**
- MPI Operator v2beta1 or Kubeflow Training Operator v1
- Installation: `kubectl apply -k "github.com/kubeflow/training-operator/manifests/overlays/standalone?ref=v1.8.1"`
- Documentation: https://www.kubeflow.org/docs/components/training/mpi/

**Example:**

```yaml
# Copyright 2025 NVIDIA CORPORATION
# SPDX-License-Identifier: Apache-2.0

apiVersion: "kubeflow.org/v2beta1"
kind: "MPIJob"
metadata:
  name: "mpi-hello-world"
  labels:
    kai.scheduler/queue: default-queue
spec:
  slotsPerWorker: 1
  mpiReplicaSpecs:
    Launcher:
      replicas: 1
      template:
        spec:
          schedulerName: kai-scheduler
          restartPolicy: OnFailure
          containers:
            - name: mpi-launcher
              image: mpioperator/mpi-pi:openmpi
              command:
                - mpirun
                - -n
                - "2"
                - /home/mpiuser/pi
              resources:
                requests:
                  cpu: "100m"
                  memory: "128Mi"
    Worker:
      replicas: 2
      template:
        spec:
          schedulerName: kai-scheduler
          restartPolicy: OnFailure
          containers:
            - name: mpi-worker
              image: mpioperator/mpi-pi:openmpi
              resources:
                requests:
                  cpu: "100m"
                  memory: "128Mi"
```

Apply: `kubectl apply -f docs/batch/examples/mpijob.yaml`

This example runs a distributed MPI calculation of Pi with 1 launcher and 2 worker pods.

### TFJob (Kubeflow Training Operator)

TFJob enables distributed TensorFlow training across multiple nodes.

**Scheduling Behavior:** Gang scheduling (all pods scheduled together)

**External Requirements:**
- Kubeflow Training Operator v1
- Installation: `kubectl apply -k "github.com/kubeflow/training-operator/manifests/overlays/standalone?ref=v1.8.1"`
- Documentation: https://www.kubeflow.org/docs/components/training/tftraining/

**Example:**

```yaml
# Copyright 2025 NVIDIA CORPORATION
# SPDX-License-Identifier: Apache-2.0

apiVersion: "kubeflow.org/v1"
kind: "TFJob"
metadata:
  name: "tf-dist-mnist"
  labels:
    kai.scheduler/queue: default-queue
spec:
  tfReplicaSpecs:
    Chief:
      replicas: 1
      restartPolicy: OnFailure
      template:
        spec:
          schedulerName: kai-scheduler
          containers:
            - name: tensorflow
              image: kubeflow/tf-mnist-with-summaries:latest
              command:
                - "python"
                - "/var/tf_mnist/mnist_with_summaries.py"
                - "--log_dir=/train/logs"
                - "--learning_rate=0.01"
                - "--batch_size=150"
              resources:
                requests:
                  cpu: "500m"
                  memory: "512Mi"
    Worker:
      replicas: 2
      restartPolicy: OnFailure
      template:
        spec:
          schedulerName: kai-scheduler
          containers:
            - name: tensorflow
              image: kubeflow/tf-mnist-with-summaries:latest
              command:
                - "python"
                - "/var/tf_mnist/mnist_with_summaries.py"
                - "--log_dir=/train/logs"
                - "--learning_rate=0.01"
                - "--batch_size=150"
              resources:
                requests:
                  cpu: "500m"
                  memory: "512Mi"
```

Apply: `kubectl apply -f docs/batch/examples/tfjob.yaml`

This example trains a distributed MNIST model with 1 chief and 2 worker replicas.

### XGBoostJob (Kubeflow Training Operator)

XGBoostJob enables distributed XGBoost training for gradient boosting workloads.

**Scheduling Behavior:** Gang scheduling (all pods scheduled together)

**External Requirements:**
- Kubeflow Training Operator v1
- Installation: `kubectl apply -k "github.com/kubeflow/training-operator/manifests/overlays/standalone?ref=v1.8.1"`
- Documentation: https://www.kubeflow.org/docs/components/training/xgboost/

**Example:**

```yaml
# Copyright 2025 NVIDIA CORPORATION
# SPDX-License-Identifier: Apache-2.0

apiVersion: "kubeflow.org/v1"
kind: "XGBoostJob"
metadata:
  name: "xgboost-dist-iris"
  labels:
    kai.scheduler/queue: default-queue
spec:
  xgbReplicaSpecs:
    Master:
      replicas: 1
      restartPolicy: OnFailure
      template:
        spec:
          schedulerName: kai-scheduler
          containers:
            - name: xgboost
              image: docker.io/kubeflow/xgboost-dist-iris:latest
              ports:
                - containerPort: 9991
                  name: xgboostjob-port
              args:
                - "--job_type=Train"
                - "--xgboost_parameter=objective:multi:softprob,num_class:3"
                - "--n_estimators=10"
                - "--learning_rate=0.1"
                - "--model_path=/tmp/xgboost-model"
                - "--model_storage_type=local"
              resources:
                requests:
                  cpu: "500m"
                  memory: "512Mi"
    Worker:
      replicas: 2
      restartPolicy: OnFailure
      template:
        spec:
          schedulerName: kai-scheduler
          containers:
            - name: xgboost
              image: docker.io/kubeflow/xgboost-dist-iris:latest
              ports:
                - containerPort: 9991
                  name: xgboostjob-port
              args:
                - "--job_type=Train"
                - "--xgboost_parameter=objective:multi:softprob,num_class:3"
                - "--n_estimators=10"
                - "--learning_rate=0.1"
              resources:
                requests:
                  cpu: "500m"
                  memory: "512Mi"
```

Apply: `kubectl apply -f docs/batch/examples/xgboostjob.yaml`

This example trains a distributed Iris classification model using XGBoost with 1 master and 2 worker pods. All 3 pods will be scheduled together via gang scheduling.

### JAXJob (Kubeflow Training Operator)

JAXJob enables distributed JAX training workloads using JAX's native distributed capabilities.

**Scheduling Behavior:** Gang scheduling (all pods scheduled together)

**External Requirements:**
- Kubeflow Training Operator v1
- Installation: `kubectl apply -k "github.com/kubeflow/training-operator/manifests/overlays/standalone?ref=v1.8.1"`
- Documentation: https://www.kubeflow.org/docs/components/training/

**Example:**

```yaml
# Copyright 2025 NVIDIA CORPORATION
# SPDX-License-Identifier: Apache-2.0

apiVersion: "kubeflow.org/v1"
kind: "JAXJob"
metadata:
  name: "jax-dist-mnist"
  labels:
    kai.scheduler/queue: default-queue
spec:
  jaxReplicaSpecs:
    Worker:
      replicas: 4
      restartPolicy: OnFailure
      template:
        spec:
          schedulerName: kai-scheduler
          containers:
            - name: jax
              image: docker.io/kubeflow/jax-mnist:latest
              ports:
                - containerPort: 6666
                  name: jaxjob-port
              env:
                - name: JAX_COORDINATOR_PORT
                  value: "6666"
                - name: JAX_NUM_PROCESSES
                  value: "4"
              args:
                - "--num-epochs=10"
                - "--batch-size=128"
                - "--learning-rate=0.001"
              resources:
                requests:
                  cpu: "1"
                  memory: "2Gi"
                limits:
                  nvidia.com/gpu: "1"
```

Apply: `kubectl apply -f docs/batch/examples/jaxjob.yaml`

This example trains a distributed MNIST model using JAX with 4 worker pods, each with 1 GPU. All 4 pods will be scheduled together via gang scheduling.

### RayJob (KubeRay Operator)

RayJob enables distributed computing and machine learning workloads using the Ray framework.

**Scheduling Behavior:** Gang scheduling (all pods in the Ray cluster scheduled together)

**External Requirements:**
- KubeRay Operator v1
- Installation: `kubectl apply -f https://raw.githubusercontent.com/ray-project/kuberay/v1.0.0/ray-operator/config/default/ray-operator.yaml`
- Documentation: https://docs.ray.io/en/latest/cluster/kubernetes/index.html

**Example:**

```yaml
# Copyright 2025 NVIDIA CORPORATION
# SPDX-License-Identifier: Apache-2.0

apiVersion: ray.io/v1
kind: RayJob
metadata:
  name: rayjob-example
  labels:
    kai.scheduler/queue: default-queue
spec:
  entrypoint: python /home/ray/samples/sample_code.py
  rayClusterSpec:
    rayVersion: '2.46.0'
    headGroupSpec:
      rayStartParams: {}
      template:
        spec:
          schedulerName: kai-scheduler
          containers:
          - name: ray-head
            image: rayproject/ray:2.46.0
            resources:
              limits:
                cpu: "1"
                nvidia.com/gpu: "1"
              requests:
                cpu: "200m"
                memory: "256Mi"
    workerGroupSpecs:
    - replicas: 2
      minReplicas: 1
      maxReplicas: 5
      groupName: gpu-workers
      rayStartParams: {}
      template:
        spec:
          schedulerName: kai-scheduler
          containers:
          - name: ray-worker
            image: rayproject/ray:2.46.0
            resources:
              limits:
                cpu: "1"
                nvidia.com/gpu: "1"
              requests:
                cpu: "200m"
                memory: "256Mi"
```

Apply: `kubectl apply -f docs/batch/examples/rayjob.yaml`

This example creates a Ray cluster with 1 head node and 2 worker nodes, each with GPU resources.

### JobSet (Kubernetes SIG)

JobSet manages a group of Jobs as a single unit, enabling complex multi-job workflows.

**Scheduling Behavior:** Gang scheduling per replicatedJob or for all jobs (depends on startup policy)

**External Requirements:**
- JobSet controller v1alpha2
- Installation: `kubectl apply --server-side -f https://github.com/kubernetes-sigs/jobset/releases/download/v0.5.2/manifests.yaml`
- Documentation: https://github.com/kubernetes-sigs/jobset

**Example:**

```yaml
# Copyright 2025 NVIDIA CORPORATION
# SPDX-License-Identifier: Apache-2.0

apiVersion: jobset.x-k8s.io/v1alpha2
kind: JobSet
metadata:
  name: jobset-example
  labels:
    kai.scheduler/queue: default-queue
spec:
  startupPolicy:
    startupPolicyOrder: AnyOrder
  successPolicy:
    operator: All
  failurePolicy:
    maxRestarts: 3
  replicatedJobs:
    - name: coordinator
      replicas: 1
      template:
        spec:
          parallelism: 1
          completions: 1
          backoffLimit: 0
          template:
            spec:
              schedulerName: kai-scheduler
              restartPolicy: OnFailure
              containers:
                - name: coordinator
                  image: busybox:1.36
                  command:
                    - /bin/sh
                    - -c
                    - |
                      echo "Coordinator starting"
                      sleep 10
                      echo "Coordinator completed"
                  resources:
                    requests:
                      cpu: "100m"
                      memory: "128Mi"
    - name: worker
      replicas: 2
      template:
        spec:
          parallelism: 2
          completions: 2
          backoffLimit: 0
          template:
            spec:
              schedulerName: kai-scheduler
              restartPolicy: OnFailure
              containers:
                - name: worker
                  image: busybox:1.36
                  command:
                    - /bin/sh
                    - -c
                    - |
                      echo "Worker starting: $HOSTNAME"
                      sleep 15
                      echo "Worker completed: $HOSTNAME"
                  resources:
                    requests:
                      cpu: "100m"
                      memory: "128Mi"
```

Apply: `kubectl apply -f docs/batch/examples/jobset.yaml`

This example creates a JobSet with a coordinator and worker jobs. With `startupPolicyOrder: AnyOrder`, KAI creates one PodGroup for all jobs together. If you use `startupPolicyOrder: InOrder`, KAI creates separate PodGroups to avoid sequencing deadlocks.

### SparkApplication (Spark Operator)

SparkApplication enables running Apache Spark workloads on Kubernetes.

**Scheduling Behavior:** Gang scheduling (driver and executors scheduled together)

**External Requirements:**
- Spark Operator for Kubernetes
- Installation: `helm repo add spark-operator https://googlecloudplatform.github.io/spark-on-k8s-operator && helm install spark spark-operator/spark-operator --namespace spark-operator --create-namespace`
- Documentation: https://github.com/GoogleCloudPlatform/spark-on-k8s-operator

**Example:**

```yaml
# Copyright 2025 NVIDIA CORPORATION
# SPDX-License-Identifier: Apache-2.0

apiVersion: sparkoperator.k8s.io/v1beta2
kind: SparkApplication
metadata:
  name: spark-pi
  labels:
    kai.scheduler/queue: default-queue
spec:
  type: Scala
  mode: cluster
  image: "apache/spark:3.5.0"
  imagePullPolicy: IfNotPresent
  mainClass: org.apache.spark.examples.SparkPi
  mainApplicationFile: "local:///opt/spark/examples/jars/spark-examples_2.12-3.5.0.jar"
  sparkVersion: "3.5.0"
  restartPolicy:
    type: Never
  driver:
    cores: 1
    coreLimit: "1200m"
    memory: "512m"
    labels:
      version: 3.5.0
    serviceAccount: spark
    schedulerName: kai-scheduler
  executor:
    cores: 1
    instances: 2
    memory: "512m"
    labels:
      version: 3.5.0
    schedulerName: kai-scheduler
```

Apply: `kubectl apply -f docs/batch/examples/sparkapplication.yaml`

This example runs the SparkPi example with 1 driver and 2 executors. Note: You must create a `spark` service account with appropriate RBAC permissions before running Spark applications.

## Topology-Aware Scheduling

For distributed workloads, you can optionally specify topology constraints to control pod placement across racks, zones, or other hierarchical domains. This is particularly useful for workloads that require low-latency communication between pods or need to avoid network bottlenecks.

Add topology annotations to your workload metadata:

```yaml
metadata:
  annotations:
    kai.scheduler/topology: "cluster-topology"
    kai.scheduler/topology-preferred-placement: "topology.kubernetes.io/rack"
```

Available placement modes:
- **Required placement**: Strictly enforces placement within specified topology domain
- **Preferred placement**: Attempts to place pods together but falls back to higher-level domains if needed

See [Topology-Aware Scheduling](../topology/README.md) for comprehensive documentation on topology configuration and scheduling strategies.

## Additional Resources

- [Pod Grouper Technical Details](../developer/pod-grouper.md) - Deep dive into PodGrouper architecture and plugin system
- [Topology-Aware Scheduling](../topology/README.md) - Configure topology-aware scheduling for distributed workloads
- [Kubeflow Training Operator](https://www.kubeflow.org/docs/components/training/) - Official documentation for distributed training jobs
- [KubeRay Documentation](https://docs.ray.io/en/latest/cluster/kubernetes/index.html) - Ray on Kubernetes guide
- [JobSet Documentation](https://github.com/kubernetes-sigs/jobset) - Kubernetes JobSet API reference
- [Spark Operator Documentation](https://github.com/GoogleCloudPlatform/spark-on-k8s-operator) - Spark on Kubernetes operator
