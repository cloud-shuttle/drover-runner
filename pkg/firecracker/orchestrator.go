package firecracker

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"syscall"
	"time"
)

// FirecrackerOrchestrator programmatically configures and boots secure guest microVM slices.
type FirecrackerOrchestrator struct {
	SocketPath string
	BinPath    string
	Stdout     io.Writer
	Stderr     io.Writer
	cmd        *exec.Cmd
	client     *http.Client
}

// NewOrchestrator returns a programmatically managed Firecracker Guest VM orchestrator.
func NewOrchestrator(socketPath, binPath string) *FirecrackerOrchestrator {
	if binPath == "" {
		binPath = "firecracker"
	}
	
	// Construct a standard HTTP client dialing Unix domain sockets
	client := &http.Client{
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
				var dialer net.Dialer
				return dialer.DialContext(ctx, "unix", socketPath)
			},
		},
	}

	return &FirecrackerOrchestrator{
		SocketPath: socketPath,
		BinPath:    binPath,
		client:     client,
	}
}

// StartProcess spawns the background firecracker microVM process bound to the Unix socket.
func (o *FirecrackerOrchestrator) StartProcess(ctx context.Context) error {
	// Clean old socket file if left over
	_ = os.Remove(o.SocketPath)

	o.cmd = exec.CommandContext(ctx, o.BinPath, "--api-sock", o.SocketPath)
	o.cmd.Stdout = o.Stdout
	o.cmd.Stderr = o.Stderr
	o.cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
	}

	if err := o.cmd.Start(); err != nil {
		return fmt.Errorf("failed to start firecracker binary %q: %w", o.BinPath, err)
	}

	// Wait for Unix Domain Socket to become active (up to 2 seconds)
	for i := 0; i < 20; i++ {
		if _, err := os.Stat(o.SocketPath); err == nil {
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}

	_ = o.Stop()
	return fmt.Errorf("timeout waiting for firecracker UDS API socket %q to start", o.SocketPath)
}

// Configure sends JSON payloads to the REST UDS socket to declare CPU, RAM, and virtual disks.
func (o *FirecrackerOrchestrator) Configure(ctx context.Context, kernelPath, rootfsPath string, vcpus, memoryMB int) error {
	// 1. Configure vCPUs and RAM
	machineConfig := map[string]interface{}{
		"vcpu_count":   vcpus,
		"mem_size_mib": memoryMB,
	}
	if err := o.putJSON(ctx, "/machine-config", machineConfig); err != nil {
		return fmt.Errorf("machine config failed: %w", err)
	}

	// 2. Configure Boot Source & Kernel arguments
	bootSource := map[string]interface{}{
		"kernel_image_path": kernelPath,
		"boot_args":         "console=ttyS0 reboot=k panic=1 pci=off",
	}
	if err := o.putJSON(ctx, "/boot-source", bootSource); err != nil {
		return fmt.Errorf("boot source config failed: %w", err)
	}

	// 3. Configure Root Drive overlay
	drive := map[string]interface{}{
		"drive_id":       "rootfs",
		"path_on_host":   rootfsPath,
		"is_root_device": true,
		"is_read_only":   false,
	}
	if err := o.putJSON(ctx, "/drives/rootfs", drive); err != nil {
		return fmt.Errorf("drive config failed: %w", err)
	}

	return nil
}

// ConfigureNetwork links Firecracker's virtual Guest eth0 to a host TAP network interface.
func (o *FirecrackerOrchestrator) ConfigureNetwork(ctx context.Context, hostDevName, macAddr string) error {
	netConfig := map[string]interface{}{
		"iface_id":      "eth0",
		"host_dev_name": hostDevName,
		"guest_mac":     macAddr,
	}
	if err := o.putJSON(ctx, "/network-interfaces/eth0", netConfig); err != nil {
		return fmt.Errorf("network interface config failed: %w", err)
	}
	return nil
}

// StartVM fires the InstanceStart trigger to execute virtualized OS execution inside the Guest VM.
func (o *FirecrackerOrchestrator) StartVM(ctx context.Context) error {
	actionPayload := map[string]interface{}{
		"action_type": "InstanceStart",
	}
	if err := o.putJSON(ctx, "/actions", actionPayload); err != nil {
		return fmt.Errorf("InstanceStart command failed: %w", err)
	}
	return nil
}

// Stop gracefully cancels and kills the firecracker microVM process.
func (o *FirecrackerOrchestrator) Stop() error {
	if o.cmd == nil || o.cmd.Process == nil {
		return nil
	}

	// Graceful terminate
	_ = o.cmd.Process.Signal(syscall.SIGTERM)

	done := make(chan error, 1)
	go func() {
		done <- o.cmd.Wait()
	}()

	select {
	case <-done:
		// Process terminated
	case <-time.After(1 * time.Second):
		// Force kill
		_ = o.cmd.Process.Kill()
	}

	_ = os.Remove(o.SocketPath)
	return nil
}

func (o *FirecrackerOrchestrator) putJSON(ctx context.Context, path string, data interface{}) error {
	bodyBytes, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("marshal JSON failed: %w", err)
	}

	url := fmt.Sprintf("http://localhost%s", path)
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, url, bytes.NewReader(bodyBytes))
	if err != nil {
		return fmt.Errorf("create HTTP request failed: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := o.client.Do(req)
	if err != nil {
		return fmt.Errorf("UDS request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
		var errResp map[string]interface{}
		_ = json.NewDecoder(resp.Body).Decode(&errResp)
		return fmt.Errorf("bad status code: %d, error: %v", resp.StatusCode, errResp)
	}

	return nil
}
