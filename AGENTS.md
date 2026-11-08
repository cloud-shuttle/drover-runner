# Agent Context for Drover Runner

Welcome, AI Agent. This document guides work in `drover-runner`.

## Ecosystem Role

> **Part of the Drover Ecosystem**: **Drover Runner** is the **unikernel platform runner and hypervisor daemon** for secure, multi-tenant microVM workloads. It provides a **local Kraft/QEMU dev loop** (`pkg/kraft`, future `dvr` CLI) and a **production path** via Firecracker on Linux with TAP networking and ephemeral storage purge. The main **Drover** coordinator dispatches agent workloads into this runtime; hosted scale also aligns with **Unikraft Cloud** via the `ExecutionSandbox` seam.

**Org index:** [`../AGENTS.md`](../AGENTS.md). **Architecture context:** [`../docs/architecture/tdd-seams-walkthrough.md`](../docs/architecture/tdd-seams-walkthrough.md) (Unikernel Platform Runner seam). **Deep dive:** [`docs/diataxis/explanation/architecture-and-security.md`](docs/diataxis/explanation/architecture-and-security.md).

## What this repo is

- **Early-stage infrastructure** — foundational seams and daemon APIs; not yet a full production hypervisor fleet.
- **Local verification** — wraps the official **`kraft`** CLI for build/run/clean without reimplementing Unikraft tooling.
- **Host daemon** — `pkg/daemon` REST server scheduling Guest VMs through `HypervisorDriver` (`QemuDriver` vs `FirecrackerDriver`).
- **Isolation subsystems** — `pkg/network` (TAP/bridge), `pkg/firecracker` (microVM orchestration), `pkg/storage` (secure purge / shred).
- **Cloud seam** — `pkg/runner.ExecutionSandbox` for Unikraft Cloud REST (create/status/logs/delete).

## Layout

| Path | Role |
|------|------|
| `pkg/runner/` | `ExecutionSandbox` interface + Unikraft Cloud adapter, tests |
| `pkg/daemon/` | `HypervisorDriver`, `Server` REST API, config, telemetry hooks |
| `pkg/kraft/` | Kraftfile parsing, `kraft` subprocess build/run |
| `pkg/firecracker/` | Firecracker API orchestration |
| `pkg/network/` | `NetworkManager` — Linux TAP vs dev mocks |
| `pkg/storage/` | Ephemeral overlay purge (`ShredFile`, audit records) |
| `docs/diataxis/` | Tutorials, how-tos, reference (`dvr` CLI), architecture explanation |
| `docs/drun-*.md` | Epic/task design notes (e.g. drun-001 Kraft integration) |
| `examples/` | Sample unikernel projects (may contain `.unikraft/build` artefacts — do not commit build output) |
| `scripts/quality-gate.py` | CRAP + coverage audit |

**CLI entry (documented, may be in progress):** `cmd/dvr` — build with `go build -o bin/dvr ./cmd/dvr` per [`README.md`](README.md) and [`docs/diataxis/reference/cli.md`](docs/diataxis/reference/cli.md). Prefer extending `pkg/kraft` and wiring thin Cobra in `cmd/dvr` rather than shelling out from handlers.

## Commands

From **repo root**:

```bash
go test ./...
go build ./...
```

Local unikernel loop (requires **`kraft`** on `PATH`):

```bash
go build -o bin/dvr ./cmd/dvr   # when cmd/dvr is present
./bin/dvr unikernel build ./path/to/project
./bin/dvr unikernel run ./path/to/project
```

CI: [`.github/workflows/ci.yml`](.github/workflows/ci.yml) — `go test -race`, coverage artefact, quality gate (CRAP limit 30000).

## Conventions

- **Seam-first:** Daemon and coordinators depend on `HypervisorDriver`, `NetworkManager`, and `ExecutionSandbox` interfaces — inject concrete drivers in `main`/server setup, not in HTTP handlers.
- **TDD before adapters:** Extend [`pkg/runner/sandbox_test.go`](pkg/runner/sandbox_test.go), [`pkg/firecracker/orchestrator_test.go`](pkg/firecracker/orchestrator_test.go), [`pkg/network/network_test.go`](pkg/network/network_test.go), [`pkg/storage/purge_test.go`](pkg/storage/purge_test.go) when changing behaviour.
- **Platform branching:** Use `runtime.GOOS` patterns already in `pkg/network` — QEMU/kraft on macOS dev, Firecracker + iptables paths on Linux production.
- **Tenant isolation:** Never share TAP devices, overlay paths, or env maps across instance IDs; purge storage on teardown (see drun-006 design).
- **Do not commit** `examples/**/.unikraft/build/` or other generated unikernel artefacts.
- **Backlog:** Beads in [`.beads/issues.jsonl`](.beads/issues.jsonl) (namespace `drun-*`).

## Boundaries (do not conflate)

| Concern | Owner |
|---------|--------|
| Parallel job coordinator / dispatch | **Drover** (`drover/`) |
| Agent loop & tool execution | **Drover Code**, **Drover Cloud** agent jobs |
| Policy / HITL gates | **Drover Guard**, **Drover Muster** |
| Trace & span UI | **Drover Trace** |
| MicroVM boot, network, purge | **This repo** |

## Related repos

- **Drover** — orchestration and cross-cutting platform docs
- **Drover Cloud** — hosted agent jobs that may target runner sandboxes
- **Drover Trace** — telemetry from daemon metrics (ClickHouse) when wired
