package kraft

import (
	"context"
	"fmt"
	"io"
	"os/exec"
	"sync"
	"syscall"
	"time"
)

// LaunchConfig contains options to programmatically run a unikernel local virtual slice.
type LaunchConfig struct {
	ProjectDir string
	Target     string // e.g. "qemu/x86_64"
	Plat       string // e.g. "qemu"
	Memory     string // e.g. "64Mi"
	NoPrompt   bool
}

// SliceInstance represents a programmatically managed guest OS VM slice running locally.
type SliceInstance struct {
	ID      string
	cmd     *exec.Cmd
	cancel  context.CancelFunc
	waitMu  sync.RWMutex
	waitErr error
	exited  chan struct{}
}

// LaunchSlice programmatically invokes 'kraft run' with the specified configuration,
// captures its execution handle, streams standard output/error, and returns a SliceInstance control.
func (k *Kraft) LaunchSlice(ctx context.Context, cfg LaunchConfig, stdout, stderr io.Writer) (*SliceInstance, error) {
	bin := k.Binary
	if bin == "" {
		bin = "kraft"
	}

	kraftArgs := []string{"run"}
	if cfg.Target != "" {
		kraftArgs = append(kraftArgs, "-t", cfg.Target)
	}
	if cfg.Plat != "" {
		kraftArgs = append(kraftArgs, "--plat", cfg.Plat)
	}
	if cfg.Memory != "" {
		kraftArgs = append(kraftArgs, "--memory", cfg.Memory)
	}
	if cfg.NoPrompt {
		kraftArgs = append(kraftArgs, "--no-prompt")
	}
	kraftArgs = append(kraftArgs, cfg.ProjectDir)

	// Create a cancellable context for the process
	processCtx, cancel := context.WithCancel(ctx)

	cmd := exec.CommandContext(processCtx, bin, kraftArgs...)
	cmd.Dir = cfg.ProjectDir

	// Bind stdout and stderr
	cmd.Stdout = stdout
	cmd.Stderr = stderr

	// Configure process group so signals can be sent cleanly to the child's children (like QEMU subprocesses spawned by kraft)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
	}

	if err := cmd.Start(); err != nil {
		cancel()
		return nil, fmt.Errorf("failed to start virtual slice: %w", err)
	}

	id := fmt.Sprintf("slice-%d", cmd.Process.Pid)
	exited := make(chan struct{})

	instance := &SliceInstance{
		ID:     id,
		cmd:    cmd,
		cancel: cancel,
		exited: exited,
	}

	// Background goroutine to watch the process completion
	go func() {
		err := cmd.Wait()
		cancel()
		instance.waitMu.Lock()
		instance.waitErr = err
		instance.waitMu.Unlock()
		close(exited)
	}()

	return instance, nil
}

// Wait blocks until the virtual slice exits, returning any run error.
func (s *SliceInstance) Wait() error {
	<-s.exited
	s.waitMu.RLock()
	defer s.waitMu.RUnlock()
	return s.waitErr
}

// Stop gracefully signals the unikernel process group to terminate,
// falling back to a hard kill if it does not exit within a timeout.
func (s *SliceInstance) Stop() error {
	if s.cmd == nil || s.cmd.Process == nil {
		return nil
	}

	// Terminate the process group (using negative PID sends the signal to all processes in that group)
	pgid, err := syscall.Getpgid(s.cmd.Process.Pid)
	if err == nil {
		_ = syscall.Kill(-pgid, syscall.SIGINT) // Send SIGINT to the group
	} else {
		_ = s.cmd.Process.Signal(syscall.SIGINT)
	}

	// Wait for up to 3 seconds for graceful exit
	select {
	case <-s.exited:
		return nil
	case <-time.After(3 * time.Second):
		// Graceful exit timed out, force kill
		if err == nil {
			_ = syscall.Kill(-pgid, syscall.SIGKILL)
		} else {
			_ = s.cmd.Process.Kill()
		}
		<-s.exited
		return nil
	}
}
