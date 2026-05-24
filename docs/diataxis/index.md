---
title: "drover-runner Documentation"
description: "Entry point for all drover-runner documentation, organised by the Diátaxis framework."
product: drover-code
audience: [member, platform-operator, evaluator]
doc_type: explanation
surface: repo-docs
---

# drover-runner Documentation

`drover-runner` is the secure multi-tenant unikernel hypervisor for the Drover platform. It boots isolated Unikraft Guest OS images on bare-metal Linux using Firecracker microVMs, with cryptographic teardown guarantees and iptables-enforced tenant network isolation.

Documentation here follows the **[Diátaxis](https://diataxis.fr)** framework — four modes, each serving a different user need. Find what you need by asking what you are trying to do:

---

## I want to **learn** by doing

→ **[Tutorial: Build and run your first unikernel](./tutorials/first-unikernel.md)**

Start here. Takes ≈10 minutes. You will compile the `dvr` CLI, build a Unikraft HelloWorld image, and boot it locally in QEMU. No prior unikernel experience required.

---

## I want to **accomplish a specific task**

→ **[How-to: Deploy the hypervisor daemon in production](./how-to/deploy-hypervisor-daemon.md)**

For platform operators who need to run the `dvr` daemon on a Linux bare-metal host with Firecracker, KVM, bridge networking, and systemd supervision.

---

## I want to **understand** how it works

→ **[Explanation: Architecture & Security Model](./explanation/architecture-and-security.md)**

Deep-dive into the dual-hypervisor design (`QemuDriver` vs `FirecrackerDriver`), the TAP/bridge network isolation topology, the zero-trust ephemeral disk shredding pipeline, and why each design decision was made.

---

## I need **precise technical information**

→ **[Reference: REST API](./reference/rest-api.md)**  
→ **[Reference: CLI](./reference/cli.md)**

Authoritative specifications for every REST endpoint (schemas, status codes, authentication) and every `dvr` CLI command (flags, examples, exit codes).

---

## Packages

The core implementation lives in `pkg/`:

| Package | Purpose |
|---|---|
| `pkg/daemon` | HTTP server, `HypervisorDriver` interface, `Config`, graceful shutdown |
| `pkg/kraft` | `kraft` CLI wrapper for local QEMU development |
| `pkg/firecracker` | Firecracker UDS orchestrator (process management, REST-over-socket) |
| `pkg/network` | Linux TAP/bridge lifecycle + iptables tenant isolation rules |
| `pkg/storage` | Secure file shredding (`ShredFile`) and cryptographic destruction audits |
