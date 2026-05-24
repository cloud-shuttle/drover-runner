package kraft

import (
	"context"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"gopkg.in/yaml.v3"
)

func TestKraftfileParsing(t *testing.T) {
	yamlData := `
spec: v0.6
name: testapp
unikraft: stable
targets:
  - qemu/x86_64
  - plat: fc
    arch: arm64
    name: fc-arm64
`

	var kf Kraftfile
	if err := yaml.Unmarshal([]byte(yamlData), &kf); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if kf.Name != "testapp" {
		t.Errorf("expected name 'testapp', got %s", kf.Name)
	}

	targets := kf.ListTargetStrings()
	if len(targets) != 2 {
		t.Fatalf("expected 2 targets, got %d", len(targets))
	}

	if targets[0] != "qemu/x86_64" {
		t.Errorf("expected target 'qemu/x86_64', got %s", targets[0])
	}

	if targets[1] != "fc/arm64" {
		t.Errorf("expected target 'fc/arm64', got %s", targets[1])
	}
}

func TestLaunchSliceArgs(t *testing.T) {
	bin, err := exec.LookPath("echo")
	if err != nil {
		bin = "echo"
	}
	k := New().WithBinary(bin)

	cfg := LaunchConfig{
		ProjectDir: t.TempDir(),
		Target:     "qemu/x86_64",
		Plat:       "qemu",
		Memory:     "128Mi",
		NoPrompt:   true,
	}

	// Just verifying that it can be initialized and launched programmatically (we use /bin/echo to exit instantly)
	ctx := context.Background()
	instance, err := k.LaunchSlice(ctx, cfg, io.Discard, io.Discard)
	if err != nil {
		t.Fatalf("failed to launch: %v", err)
	}

	if instance == nil {
		t.Fatal("expected non-nil instance")
	}

	// Wait for echo to exit
	err = instance.Wait()
	if err != nil {
		t.Logf("echo exited with: %v (expected)", err)
	}

	if !strings.HasPrefix(instance.ID, "slice-") {
		t.Errorf("expected ID prefix 'slice-', got %s", instance.ID)
	}
}

func TestStopSliceInstance(t *testing.T) {
	// Create a temp file containing a script that sleeps
	tmpDir := t.TempDir()
	scriptPath := filepath.Join(tmpDir, "mock-kraft")
	scriptContent := []byte("#!/bin/sh\ntrap 'exit 0' INT TERM\nsleep 10 &\nwait\n")
	if err := os.WriteFile(scriptPath, scriptContent, 0755); err != nil {
		t.Fatalf("failed to write mock script: %v", err)
	}

	k := New().WithBinary(scriptPath)

	cfg := LaunchConfig{
		ProjectDir: tmpDir,
		Target:     "qemu/x86_64",
		Plat:       "qemu",
		Memory:     "128Mi",
		NoPrompt:   true,
	}

	ctx := context.Background()
	instance, err := k.LaunchSlice(ctx, cfg, io.Discard, io.Discard)
	if err != nil {
		t.Fatalf("failed to launch: %v", err)
	}

	// Wait a moment for process to spin up
	time.Sleep(200 * time.Millisecond)

	// Stop it programmatically
	err = instance.Stop()
	if err != nil {
		t.Errorf("expected no error from Stop(), got %v", err)
	}

	// Wait should return immediately without blocking
	waitErr := instance.Wait()
	if waitErr == nil {
		t.Error("expected process to be interrupted and return an error/signal indication, got nil")
	}
}
