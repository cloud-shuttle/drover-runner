---
title: "CLI Reference"
description: "Complete reference for all dvr CLI commands, subcommands, flags, and global options."
product: drover-code
audience: [member, platform-operator]
doc_type: reference
topics:
  - deployment
surface: repo-docs
---

# CLI Reference

`dvr` is the command-line interface for drover-runner. It provides a local unikernel development loop via the `unikernel` command group, and daemon lifecycle management via the `daemon` command group.

**Binary name**: `dvr`  
**Build from source**: `go build -o bin/dvr ./cmd/dvr`

---

## Global Flags

These flags apply to every `dvr` command.

| Flag | Short | Default | Description |
|---|---|---|---|
| `--kraftfile` | `-K` | `""` | Path to a specific `Kraftfile` (overrides auto-discovery from the project directory). |
| `--log-level` | `-l` | `info` | Log verbosity: `debug`, `info`, `warn`, or `error`. |
| `--help` | `-h` | — | Print help for the command. |
| `--version` | — | — | Print the `dvr` version string. |

---

## `dvr version`

Print the current `dvr` version and the installed `kraft` version.

```bash
dvr version
```

**Output:**

```
dvr version 0.1.0-dev
kraft: v0.12.11
```

If `kraft` is not installed:

```
dvr version 0.1.0-dev
kraft: not found or error (exec: "kraft": executable file not found in $PATH)
  Install from https://unikraft.org/docs/cli/install
```

---

## `dvr unikernel`

**Aliases**: `uk`, `kernel`

Parent command for local unikernel development. All subcommands delegate to the `kraft` CLI with sensible drover-runner defaults. Requires `kraft` to be installed.

---

### `dvr unikernel build [path]`

Build a Unikraft unikernel from a directory containing a `Kraftfile`.

```bash
dvr unikernel build [path] [flags]
```

**Arguments:**

| Argument | Default | Description |
|---|---|---|
| `path` | `.` (current directory) | Path to the project directory containing the `Kraftfile`. |

**Flags:**

| Flag | Short | Default | Description |
|---|---|---|---|
| `--target` | `-t` | `""` | Build a specific target (e.g. `qemu/x86_64`, `fc/x86_64`). |
| `--plat` | `-p` | `""` | Platform override: `qemu`, `fc`, `xen`. |
| `--arch` | `-m` | `""` | Architecture override: `x86_64`, `arm64`. |
| `--no-update` | — | `false` | Do not update the Unikraft package index before building. |
| `--no-prompt` | — | `false` | Run non-interactively; do not prompt for target selection. |

**Examples:**

```bash
# Build in the current directory (interactive target selection via kraft)
dvr unikernel build

# Build a specific subdirectory non-interactively
dvr unikernel build ./examples/helloworld --no-update --no-prompt

# Build for a specific platform/architecture target
dvr unikernel build -t qemu/x86_64

# Build for the Firecracker production path
dvr unikernel build --plat fc
```

**Output on success:**

```
🛠️  Building unikernel in /path/to/project
   Project: helloworld (spec v0.6)
   Declared targets: [qemu/x86_64 fc/x86_64]
   → kraft build [--no-update --no-prompt /path/to/project]

level=info msg="configuring helloworld (qemu/x86_64)"
level=info msg="build completed successfully" kernel=".unikraft/build/helloworld_qemu-x86_64"

✅ Build completed successfully.
   Use 'dvr unikernel run' (or 'dvr unikernel run -t <target>') to test it locally.
```

---

### `dvr unikernel run [path|binary]`

Launch a previously built unikernel using QEMU (default) or Firecracker.

```bash
dvr unikernel run [path] [flags]
```

**Arguments:**

| Argument | Default | Description |
|---|---|---|
| `path` | `.` (current directory) | Path to the project directory or built kernel binary. |

**Flags:**

| Flag | Short | Default | Description |
|---|---|---|---|
| `--target` | `-t` | `""` | Run a specific built target (e.g. `qemu/x86_64`). |
| `--plat` | `-p` | `qemu` | Platform: `qemu` (local dev, macOS safe) or `fc` (Firecracker, Linux only). |
| `--memory` | `-M` | `64Mi` | Memory to allocate to the VM (e.g. `64Mi`, `256Mi`, `1Gi`). |
| `--detach` | `-d` | `false` | Run in the background and return immediately. |
| `--no-prompt` | — | `false` | Run non-interactively. |

**Examples:**

```bash
# Run in the current directory with defaults (QEMU, 64Mi RAM, interactive)
dvr unikernel run

# Run a specific target with 128 MB RAM, non-interactively
dvr unikernel run -t qemu/x86_64 -M 128Mi --no-prompt

# Run in background (detached)
dvr unikernel run --detach

# Run on Firecracker (Linux only)
dvr unikernel run --plat fc -M 256Mi
```

**Signal handling:**

Press `Ctrl+C` (or send `SIGTERM`/`SIGINT`) to gracefully stop the running VM:

```
^C
🛑 Captured signal interrupt. Cleaning up virtual slice...
   ✅ Virtual slice terminated cleanly.
```

---

### `dvr unikernel targets [path]`

Parse the `Kraftfile` and list all declared build targets.

```bash
dvr unikernel targets [path]
```

**Arguments:**

| Argument | Default | Description |
|---|---|---|
| `path` | `.` (current directory) | Path to the project directory or `Kraftfile`. |

**Example:**

```bash
dvr unikernel targets ./examples/helloworld
```

**Output:**

```
📋 Targets for project "helloworld" (spec v0.6) in /path/to/examples/helloworld

   1. qemu/x86_64
   2. fc/x86_64

Use with:
  dvr unikernel build -t qemu/x86_64 /path/to/examples/helloworld
  dvr unikernel run   -t qemu/x86_64 /path/to/examples/helloworld
```

---

### `dvr unikernel clean [path]`

Remove all build artefacts for a unikernel project.

```bash
dvr unikernel clean [path]
```

**Arguments:**

| Argument | Default | Description |
|---|---|---|
| `path` | `.` (current directory) | Path to the project directory. |

**Example:**

```bash
dvr unikernel clean ./examples/helloworld
```

**Output:**

```
🧹 Cleaning /path/to/examples/helloworld...
✅ Clean complete.
```

---

## `dvr daemon`

Parent command for daemon lifecycle management.

---

### `dvr daemon start`

Start the drover-runner host hypervisor daemon.

```bash
dvr daemon start [flags]
```

**Flags:**

| Flag | Short | Default | Description |
|---|---|---|---|
| `--port` | `-p` | `8080` | TCP port for the HTTP REST API to listen on. |
| `--token` | `-t` | `dvr-admin-token` | Bearer token for REST API authentication. **Always override this in production.** |
| `--mode` | `-m` | `qemu` | Hypervisor backend: `qemu` (macOS/dev) or `firecracker` (Linux/production). |
| `--kernel` | `-k` | `""` | Path to the Linux kernel image (vmlinux). Required when `--mode firecracker`. |
| `--telemetry` | — | `10` | Background telemetry poll interval in seconds. |

**Examples:**

```bash
# Start in QEMU mode for local development (macOS):
dvr daemon start --port 8080 --token my-dev-token

# Start in Firecracker mode for production Linux:
sudo dvr daemon start \
  --port 8099 \
  --token "${DVR_ADMIN_TOKEN}" \
  --mode firecracker \
  --kernel /var/lib/dvr/kernels/vmlinux
```

**Startup output:**

```
🚀 Drover Runner Daemon Booting...
   Mode:      firecracker
   Port:      8099
   Token Auth: <your-token>
   [net] Creating host bridge link dvr-br0...
   HTTP REST API active on http://localhost:8099
```

**Shutdown:**

Send `SIGINT` (Ctrl+C) or `SIGTERM` to trigger graceful shutdown. The daemon will:
1. Stop the telemetry collector.
2. Call `StopAll()` to terminate every running Guest VM and execute the full secure teardown pipeline (shredding, auditing, TAP release) for each.
3. Shut down the HTTP server within a 5-second timeout.

```
^C
🛑 Received signal interrupt. Initiating graceful shutdown...
   [daemon] Stopping instance inst-1...
   [net] Releasing host TAP device dvr-t-inst-1...
   [shred] Purged 134217728 bytes in 38ms | hash=e3b0c44298fc...
   ✅ Daemon HTTP server stopped cleanly.
```

---

## Exit Codes

| Code | Meaning |
|---|---|
| `0` | Success |
| `1` | Command error (invalid flags, kraft failure, daemon start failure, etc.) |

---

## Environment Variables

`dvr` does not currently read configuration from environment variables directly. Pass all configuration via CLI flags. In scripts, use shell variable substitution:

```bash
dvr daemon start --token "${DVR_ADMIN_TOKEN}" --port "${DVR_PORT:-8080}"
```

---

## See Also

- [REST API Reference](./rest-api.md)
- [Tutorial: Build and run your first unikernel](../tutorials/first-unikernel.md)
- [How-to: Deploy the daemon in production](../how-to/deploy-hypervisor-daemon.md)
