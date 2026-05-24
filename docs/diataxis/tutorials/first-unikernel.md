# Tutorial: Build and Run Your First Unikernel Locally

Welcome to the `dvr` unikernel local developer loop! This tutorial will guide you step-by-step from an empty folder to running your first Hello World Guest OS unikernel locally on your machine using QEMU emulation.

---

### Prerequisites & Requirements
Before you begin, you will need:
* A macOS or Linux developer machine.
* Go 1.21 or higher installed on your system.
* The `kraft` CLI engine installed. If you do not have it, install it by following the [Official Unikraft CLI Guide](https://unikraft.org/docs/cli/install).

* **Time Estimate**: 10 minutes.
* **What you will learn**: How to build the `dvr` CLI, parse a project configuration, compile a Guest OS unikernel locally, and launch a lightweight virtual emulated slice.

---

### Step 1: Build the dvr CLI Tool
First, let's compile our CLI runner from source inside the repository.

1. Open your terminal.
2. Navigate to the root directory of the `drover-runner` workspace.
3. Run the compile command:
   ```bash
   go build -o bin/dvr ./cmd/dvr
   ```
4. Verify the CLI compiled successfully by checking the version:
   ```bash
   ./bin/dvr version
   ```
   *Expected output:*
   ```text
   dvr version 0.1.0-dev
   kraft: v0.12.11
   ```

---

### Step 2: Explore the HelloWorld Project Structure
For this tutorial, we will use the minimal HelloWorld project in the repository. Let's inspect its structure.

1. Navigate to the `examples/helloworld` directory:
   ```bash
   cd examples/helloworld
   ```
2. Open and read the `Kraftfile`:
   ```bash
   cat Kraftfile
   ```
   *Notice the target specifications:*
   ```yaml
   spec: v0.6
   name: helloworld
   unikraft: stable
   targets:
     - qemu/x86_64
     - fc/x86_64
   ```
   This configuration declares that the application compiles to run under QEMU emulation (local development) and Firecracker (bare-metal production).

---

### Step 3: Discover Compile Targets via dvr
Let's ask the `dvr` tool to discover and list what targets our project can compile into.

1. Run the `targets` subcommand inside the project directory:
   ```bash
   ../../bin/dvr unikernel targets .
   ```
   *Expected output:*
   ```text
   📋 Targets for project "helloworld" (spec v0.6) in /path/to/examples/helloworld

     1. qemu/x86_64
     2. fc/x86_64
   ```

---

### Step 4: Compile the Guest OS image
Now, let's compile the project to produce a bootable Guest OS image for QEMU.

1. Execute the build command:
   ```bash
   ../../bin/dvr unikernel build . --no-update --no-prompt
   ```
   *Expected output:*
   ```text
   🛠️  Building unikernel in /path/to/examples/helloworld
      Project: helloworld (spec v0.6)
      Declared targets: [qemu/x86_64 fc/x86_64]
      → kraft build [--no-update --no-prompt /path/to/examples/helloworld]

   level=info msg="configuring helloworld (qemu/x86_64)"
   level=info msg="build completed successfully" kernel=".unikraft/build/helloworld_qemu-x86_64"

   ✅ Build completed successfully.
   ```

---

### Step 5: Launch the Guest VM Slice
With the Guest OS compiled, we are ready to boot the virtual machine locally.

1. Execute the run command, specifying the QEMU platform target:
   ```bash
   ../../bin/dvr unikernel run . -t qemu/x86_64 --no-prompt
   ```
   *Expected output:*
   ```text
   🚀 Running unikernel from /path/to/examples/helloworld
   level=warning msg="using hardware emulation"
   level=info msg=using arch="x86_64" plat=qemu
   o.   .o       _ _               __ _
   Oo   Oo  ___ (_) | __ __  __ _ ' _) :_
   oO   oO ' _ `| | |/ /  _)' _` | |_|  _)
   oOo oOO| | | | |   (| | | (_) |  _) :_
    OoOoO ._, ._:_:_,\_._,  .__,_:_, \___)
                     Ijiraq 0.21.0~d1c2fac
   weak main() called. Symbol was not replaced!
      ✅ Unikernel virtual slice completed successfully.
   ```

Congratulations! You have successfully built the `dvr` client tool, compiled a Unikraft Guest OS image, and booted a virtual machine slice on your machine.

---

### Next Steps
* To deploy and host this Guest VM inside the production multi-tenant daemon, check out the [How-to Guide: Deploy the Hypervisor Daemon](../how-to/deploy-hypervisor-daemon.md).
* To understand the underlying virtualization boundaries and zero-trust sanitization engines, read the [Explanation: Architectural Design and Security](../explanation/architecture-and-security.md).
