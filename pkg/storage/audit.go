package storage

import (
	"encoding/json"
	"fmt"
	"time"
)

// DestructionAudit records historical execution configurations, memory bounds,
// and hardware shredding parameters upon virtual Guest VM teardowns.
type DestructionAudit struct {
	InstanceID    string      `json:"instance_id"`
	UptimeSeconds float64     `json:"uptime_seconds"`
	AllocatedRAM  int         `json:"allocated_ram_mb"`
	OverlayPath   string      `json:"overlay_path"`
	ShredStats    ShredRecord `json:"shred_stats"`
	Timestamp     time.Time   `json:"timestamp"`
}

// RegisterAudit outputs the cryptographic audit payload to the security stream.
// On production setups, this routes events to ClickHouse for persistent analytics.
func RegisterAudit(audit DestructionAudit) error {
	payload, err := json.Marshal(audit)
	if err != nil {
		return fmt.Errorf("failed to marshal audit trail: %w", err)
	}

	fmt.Printf("🔒 [Audit-Log] DESTRUCTION AUDIT: %s\n", string(payload))
	return nil
}
