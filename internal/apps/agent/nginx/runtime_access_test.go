package nginx

import (
	"os"
	"path/filepath"
	"testing"
)

func TestEnsureWorkerReadableTreeFixesRestrictedPagesFiles(t *testing.T) {
	tempDir := t.TempDir()
	pagesDir := filepath.Join(tempDir, "data", "var", "lib", "openflare", "pages")
	releaseDir := filepath.Join(pagesDir, "deployments", "1", "releases", "abc123")
	if err := os.MkdirAll(releaseDir, 0o700); err != nil {
		t.Fatalf("MkdirAll failed: %v", err)
	}
	indexPath := filepath.Join(releaseDir, "index.html")
	if err := os.WriteFile(indexPath, []byte("<html></html>"), 0o600); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	if err := EnsureWorldTraversablePath(pagesDir); err != nil {
		t.Fatalf("EnsureWorldTraversablePath failed: %v", err)
	}
	if err := EnsureWorkerReadableTree(pagesDir); err != nil {
		t.Fatalf("EnsureWorkerReadableTree failed: %v", err)
	}

	info, err := os.Stat(indexPath)
	if err != nil {
		t.Fatalf("Stat failed: %v", err)
	}
	if info.Mode().Perm() != nginxConfigFilePerm {
		t.Fatalf("expected index.html mode %o, got %o", nginxConfigFilePerm, info.Mode().Perm())
	}
	etcInfo, err := os.Stat(filepath.Join(tempDir, "data", "var"))
	if err != nil {
		t.Fatalf("Stat var failed: %v", err)
	}
	if etcInfo.Mode().Perm()&0o005 == 0 {
		t.Fatalf("expected var directory to be world-traversable, got %o", etcInfo.Mode().Perm())
	}
}

func TestManagerEnsureWorkerReadAccessIncludesPagesDir(t *testing.T) {
	tempDir := t.TempDir()
	dataDir := filepath.Join(tempDir, "data")
	pagesRoot := filepath.Join(dataDir, "var", "lib", "openflare", "pages")
	releaseDir := filepath.Join(pagesRoot, "deployments", "1", "releases", "abc123")
	if err := os.MkdirAll(releaseDir, 0o700); err != nil {
		t.Fatalf("MkdirAll failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(releaseDir, "index.html"), []byte("ok"), 0o600); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	manager := &Manager{PagesDir: pagesRoot}
	if err := manager.EnsureWorkerReadAccess(); err != nil {
		t.Fatalf("EnsureWorkerReadAccess failed: %v", err)
	}

	info, err := os.Stat(filepath.Join(tempDir, "data"))
	if err != nil {
		t.Fatalf("Stat data failed: %v", err)
	}
	if info.Mode().Perm()&0o005 == 0 {
		t.Fatalf("expected data directory to be world-traversable, got %o", info.Mode().Perm())
	}
	indexInfo, err := os.Stat(filepath.Join(releaseDir, "index.html"))
	if err != nil {
		t.Fatalf("Stat index failed: %v", err)
	}
	if indexInfo.Mode().Perm() != nginxConfigFilePerm {
		t.Fatalf("expected index.html mode %o, got %o", nginxConfigFilePerm, indexInfo.Mode().Perm())
	}
}
