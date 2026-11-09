# drover-runner

Unikernel Platform Runner and Hypervisor daemon for executing secure multi-tenant microVM containers.

Part of the [Drover](https://github.com/cloud-shuttle/drover) parallel orchestration platform.

## Key Features

- **Local Sandbox Verification**: Integrates with local `kraft` CLI to run unikernel targets offline.
- **Firecracker Orchestration**: Spawns isolated multi-tenant guest OS slices using Firecracker microVMs on KVM hosts.
- **Virtual Isolation**: Configures isolated virtual bridge networks (TAP/TUN) and secure overlay disks with dynamic memory cleanup on completion.

## Status

🚧 Early development. Foundational components for secure unikernel execution environments.

## Backlog & Roadmap

Work items, epics, and tasks are tracked locally in JSON Lines format inside [`.beads/issues.jsonl`](.beads/issues.jsonl) following the platform's Beads convention.

See the main [drover](https://github.com/cloud-shuttle/drover) repository for the coordinator, architecture, and cross-cutting concerns.

## Local Development (drun-001)

> **Docs correction (rationalization 2026-11-09):** the default tree is a **Go package library** (`pkg/kraft`, `pkg/firecracker`, `pkg/network`, `pkg/storage`, `pkg/daemon`, `pkg/runner`). There is **no `cmd/` / `main.go` entrypoint** and no `./cmd/dvr` binary in this repository today. Local sandbox UX lives primarily in [`drover-cli`](https://github.com/cloud-shuttle/drover-cli). Managed UKC remote workers live in [`drover-code`](https://github.com/cloud-shuttle/drover-code).

Import the packages from a consumer (or a future thin `cmd` wrapper):

```bash
go test ./...
```

Historical kraft-integration design notes: [docs/drun-001-kraft-integration-design.md](docs/drun-001-kraft-integration-design.md).

**Consolidation candidate (not executed):** conditional merge into `drover-cli/internal/unikernel` / `internal/hypervisor`, preserving backend interfaces and tests. Privileged KVM/TAP code must not move into Cloud's public control-plane process. Requires Peter's explicit approval.

## Contributing

This is early-stage infrastructure. Roadmap items are captured in the Beads backlog. Contributions and discussions welcome via issues and PRs once the core daemon stabilizes.
