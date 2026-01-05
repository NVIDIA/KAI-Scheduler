# GPU Tetris (Topology-aware)

A tiny side-project that visualizes GPU allocation in a KAI-Scheduler cluster as a "Tetris" board, grouped by the KAI Topology CRD.

## What it does
- Downloads a KAI-Scheduler snapshot (`/get-snapshot`, zip containing `snapshot.json`) or reads a local snapshot zip.
- Converts it into a lightweight `viz.json` payload.
- Serves a single-page UI that renders per-node GPU columns with blocks for allocated pods.
- Provides a small "Create Pod" form in the UI to create a GPU pod and then observe where it lands.

## Run (local)
1) Port-forward the scheduler HTTP endpoint that has the snapshot plugin enabled.

2) Run:
- `cd cmd/gpu-tetris && go run . --snapshot-url http://localhost:8080/get-snapshot --listen :8099`

Alternative (from repo root):
- `go -C cmd/gpu-tetris run . --snapshot-url http://localhost:8080/get-snapshot --listen :8099`

3) Open:
- `http://localhost:8099/`

## Notes
- Allocations are derived from `BindRequest` objects in the snapshot (Succeeded only).
- Topology grouping uses the first `Topology` object in the snapshot; if none exist, all nodes are shown under a single root domain.

## Create Pod
- The UI submits `POST /api/pods` to the same server.
- The server uses kubeconfig (or in-cluster config) to create a pod with `schedulerName` set to `--scheduler-name` (default: `kai-scheduler`).
- Queue is set via label `kai.scheduler/queue`.

Useful flags:
- `--kubeconfig` (optional)
- `--scheduler-name`
- `--default-queue`
- `--default-namespace`
- `--default-image`
