---
title: "Architecture & Security Model"
description: "Conceptual model of drover-runner's dual-hypervisor design, zero-trust network isolation, and ephemeral storage sanitization guarantees."
product: drover-code
audience: [member, platform-operator]
doc_type: explanation
topics:
  - deployment
  - security
  - tenancy
surface: repo-docs
---

# Architecture & Security Model

This document explains *why* drover-runner is designed the way it is: the layered structure, the reasoning behind key decisions, and the security properties that those decisions guarantee. It is not a step-by-step guide; for that, see the [Tutorial](../tutorials/first-unikernel.md) or the [How-to guides](../how-to/).

---

## The Central Problem: Multi-Tenant VM Isolation

A unikernel hypervisor daemon has one fundamental responsibility beyond running virtual machines: ensuring that **no artefact of one tenant's workload can ever reach another tenant**. This encompasses three distinct attack surfaces:

1. **Compute** — preventing Guest VM processes from escaping the hypervisor boundary.
2. **Network** — preventing Guest VM traffic from reaching other tenants on the same host.
3. **Storage** — preventing data remnants from surviving a VM termination and being accessible by the next workload allocated to that disk.

drover-runner addresses each of these with a dedicated subsystem. Understanding how they fit together is the purpose of this document.

---

## Layer 1: The HypervisorDriver Abstraction

The entire execution plane in drover-runner is expressed through a single Go interface:

```go
// pkg/daemon/driver.go
type HypervisorDriver interface {
    LaunchVM(ctx context.Context, id string, imageName string, memoryMB int, env map[string]string) (*ActiveVM, error)
}
```

This interface has two concrete implementations that serve different environments:

| Implementation | When Used | Underlying Engine |
|---|---|---|
| `QemuDriver` | macOS / local development | `kraft` CLI wrapping QEMU emulation |
| `FirecrackerDriver` | Linux bare-metal / production | Firecracker microVM via Unix Domain Socket |

### Why two drivers?

Firecracker is a production-grade hypervisor built by AWS for Lambda and Fargate. It provides **sub-second VM boot times** and a drastically reduced attack surface compared to QEMU (it has no BIOS, no PCI bus emulation, no USB subsystem). However, Firecracker only runs on Linux with KVM available — it cannot run on macOS.

QEMU, by contrast, runs in user-space emulation on any platform, including macOS, making it practical for inner-loop development. The `QemuDriver` delegates to the `kraft` CLI, which handles the build toolchain and target manifests for Unikraft OS images.

The `HypervisorDriver` interface is the **seam** that lets the same daemon binary and REST API serve both contexts without any conditional logic bleeding into the HTTP layer.

---

## Layer 2: The Daemon REST API

The daemon is a lightweight, single-binary HTTP server (`pkg/daemon/server.go`) that exposes a minimal REST interface over a configurable port. **All endpoints require Bearer token authentication.**

```
POST   /v1/instances          → Boot a new Guest VM slice
GET    /v1/instances/{id}     → Inspect a running instance
GET    /v1/instances/{id}/logs → Stream instance stdout/stderr
DELETE /v1/instances/{id}     → Terminate and sanitize an instance
```

Internally, the server stores a `sync.Map` of `*ActiveVM` handles, each of which carries a `Stop()` closure bound to the specific hypervisor process that created it. This means the teardown logic (network release, disk shredding, audit compilation) is always encapsulated with the instance, and the HTTP handler remains trivially simple.

### Concurrency model

Each `LaunchVM` call runs synchronously on the HTTP request goroutine. Log streaming uses a `safeBufferWriter` (a `sync.Mutex`-guarded `bytes.Buffer`) that is safe for concurrent writes by the VM process's stdout/stderr forwarding goroutine.

---

## Layer 3: The Network Isolation Plane

When a Firecracker microVM boots, it needs a virtual network interface. drover-runner manages the complete lifecycle of host-level Linux TAP devices through the `NetworkManager` interface (`pkg/network/network.go`):

```go
type NetworkManager interface {
    SetupBridge(ctx context.Context) error
    AllocateTAP(ctx context.Context, instanceID string) (tapName string, macAddr string, err error)
    ReleaseTAP(ctx context.Context, instanceID string) error
}
```

### The topology

```
Guest VM (microVM)
      │
      │  virtio-net (inside VM kernel)
      ▼
  TAP device (dvr-t-{id})        ← per-instance, host kernel object
      │
      │  enslaved via ip link set master
      ▼
  Bridge (dvr-br0)               ← single shared bridge on host
      │
      │  iptables MASQUERADE NAT
      ▼
   Host NIC (eth0, upstream)
```

Each Guest VM gets its own TAP device with a **deterministically generated, locally-administered MAC address** (derived from `SHA-256(instanceID)`). This makes the identity of each device auditable and repeatable without a lease database.

### Tenant isolation by default

The iptables ruleset enforces a strict **deny-by-default inter-tenant policy** on the bridge:

```
# Outbound NAT: all guest traffic masquerades behind the host's public IP
iptables -t nat -A POSTROUTING -s 172.20.0.0/16 ! -o dvr-br0 -j MASQUERADE

# Allow guests to reach external networks
iptables -A FORWARD -i dvr-br0 ! -o dvr-br0 -j ACCEPT

# Allow return traffic for established connections
iptables -A FORWARD -o dvr-br0 -m conntrack --ctstate RELATED,ESTABLISHED -j ACCEPT

# Block guest-to-guest traffic on the same bridge
iptables -A FORWARD -i dvr-br0 -o dvr-br0 -j DROP
```

The critical rule is the last one: **traffic entering and leaving the same bridge interface is unconditionally dropped**. Two tenants on the same physical host cannot communicate with each other at the network layer.

On macOS (local development), the `mockNetworkManager` implementation produces the same interface contract and log output, but skips all `ip` and `iptables` system calls, allowing the same code to run without root privileges.

---

## Layer 4: Ephemeral Disk Sanitization

Each Guest VM receives its own **ephemeral overlay disk**: a private copy of the base rootfs image written to a temporary path (`/tmp/dvr-{fc,}-overlay-{id}.ext4`). This copy-on-write approach means:

- Mutations inside the VM are local to that overlay and cannot affect the base image.
- Multiple VMs can share the same base image safely.

However, creating a private overlay is only half of the isolation story. When the VM terminates, a naive `os.Remove()` would unlink the directory entry but leave the raw bytes physically on the NVMe or SSD blocks until the filesystem happens to reuse them. This is a **data remnant** risk: a subsequent tenant whose overlay is allocated to the same physical blocks could, in pathological circumstances, recover those bytes.

### The ShredFile pipeline

`pkg/storage/purge.go` implements a deterministic six-step sanitization:

```
1. Open the overlay file with O_RDWR exclusive access
2. Stat() the exact byte size
3. Overwrite every byte with 0x00 in 64 KB chunks (respecting context cancellation)
4. Call file.Sync() → triggers fdatasync syscall → forces OS buffer flush to physical media
5. Seek back to offset 0; read a 1 KB verification sample
6. Compute SHA-256(sample) → hash of all-zeros is deterministic and verifiable
7. os.Remove() to unlink the now-zeroed file pointer
```

The resulting `ShredRecord` carries the bytes purged, the duration, and the `sanitized_hash`. Because a zeroed file always produces the same SHA-256, any monitoring system can **programmatically assert** that the shred succeeded without re-reading the deleted file.

### Cryptographic destruction audits

After each shred, a `DestructionAudit` record is compiled and dispatched:

```go
type DestructionAudit struct {
    InstanceID    string      // VM identity
    UptimeSeconds float64     // Actual runtime before termination
    AllocatedRAM  int         // Memory limit in MB
    OverlayPath   string      // Path that was shredded
    ShredStats    ShredRecord // Cryptographic proof of sanitization
    Timestamp     time.Time   // Wall-clock of teardown
}
```

These records are queued for batch dispatch to ClickHouse via `pkg/daemon/telemetry.go`, providing a persistent, queryable audit trail of every storage destruction event across the fleet.

---

## The Full Instance Lifecycle

```
POST /v1/instances
         │
         ▼
  [1] Generate instance ID (inst-N)
         │
         ▼
  [2] SetupBridge (idempotent — no-op if bridge exists)
         │
         ▼
  [3] AllocateTAP → dvr-t-{id} linked to dvr-br0
         │
         ▼
  [4] Copy base rootfs → /tmp/dvr-fc-overlay-{id}.ext4
         │
         ▼
  [5] Start firecracker process (UDS at /tmp/dvr-fc-{id}.socket)
         │
         ▼
  [6] Configure VM: vCPUs, memory, kernel boot args, overlay disk
         │
         ▼
  [7] ConfigureNetwork: bind virtio-net to TAP device
         │
         ▼
  [8] BootVM → microVM is live
         │
         ▼
  201 Created {id, state: "running", ip: "172.20.0.x"}


DELETE /v1/instances/{id}
         │
         ▼
  [1] orchestrator.Stop() → SIGTERM firecracker process
         │
         ▼
  [2] storage.ShredFile() → zero-overwrite + fdatasync + verify
         │
         ▼
  [3] storage.RegisterAudit() → queue DestructionAudit to ClickHouse
         │
         ▼
  [4] netMgr.ReleaseTAP() → ip link delete dvr-t-{id}
         │
         ▼
  204 No Content
```

Every step in this lifecycle is reversible on failure: if any stage errors, the preceding allocations are explicitly released. This prevents dangling TAP devices or leaked overlay files from accumulating on the host.

---

## Design Decisions & Trade-offs

### Why Unikraft OS images?

Unikernels compile application logic directly into the OS image, producing a single bootable binary with no shell, no package manager, and no unnecessary kernel modules. The attack surface is orders of magnitude smaller than a conventional container. This aligns with Drover's zero-trust execution model.

### Why a flat REST API rather than gRPC?

The daemon is intentionally simple. A flat JSON REST API can be inspected with `curl`, driven by any language's HTTP client, and integrated into shell scripts without a generated stub. The daemon's role is coordination, not performance-critical data transport.

### Why not use Linux namespaces/cgroups directly (i.e., containers)?

Firecracker microVMs provide **hardware-enforced VM boundaries** via KVM, whereas containers share the host kernel and are protected only by software-enforced namespace separation. The drover-runner model prioritizes the stronger isolation guarantee for multi-tenant workloads.

---

## Further Reading

- [Tutorial: Build and run your first unikernel](../tutorials/first-unikernel.md)
- [How-to: Deploy the hypervisor daemon in production](../how-to/deploy-hypervisor-daemon.md)
- [Reference: REST API](../reference/rest-api.md)
- [Reference: CLI](../reference/cli.md)
