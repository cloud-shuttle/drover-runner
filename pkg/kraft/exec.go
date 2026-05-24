package kraft

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// CommandError is a rich error type returned when a kraft subcommand fails.
// It captures exit code, combined output (or the streamed tail), and the original error
// so that higher layers (daemon, drover) can make good decisions and surface nice messages.
type CommandError struct {
	Command  string   // e.g. "kraft"
	Args     []string
	Dir      string
	ExitCode int
	Output   string // last portion of combined stdout+stderr (useful even with streaming)
	Err      error  // underlying exec error
}

func (e *CommandError) Error() string {
	msg := fmt.Sprintf("kraft %s failed", strings.Join(append([]string{e.Command}, e.Args...), " "))
	if e.Dir != "" {
		msg += fmt.Sprintf(" (in %s)", e.Dir)
	}
	if e.ExitCode != 0 {
		msg += fmt.Sprintf(" (exit %d)", e.ExitCode)
	}
	if e.Output != "" {
		// Only show a snippet in the error string; full output was already streamed to user
		snippet := e.Output
		if len(snippet) > 300 {
			snippet = snippet[len(snippet)-300:]
		}
		msg += "\n--- kraft output (tail) ---\n" + snippet
	}
	if e.Err != nil {
		msg += "\nroot cause: " + e.Err.Error()
	}
	return msg
}

func (e *CommandError) Unwrap() error {
	return e.Err
}

// IsKraftError reports whether err is or wraps a *CommandError.
func IsKraftError(err error) bool {
	var ke *CommandError
	return errors.As(err, &ke)
}

// Kraft is the main entrypoint for invoking kraft from dvr.
type Kraft struct {
	// Binary is the path to the kraft executable. Defaults to "kraft" (in PATH).
	Binary string

	// LogLevel is passed as --log-level if set.
	LogLevel string
}

// New returns a Kraft helper with sane defaults.
func New() *Kraft {
	return &Kraft{Binary: "kraft"}
}

// WithBinary allows overriding the kraft binary (useful for tests or vendored installs).
func (k *Kraft) WithBinary(bin string) *Kraft {
	k.Binary = bin
	return k
}

// Run executes a kraft subcommand, streaming stdout/stderr live to the provided writers
// (usually os.Stdout/os.Stderr so the user sees real-time progress).
// On success it returns nil.
// On failure it returns a *CommandError with rich context.
func (k *Kraft) Run(ctx context.Context, dir string, args []string, stdout, stderr *os.File) error {
	bin := k.Binary
	if bin == "" {
		bin = "kraft"
	}

	cmd := exec.CommandContext(ctx, bin, args...)
	cmd.Dir = dir

	cmd.Stdout = stdout
	cmd.Stderr = stderr

	err := cmd.Run()

	if err != nil {
		exitCode := 1
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		}

		// For streaming cases the full log is already visible to the user.
		// We still return a structured error for programmatic use.
		return &CommandError{
			Command:  bin,
			Args:     args,
			Dir:      dir,
			ExitCode: exitCode,
			Err:      err,
			// Output left empty here because we streamed; callers who want full capture
			// can use RunCapture instead.
		}
	}
	return nil
}

// RunCapture is like Run but captures all output instead of (or in addition to) streaming.
// Useful for "targets" listing, version probing, or non-interactive subcommands.
func (k *Kraft) RunCapture(ctx context.Context, dir string, args []string) (string, error) {
	bin := k.Binary
	if bin == "" {
		bin = "kraft"
	}

	cmd := exec.CommandContext(ctx, bin, args...)
	cmd.Dir = dir

	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf

	err := cmd.Run()

	output := buf.String()

	if err != nil {
		exitCode := 1
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		}
		return output, &CommandError{
			Command:  bin,
			Args:     args,
			Dir:      dir,
			ExitCode: exitCode,
			Output:   output,
			Err:      err,
		}
	}
	return output, nil
}

// Build is a convenience wrapper around "kraft build".
func (k *Kraft) Build(ctx context.Context, dir string, extraArgs []string, stdout, stderr *os.File) error {
	args := append([]string{"build"}, extraArgs...)
	return k.Run(ctx, dir, args, stdout, stderr)
}

// RunVM is a convenience wrapper around "kraft run".
func (k *Kraft) RunVM(ctx context.Context, dir string, extraArgs []string, stdout, stderr *os.File) error {
	args := append([]string{"run"}, extraArgs...)
	return k.Run(ctx, dir, args, stdout, stderr)
}

// Version returns the version string of the kraft binary.
func (k *Kraft) Version(ctx context.Context) (string, error) {
	out, err := k.RunCapture(ctx, "", []string{"version"})
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}
