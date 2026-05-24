package network

import (
	"context"
	"testing"
)

func TestGenerateMAC(t *testing.T) {
	mac1 := GenerateMAC("inst-1")
	mac2 := GenerateMAC("inst-2")

	if mac1 == mac2 {
		t.Error("expected distinct MAC addresses for distinct instance IDs")
	}

	if mac1[:6] != "06:00:" {
		t.Errorf("expected locally administered unicast prefix '06:00:', got %s", mac1[:6])
	}
}

func TestMockNetworkManagerAllocation(t *testing.T) {
	mgr := NewManager("dvr-br0", "172.20.0.1/16")
	ctx := context.Background()

	err := mgr.SetupBridge(ctx)
	if err != nil {
		t.Fatalf("failed to setup bridge: %v", err)
	}

	tapName, macAddr, err := mgr.AllocateTAP(ctx, "inst-1")
	if err != nil {
		t.Fatalf("failed to allocate TAP: %v", err)
	}

	if tapName != "dvr-t-inst-1" {
		t.Errorf("expected tapName 'dvr-t-inst-1', got %s", tapName)
	}

	if macAddr == "" {
		t.Error("expected non-empty MAC address")
	}

	err = mgr.ReleaseTAP(ctx, "inst-1")
	if err != nil {
		t.Fatalf("failed to release TAP: %v", err)
	}
}
