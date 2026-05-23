# drun-001: Local KraftKit CLI Integration for dvr — Design & Implementation Breakdown

**Status**: In progress (scaffolding started)  
**Epic**: drun-000  
**Priority**: P1  
**Goal**: Bind `dvr` CLI commands to the official `kraft` (KraftKit) engine so developers get a fast, reliable, offline local verification loop for Unikraft unikernels.

## Acceptance Criteria (from backlog)

- Binds `dvr --unikernel` commands to the open-source kraft CLI engine
- Compiles Kraftfiles locally and creates offline build targets
- Captures and pipes compilation errors to the developer console

## Context & Why This First

`drover-runner` is the secure execution runtime that the main [drover](https://github.com/cloud-shuttle/drover) coordinator will dispatch agent workloads into.

Before we can build the production multi-tenant Firecracker daemon (drun-003+), we need a **local developer loop** that lets people:

1. Write or consume Unikraft unikernels
2. Build them quickly on their laptop (QEMU)
3. Run them locally with good UX
4. Iterate without needing bare metal or the full hypervisor

`kraft` (https://unikraft.org) is the official, mature CLI for exactly this. We do **not** re-implement the build system — we wrap it beautifully under the `dvr` brand and prepare the surface for future daemon commands.

## Proposed Command Surface (v0.1 for drun-001/002)

```bash
dvr [global flags]
  version
  unikernel (aliases: uk, kernel)
    build [path]            # primary: compile via kraft
    run [path|binary]       # execute locally via kraft (QEMU by default)
    clean [path]
    targets [path]          # discover what the Kraftfile can produce
    new                     # (stretch) scaffold a minimal unikernel project
    logs                    # (future) tail logs of a local run
```

Global flags (inherited):
- `-K, --kraftfile` — explicit Kraftfile path
- `--log-level`
- (later) `--context`, `--remote` for talking to a dvr daemon

Future top-level commands (after local loop is solid):
- `dvr daemon start`
- `dvr run --target <vm-id>` (dispatch to production runner)
- `dvr ps`, `dvr kill`, etc.

## Detailed Breakdown of Work (Small Implementable Steps)

### Phase 1 — Foundations (done in initial scaffold)
1. `go.mod` + module `github.com/cloud-shuttle/drover-runner`
2. Basic cobra root command (`cmd/dvr/main.go`) modeled on the sibling `drover` CLI
3. `dvr version` (shows dvr + best-effort kraft version)
4. `dvr unikernel` parent command group with clear Long description tying it to drun-001

### Phase 2 — Core Build Loop (in progress)
5. `dvr unikernel build [dir]`
   - Thin, high-fidelity wrapper around `kraft build`
   - Streams stdout + stderr live
   - Good error wrapping ("kraft build failed — see above")
   - Respects `--kraftfile`, `--target`, `--plat`, `--arch`
   - Sensible defaults (auto-detect Kraftfile)
6. `dvr unikernel run [dir|binary]`
   - Wrapper around `kraft run`
   - Default platform `qemu` for local macOS/Linux dev
   - Common flags: `--memory`, `--port`, `--detach`, `--target`
7. `dvr unikernel clean`
8. Basic `dvr unikernel targets` (later: parse Kraftfile YAML for nice table)

### Phase 3 — Polish & DX
9. Proper combined output + structured error types (so higher layers or drover can react)
10. Auto-detection of available targets from Kraftfile (parse YAML or call kraft with machine-readable output)
11. Better progress / emoji output consistent with drover style
12. Shell completion for targets (using kraft or our parser)
13. Unit tests for the wrapper (mock exec or use test binaries)
14. Update README + link from .beads/dr-001

### Phase 4 — Bridge to drun-002 and beyond
15. Add `--plat fc` friendly paths + metadata so the same build artifacts can be handed to the future Firecracker orchestrator
16. Produce machine-readable output (`--output json`) for the daemon to consume
17. Begin the Go library package (`pkg/unikernel/`) that the future `dvr-daemon` can import instead of shelling out

## Kraft CLI Surface We Rely On (researched 2026-05-23)

Relevant commands (kraft 0.12.11):
- `kraft build [-K Kraftfile] [-t target] [--plat qemu|fc] [dir]`
- `kraft run ...` (very rich: ports, volumes, memory, networks, `--plat`)
- `kraft clean`
- `kraft pkg`, `kraft menu`, etc. for power users

Kraftfile spec (v0.6) is well documented: https://unikraft.org/docs/cli/reference/kraftfile/latest

Key fields we will eventually parse:
- `targets: [{plat, arch, name, kconfig}]`
- `unikraft`, `libraries`, `rootfs`, `cmd`, `env`

## Non-Goals for drun-001
- Do not implement any unikernel build logic ourselves
- Do not talk to the production daemon yet
- Do not handle multi-tenancy or secure isolation (that's drun-003+)
- Keep the wrapper as thin and faithful as possible — we want to feel like "the nice way to drive kraft for Drover workloads"

## Open Questions / Decisions
- Binary name: `dvr` (short, matches the Beads notes "dvr --unikernel commands")
- Should we eventually vendor or re-export parts of kraft as a Go library? (probably not — keep kraft as the engine)
- Error model: wrap `exec.ExitError` and surface the exact kraft message + our context
- How much do we parse vs. delegate? (start with delegate + light parsing for `targets`)

## Current Implementation Status (as of this doc)

- [x] Module + cobra skeleton
- [x] `unikernel` command group + `build` / `run` / `clean` / `targets` stubs
- [x] Live stdout/stderr streaming + clean error wrapping
- [ ] Proper kraft version probing in `dvr version`
- [ ] YAML parsing of Kraftfile for `targets` listing
- [ ] Tests
- [ ] Documentation updates + Beads status bump

## How to Continue

Run:
```bash
go run ./cmd/dvr unikernel build .
# (will need a real Kraftfile project in the directory)
```

Next concrete tasks for a contributor:
1. Make `dvr version` actually call `kraft version` and print it
2. Implement a small `pkg/kraft` package that knows how to invoke kraft and parse its machine-readable output where available
3. Add a real `targets` lister that reads the Kraftfile and pretty-prints the possible `plat/arch` combinations

---

This design directly fulfills drun-001 and sets up a clean path into drun-002 (QEMU/KVM local slice launcher) and the later daemon work.
