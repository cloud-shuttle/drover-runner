package firecracker

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestOrchestratorConfigurationAndBoot(t *testing.T) {
	// Construct a short path under /tmp to bypass the 104-char macOS socket limit
	socketPath := filepath.Join("/tmp", fmt.Sprintf("mock-fc-%d.sock", time.Now().UnixNano()))
	defer os.Remove(socketPath)

	// Spin up a mock Firecracker UDS HTTP Server
	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		t.Fatalf("failed to create Unix listener: %v", err)
	}
	defer listener.Close()

	calledEndpoints := make(map[string]bool)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calledEndpoints[r.URL.Path] = true

		if r.Method != http.MethodPut {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		// Read and validate payloads
		var body map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		switch r.URL.Path {
		case "/machine-config":
			if body["vcpu_count"] != float64(2) || body["mem_size_mib"] != float64(256) {
				t.Errorf("invalid machine config payload: %+v", body)
			}
		case "/boot-source":
			if body["kernel_image_path"] != "/tmp/vmlinux" {
				t.Errorf("invalid boot source payload: %+v", body)
			}
		case "/drives/rootfs":
			if body["path_on_host"] != "/tmp/rootfs.ext4" || body["drive_id"] != "rootfs" {
				t.Errorf("invalid drive payload: %+v", body)
			}
		case "/actions":
			if body["action_type"] != "InstanceStart" {
				t.Errorf("invalid action payload: %+v", body)
			}
		case "/network-interfaces/eth0":
			if body["iface_id"] != "eth0" || body["host_dev_name"] != "dvr-tap-inst-1" || body["guest_mac"] != "06:00:11:22:33:44" {
				t.Errorf("invalid network config payload: %+v", body)
			}
		default:
			w.WriteHeader(http.StatusNotFound)
			return
		}

		w.WriteHeader(http.StatusNoContent)
	})

	srv := &http.Server{Handler: handler}
	go func() {
		_ = srv.Serve(listener)
	}()
	defer srv.Shutdown(context.Background())

	// Instantiate the orchestrator (pointing to our mock socket)
	orchestrator := NewOrchestrator(socketPath, "mock-binary")

	ctx := context.Background()

	// 1. Configure over UDS socket
	err = orchestrator.Configure(ctx, "/tmp/vmlinux", "/tmp/rootfs.ext4", 2, 256)
	if err != nil {
		t.Fatalf("failed to configure orchestrator: %v", err)
	}

	// 2. Configure Network over UDS socket
	err = orchestrator.ConfigureNetwork(ctx, "dvr-tap-inst-1", "06:00:11:22:33:44")
	if err != nil {
		t.Fatalf("failed to configure network: %v", err)
	}

	// 3. Start VM over UDS socket
	err = orchestrator.StartVM(ctx)
	if err != nil {
		t.Fatalf("failed to start VM: %v", err)
	}

	// Verify all expected mock REST endpoints were called
	expected := []string{"/machine-config", "/boot-source", "/drives/rootfs", "/network-interfaces/eth0", "/actions"}
	for _, path := range expected {
		if !calledEndpoints[path] {
			t.Errorf("expected endpoint %s was not called", path)
		}
	}
}
