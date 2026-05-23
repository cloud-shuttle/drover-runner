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

## Contributing

This is early-stage infrastructure. Roadmap items are captured in the Beads backlog. Contributions and discussions welcome via issues and PRs once the core daemon stabilizes.
