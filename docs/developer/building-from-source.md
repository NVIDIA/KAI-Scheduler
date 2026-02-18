# Building from Source

## Prerequisites

- **Docker** (with a working credential helper): All `make build` and `make test` targets that involve building or running container images require Docker to be installed and properly configured.
  - On **WSL (Windows Subsystem for Linux)** with Docker Desktop: Docker Desktop's credential helper (`docker-credential-desktop.exe`) is a Windows executable and is **not available** inside WSL. If `make test` or `make build` fails with an error like `exec: "docker-credential-desktop.exe": executable file not found in $PATH`, fix this by editing `~/.docker/config.json` inside WSL and removing or replacing the `credsStore` entry:
    ```json
    {
      "credsStore": ""
    }
    ```
    Then run `docker login` from within WSL to re-authenticate.
  - Alternatively, you can run individual Go unit tests **without Docker** using:
    ```sh
    go test ./...
    ```
    This skips the Docker builder image and runs unit tests directly on the host. Integration/envtest tests that require a Kubernetes API server still need Docker.

- **Go** `1.24+`: Required if you run `go test ./...` or build binaries directly on the host.
- **Helm** `3.x`: Required for packaging and testing the Helm chart (`make test-chart`).
- **kind** (optional): For loading images into a local cluster.

## Build and Deploy Steps

To build and deploy KAI Scheduler from source, follow these steps:

1. Clone the repository:
   ```sh
   git clone git@github.com:NVIDIA/KAI-scheduler.git
   cd KAI-scheduler
   ```

2. Build the container images, these images will be built locally (not pushed to a remote registry)
   ```sh
   make build
   ```
   If you want to push the images to a private docker registry, you can set in DOCKER_REPO_BASE var: 
   ```sh
   DOCKER_REPO_BASE=<REGISTRY-URL> make build
   ```

3. Package the Helm chart:
   ```sh
   helm package ./deployments/kai-scheduler -d ./charts
   ```
   
4. Make sure the images are accessible from cluster nodes, either by pushing the images to a private registry or loading them to nodes cache.
   For example, you can load the images to kind cluster with this command:
   ```sh
   for img in $(docker images --format '{{.Repository}}:{{.Tag}}' | grep kai-scheduler); 
      do kind load docker-image $img --name <KIND-CLUSTER-NAME>; done
   ```

5. Install on your cluster:
   ```sh
   helm upgrade -i kai-scheduler -n kai-scheduler --create-namespace ./charts/kai-scheduler-0.0.0.tgz
   ```