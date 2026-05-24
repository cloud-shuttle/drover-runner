package network

import (
	"context"
	"crypto/sha256"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
)

// NetworkManager manages host network bridges, TAP interface allocations, and NAT boundaries.
type NetworkManager interface {
	SetupBridge(ctx context.Context) error
	AllocateTAP(ctx context.Context, instanceID string) (tapName string, macAddr string, err error)
	ReleaseTAP(ctx context.Context, instanceID string) error
}

// NewManager returns a platform-appropriate NetworkManager implementation.
func NewManager(bridgeName, bridgeIP string) NetworkManager {
	if runtime.GOOS == "linux" {
		return &linuxNetworkManager{
			bridgeName: bridgeName,
			bridgeIP:   bridgeIP,
		}
	}
	return &mockNetworkManager{
		bridgeName: bridgeName,
		bridgeIP:   bridgeIP,
	}
}

// Helper to generate a deterministic locally administered unicast MAC address.
func GenerateMAC(id string) string {
	h := sha256.New()
	_, _ = h.Write([]byte(id))
	sum := h.Sum(nil)
	// Unicast locally administered prefix: 06:00
	return fmt.Sprintf("06:00:%02x:%02x:%02x:%02x", sum[0], sum[1], sum[2], sum[3])
}

// mockNetworkManager provides macOS dynamic loopback compatibility.
type mockNetworkManager struct {
	bridgeName string
	bridgeIP   string
}

func (m *mockNetworkManager) SetupBridge(ctx context.Context) error {
	fmt.Printf("   [net-mock] Initialized virtual bridge %s (Gateway: %s)\n", m.bridgeName, m.bridgeIP)
	return nil
}

func (m *mockNetworkManager) AllocateTAP(ctx context.Context, instanceID string) (string, string, error) {
	tapName := fmt.Sprintf("dvr-t-%s", instanceID)
	if len(tapName) > 15 {
		tapName = tapName[:15]
	}
	macAddr := GenerateMAC(instanceID)
	fmt.Printf("   [net-mock] Allocated virtual link %s (MAC: %s) on bridge %s\n", tapName, macAddr, m.bridgeName)
	return tapName, macAddr, nil
}

func (m *mockNetworkManager) ReleaseTAP(ctx context.Context, instanceID string) error {
	tapName := fmt.Sprintf("dvr-t-%s", instanceID)
	if len(tapName) > 15 {
		tapName = tapName[:15]
	}
	fmt.Printf("   [net-mock] Released link %s from bridge %s\n", tapName, m.bridgeName)
	return nil
}

// linuxNetworkManager drives real ip/iptables configurations on Linux bare-metal hosts.
type linuxNetworkManager struct {
	bridgeName string
	bridgeIP   string
}

func (m *linuxNetworkManager) SetupBridge(ctx context.Context) error {
	// Check if bridge link already exists
	if exec.CommandContext(ctx, "ip", "link", "show", m.bridgeName).Run() == nil {
		return nil
	}

	fmt.Printf("   [net] Creating host bridge link %s...\n", m.bridgeName)

	// Create bridge interface
	if err := exec.CommandContext(ctx, "ip", "link", "add", "name", m.bridgeName, "type", "bridge").Run(); err != nil {
		return fmt.Errorf("failed to create bridge: %w", err)
	}

	// Assign Gateway IP
	if err := exec.CommandContext(ctx, "ip", "addr", "add", m.bridgeIP, "dev", m.bridgeName).Run(); err != nil {
		return fmt.Errorf("failed to bind bridge address: %w", err)
	}

	// Bring bridge up
	if err := exec.CommandContext(ctx, "ip", "link", "set", "dev", m.bridgeName, "up").Run(); err != nil {
		return fmt.Errorf("failed to enable bridge link: %w", err)
	}

	// Extract subnet CIDR for NAT masquerading
	subnet := "172.20.0.0/16"
	if parts := strings.Split(m.bridgeIP, "/"); len(parts) == 2 {
		// Calculate simple class B subnet prefix
		ipParts := strings.Split(parts[0], ".")
		if len(ipParts) == 4 {
			subnet = fmt.Sprintf("%s.%s.0.0/%s", ipParts[0], ipParts[1], parts[1])
		}
	}

	// Inject Outbound Masquerade Rules (NAT) defensively
	_ = exec.CommandContext(ctx, "iptables", "-t", "nat", "-D", "POSTROUTING", "-s", subnet, "!", "-o", m.bridgeName, "-j", "MASQUERADE").Run()
	if err := exec.CommandContext(ctx, "iptables", "-t", "nat", "-A", "POSTROUTING", "-s", subnet, "!", "-o", m.bridgeName, "-j", "MASQUERADE").Run(); err != nil {
		return fmt.Errorf("failed to inject NAT masquerade: %w", err)
	}

	// Inject Forward rules
	_ = exec.CommandContext(ctx, "iptables", "-D", "FORWARD", "-i", m.bridgeName, "!", "-o", m.bridgeName, "-j", "ACCEPT").Run()
	if err := exec.CommandContext(ctx, "iptables", "-A", "FORWARD", "-i", m.bridgeName, "!", "-o", m.bridgeName, "-j", "ACCEPT").Run(); err != nil {
		return fmt.Errorf("failed to allow forward out: %w", err)
	}

	_ = exec.CommandContext(ctx, "iptables", "-D", "FORWARD", "-o", m.bridgeName, "-m", "conntrack", "--ctstate", "RELATED,ESTABLISHED", "-j", "ACCEPT").Run()
	if err := exec.CommandContext(ctx, "iptables", "-A", "FORWARD", "-o", m.bridgeName, "-m", "conntrack", "--ctstate", "RELATED,ESTABLISHED", "-j", "ACCEPT").Run(); err != nil {
		return fmt.Errorf("failed to allow established return forward: %w", err)
	}

	// Enforce strict Tenant Isolation (Blocked forward traffic guest-to-guest on same bridge)
	_ = exec.CommandContext(ctx, "iptables", "-D", "FORWARD", "-i", m.bridgeName, "-o", m.bridgeName, "-j", "DROP").Run()
	if err := exec.CommandContext(ctx, "iptables", "-A", "FORWARD", "-i", m.bridgeName, "-o", m.bridgeName, "-j", "DROP").Run(); err != nil {
		return fmt.Errorf("failed to inject tenant isolation drops: %w", err)
	}

	return nil
}

func (m *linuxNetworkManager) AllocateTAP(ctx context.Context, instanceID string) (string, string, error) {
	tapName := fmt.Sprintf("dvr-t-%s", instanceID)
	// Linux interface names must be 15 chars or less
	if len(tapName) > 15 {
		tapName = tapName[:15]
	}
	macAddr := GenerateMAC(instanceID)

	fmt.Printf("   [net] Allocating host TAP device %s (MAC: %s)...\n", tapName, macAddr)

	// Create TAP interface link
	if err := exec.CommandContext(ctx, "ip", "tuntap", "add", "dev", tapName, "mode", "tap").Run(); err != nil {
		return "", "", fmt.Errorf("failed to create TAP link: %w", err)
	}

	// Attach TAP to shared bridge
	if err := exec.CommandContext(ctx, "ip", "link", "set", "dev", tapName, "master", m.bridgeName).Run(); err != nil {
		_ = m.deleteLink(ctx, tapName)
		return "", "", fmt.Errorf("failed to link TAP to bridge: %w", err)
	}

	// Bring TAP up
	if err := exec.CommandContext(ctx, "ip", "link", "set", "dev", tapName, "up").Run(); err != nil {
		_ = m.deleteLink(ctx, tapName)
		return "", "", fmt.Errorf("failed to enable TAP link: %w", err)
	}

	return tapName, macAddr, nil
}

func (m *linuxNetworkManager) ReleaseTAP(ctx context.Context, instanceID string) error {
	tapName := fmt.Sprintf("dvr-t-%s", instanceID)
	if len(tapName) > 15 {
		tapName = tapName[:15]
	}

	fmt.Printf("   [net] Releasing host TAP device %s...\n", tapName)
	return m.deleteLink(ctx, tapName)
}

func (m *linuxNetworkManager) deleteLink(ctx context.Context, tapName string) error {
	_ = exec.CommandContext(ctx, "ip", "link", "set", "dev", tapName, "down").Run()
	return exec.CommandContext(ctx, "ip", "link", "delete", "dev", tapName).Run()
}
