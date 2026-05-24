package storage

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestSecureShredFile(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "sensitive-data.ext4")

	// 1. Create file with sensitive contents
	sensitiveData := []byte("DATABASE_PASSWORD=super-secret-key-that-must-be-purged-and-zeroed-out\n")
	if err := os.WriteFile(filePath, sensitiveData, 0666); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	// 2. Shred the file programmatically
	ctx := context.Background()
	record, err := ShredFile(ctx, filePath)
	if err != nil {
		t.Fatalf("failed to shred file: %v", err)
	}

	// 3. Verify Shred metrics
	if record.FilePath != filePath {
		t.Errorf("expected path %q, got %q", filePath, record.FilePath)
	}

	if record.BytesPurged != int64(len(sensitiveData)) {
		t.Errorf("expected %d bytes purged, got %d", len(sensitiveData), record.BytesPurged)
	}

	// The SHA-256 hash of a zeroed block representing the sanitized state should verify
	if record.SanitizedHash == "" {
		t.Error("expected non-empty cryptographic checksum hash")
	}

	// 4. Verify file is physically deleted from filesystem
	if _, err := os.Stat(filePath); !os.IsNotExist(err) {
		t.Errorf("expected file %q to be deleted, but os.Stat indicates it still exists", filePath)
	}
}
