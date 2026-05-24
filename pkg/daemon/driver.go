package daemon

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/cloud-shuttle/drover-runner/pkg/firecracker"
	"github.com/cloud-shuttle/drover-runner/pkg/kraft"
	"github.com/cloud-shuttle/drover-runner/pkg/network"
	"github.com/cloud-shuttle/drover-runner/pkg/storage"
)

// ActiveVM represents the identity and control handle of a running Guest VM.
type ActiveVM struct {
	ID    string
	IP    string
	State string
	Stop  func() error
	Logs  func() (string, error)
}

// HypervisorDriver abstracts low-level hypervisor actions (QEMU vs Firecracker).
type HypervisorDriver interface {
	LaunchVM(ctx context.Context, id string, imageName string, memoryMB int, env map[string]string) (*ActiveVM, error)
}

// QemuDriver wraps our programmatic kraft runner.
type QemuDriver struct {
	k *kraft.Kraft
}

// NewQemuDriver returns a new QEMU-based hypervisor driver.
func NewQemuDriver() HypervisorDriver {
	return &QemuDriver{
		k: kraft.New(),
	}
}

// LaunchVM launches a local Guest VM using QEMU emulation.
func (d *QemuDriver) LaunchVM(ctx context.Context, id string, imageName string, memoryMB int, env map[string]string) (*ActiveVM, error) {
	var logBuf bytes.Buffer
	var bufMu sync.Mutex
	startTime := time.Now()

	// 1. Create a mock temporary overlay disk file to simulate secure shredding locally
	overlayPath := filepath.Join(os.TempDir(), fmt.Sprintf("dvr-overlay-%s.ext4", id))
	mockData := []byte("TENANT_TOKEN=session_secret_key_12345\nDATABASE_URL=postgres://tenant:secret@localhost:5432\n")
	if err := os.WriteFile(overlayPath, mockData, 0666); err != nil {
		return nil, fmt.Errorf("failed to allocate mock overlay disk: %w", err)
	}

	// Wrap buffer to be thread-safe
	safeWriter := &safeBufferWriter{
		buf: &logBuf,
		mu:  &bufMu,
	}

	cfg := kraft.LaunchConfig{
		ProjectDir: imageName,
		Target:     "qemu/x86_64",
		Plat:       "qemu",
		Memory:     fmt.Sprintf("%dMi", memoryMB),
		NoPrompt:   true,
	}

	instance, err := d.k.LaunchSlice(ctx, cfg, safeWriter, safeWriter)
	if err != nil {
		_ = os.Remove(overlayPath)
		return nil, fmt.Errorf("qemu launch failed: %w", err)
	}

	active := &ActiveVM{
		ID:    id,
		IP:    "127.0.0.1",
		State: "running",
		Stop: func() error {
			errStop := instance.Stop()
			
			// Zero-trust disk shredding
			shredStats, errShred := storage.ShredFile(context.Background(), overlayPath)
			if errShred != nil {
				fmt.Printf("⚠️  [shred-error] Failed to shred overlay disk: %v\n", errShred)
			} else {
				// Log cryptographic destruction audit
				uptime := time.Since(startTime).Seconds()
				audit := storage.DestructionAudit{
					InstanceID:    id,
					UptimeSeconds: uptime,
					AllocatedRAM:  memoryMB,
					OverlayPath:   overlayPath,
					ShredStats:    *shredStats,
					Timestamp:     time.Now(),
				}
				_ = storage.RegisterAudit(audit)
			}

			return errStop
		},
		Logs: func() (string, error) {
			bufMu.Lock()
			defer bufMu.Unlock()
			return logBuf.String(), nil
		},
	}

	return active, nil
}

type safeBufferWriter struct {
	buf *bytes.Buffer
	mu  *sync.Mutex
}

func (w *safeBufferWriter) Write(p []byte) (n int, err error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.buf.Write(p)
}

// FirecrackerDriver represents the production Linux microVM runner.
type FirecrackerDriver struct {
	BinPath    string
	KernelPath string
	netMgr     network.NetworkManager
}

func NewFirecrackerDriver() HypervisorDriver {
	return &FirecrackerDriver{
		BinPath:    "firecracker",
		KernelPath: "/var/lib/dvr/kernels/vmlinux",
		netMgr:     network.NewManager("dvr-br0", "172.20.0.1/16"),
	}
}

func (d *FirecrackerDriver) LaunchVM(ctx context.Context, id string, imageName string, memoryMB int, env map[string]string) (*ActiveVM, error) {
	var logBuf bytes.Buffer
	var bufMu sync.Mutex
	startTime := time.Now()

	safeWriter := &safeBufferWriter{
		buf: &logBuf,
		mu:  &bufMu,
	}

	// 1. Initialize host-level bridge (defensive run)
	if err := d.netMgr.SetupBridge(ctx); err != nil {
		return nil, fmt.Errorf("failed to setup host network bridge: %w", err)
	}

	// 2. Allocate host TAP interface
	tapName, macAddr, err := d.netMgr.AllocateTAP(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to allocate host TAP interface: %w", err)
	}

	// 3. Create unique temporary overlay from base image
	overlayPath := filepath.Join(os.TempDir(), fmt.Sprintf("dvr-fc-overlay-%s.ext4", id))
	if err := copyFile(imageName, overlayPath); err != nil {
		_ = d.netMgr.ReleaseTAP(ctx, id)
		return nil, fmt.Errorf("failed to allocate unique instance overlay disk: %w", err)
	}

	// Use temporary directory for UDS socket to ensure write permissions without root
	socketPath := filepath.Join(os.TempDir(), fmt.Sprintf("dvr-fc-%s.socket", id))
	
	orchestrator := firecracker.NewOrchestrator(socketPath, d.BinPath)
	orchestrator.Stdout = safeWriter
	orchestrator.Stderr = safeWriter

	// 4. Spawns firecracker process
	if err := orchestrator.StartProcess(ctx); err != nil {
		_ = os.Remove(overlayPath)
		_ = d.netMgr.ReleaseTAP(ctx, id)
		return nil, fmt.Errorf("failed to start firecracker orchestrator process: %w", err)
	}

	// 5. Configure VM parameters via socket (Kconfig + unique overlay filesystem)
	if err := orchestrator.Configure(ctx, d.KernelPath, overlayPath, 1, memoryMB); err != nil {
		_ = orchestrator.Stop()
		_ = os.Remove(overlayPath)
		_ = d.netMgr.ReleaseTAP(ctx, id)
		return nil, fmt.Errorf("failed to configure firecracker VM over UDS: %w", err)
	}

	// 6. Link VM interface to the host TAP device via socket
	if err := orchestrator.ConfigureNetwork(ctx, tapName, macAddr); err != nil {
		_ = orchestrator.Stop()
		_ = os.Remove(overlayPath)
		_ = d.netMgr.ReleaseTAP(ctx, id)
		return nil, fmt.Errorf("failed to configure firecracker VM network over UDS: %w", err)
	}

	// 7. Boot Guest microVM slice
	if err := orchestrator.StartVM(ctx); err != nil {
		_ = orchestrator.Stop()
		_ = os.Remove(overlayPath)
		_ = d.netMgr.ReleaseTAP(ctx, id)
		return nil, fmt.Errorf("failed to boot firecracker VM: %w", err)
	}

	active := &ActiveVM{
		ID:    id,
		IP:    "172.20.0.2", // Will be allocated dynamically in a future milestone, but maps inside bridge CIDR now
		State: "running",
		Stop: func() error {
			errStop := orchestrator.Stop()
			
			// Zero-trust disk shredding
			shredStats, errShred := storage.ShredFile(context.Background(), overlayPath)
			if errShred != nil {
				fmt.Printf("⚠️  [shred-error] Failed to shred overlay disk: %v\n", errShred)
			} else {
				// Log cryptographic destruction audit
				uptime := time.Since(startTime).Seconds()
				audit := storage.DestructionAudit{
					InstanceID:    id,
					UptimeSeconds: uptime,
					AllocatedRAM:  memoryMB,
					OverlayPath:   overlayPath,
					ShredStats:    *shredStats,
					Timestamp:     time.Now(),
				}
				_ = storage.RegisterAudit(audit)
			}

			errNet := d.netMgr.ReleaseTAP(context.Background(), id)
			if errStop != nil {
				return errStop
			}
			return errNet
		},
		Logs: func() (string, error) {
			bufMu.Lock()
			defer bufMu.Unlock()
			return logBuf.String(), nil
		},
	}

	return active, nil
}

// Helper utility to copy files (used to duplicate base rootfs overlays).
func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err = io.Copy(out, in); err != nil {
		return err
	}
	return out.Sync()
}
