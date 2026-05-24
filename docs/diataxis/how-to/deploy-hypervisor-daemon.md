---
title: "Deploy the Hypervisor Daemon in Production"
description: "Step-by-step guide to deploying the dvr daemon on a Linux bare-metal host with Firecracker, KVM, and production networking."
product: drover-code
audience: [platform-operator]
doc_type: how-to
topics:
  - deployment
  - security
  - tenancy
surface: repo-docs
---

# How-to: Deploy the Hypervisor Daemon in Production

This guide explains how to deploy the `dvr` daemon on a **Linux bare-metal host** using the Firecracker hypervisor. Follow these steps when you are ready to run multi-tenant Guest VM workloads in a production or staging environment.

> **Assumes**: You have completed the [Tutorial: Build and run your first unikernel](../tutorials/first-unikernel.md) and understand the daemon REST API. For background on the isolation model, read the [Architecture & Security explanation](../explanation/architecture-and-security.md).

---

## Prerequisites

| Requirement | Minimum Version | Notes |
|---|---|---|
| Linux kernel | 5.10+ | Required for KVM and `ip tuntap` support |
| KVM module | loaded | `lsmod \| grep kvm` must return output |
| Firecracker binary | v1.4+ | Download from [github.com/firecracker-microvm/firecracker/releases](https://github.com/firecracker-microvm/firecracker/releases) |
| Go toolchain | 1.21+ | For building `dvr` from source |
| `iproute2` / `iptables` | — | Standard on most distros; verify with `ip --version` |
| Root / `CAP_NET_ADMIN` | — | Required for TAP device allocation and iptables rules |

---

## Step 1: Verify KVM Access

```bash
ls -la /dev/kvm
```

Expected output:

```
crw-rw---- 1 root kvm 10, 232 May 24 09:00 /dev/kvm
```

If `/dev/kvm` is missing, enable virtualisation in your server BIOS/UEFI and then load the module:

```bash
# For Intel hosts:
sudo modprobe kvm_intel

# For AMD hosts:
sudo modprobe kvm_amd
```

Add the daemon's service user to the `kvm` group so it can open `/dev/kvm` without running the entire process as root:

```bash
sudo usermod -aG kvm dvr-daemon
```

---

## Step 2: Install the Firecracker Binary

Download the latest stable release and install it into the system path:

```bash
FIRECRACKER_VERSION=v1.7.0

curl -fsSL \
  "https://github.com/firecracker-microvm/firecracker/releases/download/${FIRECRACKER_VERSION}/firecracker-${FIRECRACKER_VERSION}-x86_64.tgz" \
  -o /tmp/firecracker.tgz

tar -xzf /tmp/firecracker.tgz -C /tmp

sudo install -o root -g root -m 0755 \
  /tmp/release-${FIRECRACKER_VERSION}-x86_64/firecracker-${FIRECRACKER_VERSION}-x86_64 \
  /usr/local/bin/firecracker

firecracker --version
```

---

## Step 3: Build and Install the dvr Binary

On the Linux host (or cross-compile from macOS with `GOOS=linux`):

```bash
# From the drover-runner repository root:
go build -o bin/dvr ./cmd/dvr

# Install system-wide:
sudo install -o root -g root -m 0755 bin/dvr /usr/local/bin/dvr

dvr version
```

---

## Step 4: Prepare the Kernel and Rootfs Images

Firecracker requires a Linux kernel binary and a rootfs ext4 image. You can use Unikraft pre-built images or supply your own:

```bash
# Create the library directory:
sudo mkdir -p /var/lib/dvr/kernels
sudo mkdir -p /var/lib/dvr/images

# Example: copy your compiled Unikraft kernel:
sudo cp .unikraft/build/helloworld_fc-x86_64 /var/lib/dvr/kernels/vmlinux

# Example: copy your compiled Unikraft rootfs:
sudo cp .unikraft/build/helloworld_fc-x86_64.ext4 /var/lib/dvr/images/helloworld.ext4
```

> The daemon creates a **private ephemeral overlay copy** of the rootfs for each instance, so the base image at `/var/lib/dvr/images/` is never mutated.

---

## Step 5: Enable IP Forwarding

The TAP bridge requires IP forwarding to be enabled on the host:

```bash
# Enable immediately:
sudo sysctl -w net.ipv4.ip_forward=1

# Make permanent across reboots:
echo "net.ipv4.ip_forward = 1" | sudo tee -a /etc/sysctl.d/99-dvr.conf
sudo sysctl --system
```

---

## Step 6: Start the Daemon

Run the daemon with the `firecracker` hypervisor mode:

```bash
sudo dvr daemon start \
  --port 8099 \
  --token "${DVR_ADMIN_TOKEN}" \
  --mode firecracker \
  --kernel /var/lib/dvr/kernels/vmlinux
```

**Flag reference:**

| Flag | Default | Description |
|---|---|---|
| `--port` | `8080` | TCP port to listen on |
| `--token` | *(required)* | Bearer token for REST API authentication |
| `--mode` | `qemu` | Hypervisor backend: `qemu` or `firecracker` |
| `--kernel` | `""` | Path to the vmlinux kernel image (Firecracker mode only) |

You should see output similar to:

```
   [net] Creating host bridge link dvr-br0...
🚀 dvr daemon listening on :8099 [firecracker mode]
```

---

## Step 7: Verify the Bridge and iptables Rules

In a separate terminal, verify that the host network bridge was created correctly:

```bash
ip link show dvr-br0
```

```
5: dvr-br0: <BROADCAST,MULTICAST,UP,LOWER_UP> mtu 1500 qdisc noqueue state UP mode DEFAULT
    link/ether 76:f4:11:... brd ff:ff:ff:ff:ff:ff
```

Verify the tenant-isolation iptables rules are active:

```bash
sudo iptables -L FORWARD -n --line-numbers
```

You should see a `DROP` rule for `dvr-br0 → dvr-br0` traffic (the guest-to-guest isolation rule):

```
...
N  DROP   all  --  dvr-br0  dvr-br0  0.0.0.0/0  0.0.0.0/0
```

---

## Step 8: Spawn a Test Instance

With the daemon running, use `curl` to launch a Guest VM:

```bash
curl -s -X POST http://localhost:8099/v1/instances \
  -H "Authorization: Bearer ${DVR_ADMIN_TOKEN}" \
  -H "Content-Type: application/json" \
  -d '{
    "image_name": "/var/lib/dvr/images/helloworld.ext4",
    "memory_mb": 128,
    "env": {}
  }'
```

Expected response:

```json
{
  "id": "inst-1",
  "state": "running",
  "ip": "172.20.0.2"
}
```

---

## Step 9: Inspect Logs and Terminate Cleanly

Stream the instance's console output:

```bash
curl -s http://localhost:8099/v1/instances/inst-1/logs \
  -H "Authorization: Bearer ${DVR_ADMIN_TOKEN}"
```

Terminate the instance and trigger the secure purge pipeline:

```bash
curl -s -X DELETE http://localhost:8099/v1/instances/inst-1 \
  -H "Authorization: Bearer ${DVR_ADMIN_TOKEN}"
```

On the daemon console you will see the full teardown sequence:

```
   [net] Releasing host TAP device dvr-t-inst-1...
   [shred] Purged 134217728 bytes in 42ms | hash=e3b0c44298fc...
   [audit] DestructionAudit queued: inst-1, uptime=12.4s, ram=128MB
```

---

## Running as a systemd Service

For unattended production operation, install the daemon as a systemd unit:

```ini
# /etc/systemd/system/dvr-daemon.service

[Unit]
Description=Drover Runner Hypervisor Daemon
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
User=root
ExecStart=/usr/local/bin/dvr daemon start \
    --port 8099 \
    --token ${DVR_ADMIN_TOKEN} \
    --mode firecracker \
    --kernel /var/lib/dvr/kernels/vmlinux
Restart=on-failure
RestartSec=5s

# Hard resource limits
LimitNOFILE=65536
LimitNPROC=8192

[Install]
WantedBy=multi-user.target
```

Enable and start:

```bash
sudo systemctl daemon-reload
sudo systemctl enable --now dvr-daemon
sudo systemctl status dvr-daemon
```

---

## Troubleshooting

### `failed to start firecracker orchestrator process`

- Verify `firecracker` is in `$PATH`: `which firecracker`
- Verify `/dev/kvm` is accessible by the daemon's user
- Check that the UDS socket path is writable: `/tmp/dvr-fc-{id}.socket` requires write access to `/tmp`

### `failed to create bridge` or `iptables: Permission denied`

The daemon requires root or `CAP_NET_ADMIN` for bridge and iptables management. Either run as root or grant the specific capability to the binary:

```bash
sudo setcap cap_net_admin+ep /usr/local/bin/dvr
```

### `failed to allocate unique instance overlay disk`

Verify the base image path passed to `image_name` exists and is readable. The overlay is copied to `/tmp/` — ensure the host has adequate free space there.

### Guest VM cannot reach the internet

1. Confirm IP forwarding is enabled: `sysctl net.ipv4.ip_forward` → must be `1`
2. Confirm the MASQUERADE rule is present: `sudo iptables -t nat -L POSTROUTING -n`
3. Check the host's default route: `ip route show default`

---

## Next Steps

- [Reference: REST API](../reference/rest-api.md) — full endpoint specification
- [Reference: CLI](../reference/cli.md) — all `dvr` CLI flags
- [Explanation: Architecture & Security](../explanation/architecture-and-security.md) — understand the isolation guarantees
