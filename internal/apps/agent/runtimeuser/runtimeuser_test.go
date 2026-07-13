package runtimeuser

import (
	"os"
	"path/filepath"
	"testing"
)

func TestEnsurePathOwnershipNormalizesModes(t *testing.T) {
	tempDir := t.TempDir()
	dataDir := filepath.Join(tempDir, "data")
	releaseDir := filepath.Join(dataDir, "var", "lib", "openflare", "pages", "releases", "abc")
	if err := os.MkdirAll(releaseDir, 0o700); err != nil {
		t.Fatalf("MkdirAll failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(releaseDir, "index.html"), []byte("ok"), 0o600); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	pagesRoot := filepath.Join(dataDir, "var", "lib", "openflare", "pages")
	if err := EnsurePathOwnership(pagesRoot, 0o755, 0o644); err != nil {
		t.Fatalf("EnsurePathOwnership failed: %v", err)
	}

	dataInfo, err := os.Stat(dataDir)
	if err != nil {
		t.Fatalf("Stat dataDir failed: %v", err)
	}
	if dataInfo.Mode().Perm()&0o005 == 0 {
		t.Fatalf("expected dataDir to be world-traversable, got %o", dataInfo.Mode().Perm())
	}
	indexInfo, err := os.Stat(filepath.Join(releaseDir, "index.html"))
	if err != nil {
		t.Fatalf("Stat index failed: %v", err)
	}
	if indexInfo.Mode().Perm() != 0o644 {
		t.Fatalf("expected mode 0644, got %o", indexInfo.Mode().Perm())
	}
}
