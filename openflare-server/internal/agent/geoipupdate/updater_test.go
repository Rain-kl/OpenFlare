package geoipupdate

import (
	"os"
	"path/filepath"
	"testing"
)

func TestEnsureInitialDatabaseCopiesEmbeddedMMDB(t *testing.T) {
	tempDir := t.TempDir()
	path := filepath.Join(tempDir, "GeoLite2-Country.mmdb")
	updater := &Updater{MMDBPath: path}

	if err := updater.EnsureInitialDatabase(); err != nil {
		t.Fatalf("EnsureInitialDatabase failed: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("expected mmdb to exist: %v", err)
	}
	if info.Size() == 0 {
		t.Fatal("expected copied mmdb to be non-empty")
	}
}
