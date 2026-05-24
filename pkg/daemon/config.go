package daemon

import (
	"time"
)

// Config holds the runtime configuration parameters for the dvr host hypervisor daemon.
type Config struct {
	Port           int           // HTTP server listen port
	AuthToken      string        // Bearer token required for REST API authentication
	HypervisorMode string        // "qemu" or "firecracker"
	KernelPath     string        // Path to default kernel image
	TelemetrySecs  time.Duration // Metrics collection interval in seconds
}

// DefaultConfig returns a sensible baseline configuration for local development.
func DefaultConfig() Config {
	return Config{
		Port:           8080,
		AuthToken:      "dvr-admin-token",
		HypervisorMode: "qemu",
		KernelPath:     "",
		TelemetrySecs:  10 * time.Second,
	}
}
