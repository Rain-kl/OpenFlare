//go:build !windows

// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

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
