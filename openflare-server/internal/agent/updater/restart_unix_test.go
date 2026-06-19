//go:build !windows

package updater

import (
	"path/filepath"
	"testing"
)

func TestRemoveBackupBinaryIgnoresMissingFile(t *testing.T) {
	backupPath := filepath.Join(t.TempDir(), "openflare-agent.bak")
	if err := removeBackupBinary(backupPath); err != nil {
		t.Fatalf("expected missing backup cleanup to be ignored: %v", err)
	}
}
