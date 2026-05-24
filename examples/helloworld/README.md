# examples/helloworld

Minimal Unikraft "Hello World" project used for end-to-end testing of the `dvr` CLI.

## Usage with dvr (local developer loop)

```bash
# From the root of drover-runner
cd examples/helloworld

# 1. Discover what targets are available (uses our Kraftfile parser)
dvr unikernel targets .

# 2. Build for the local QEMU target (drun-001 / drun-002)
dvr unikernel build .

# 3. Run it locally
dvr unikernel run -t qemu/x86_64 .

# Or let it pick the first declared target
dvr unikernel run .
```

First build will download the Unikraft core and base libraries (can take 1-4 minutes depending on your connection). Subsequent invocations are fast because kraft caches everything under `~/.kraft` (or the equivalent on your OS).

## Why this example?

- Declares both `qemu/x86_64` (local dev) and `fc/x86_64` (what the future production Firecracker hypervisor daemon will use).
- Exercises `ParseKraftfile`, target listing, build, and run paths in the `pkg/kraft` + `cmd/dvr` integration.
- Corresponds directly to the acceptance criteria of drun-001.

## Clean up

```bash
dvr unikernel clean .
```

## Related

See the main design document: [../../docs/drun-001-kraft-integration-design.md](../../docs/drun-001-kraft-integration-design.md)
