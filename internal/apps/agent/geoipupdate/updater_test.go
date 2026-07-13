package geoipupdate

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
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

func TestEnsureInitialDatabasesCopiesEmbeddedCityWithoutDownload(t *testing.T) {
	tempDir := t.TempDir()
	countryPath := filepath.Join(tempDir, "GeoLite2-Country.mmdb")
	cityPath := filepath.Join(tempDir, "GeoLite2-City.mmdb")
	updater := &Updater{
		MMDBPath:        countryPath,
		CityMMDBPath:    cityPath,
		CityDownloadURL: "https://geo.example/GeoLite2-City.mmdb",
		downloadDatabase: func(_ context.Context, path, downloadURL string) error {
			t.Fatalf("initial embedded seed must not download %s from %s", path, downloadURL)
			return nil
		},
	}

	if err := updater.EnsureInitialDatabases(context.Background()); err != nil {
		t.Fatalf("EnsureInitialDatabases failed: %v", err)
	}
	if _, err := os.Stat(countryPath); err != nil {
		t.Fatalf("expected embedded Country database: %v", err)
	}
	data, err := os.ReadFile(cityPath)
	if err != nil || len(data) == 0 {
		t.Fatalf("expected embedded City database, size=%d err=%v", len(data), err)
	}
}

func TestEnsureInitialDatabasesKeepsExistingCityWithoutDownload(t *testing.T) {
	tempDir := t.TempDir()
	countryPath := filepath.Join(tempDir, "GeoLite2-Country.mmdb")
	cityPath := filepath.Join(tempDir, "GeoLite2-City.mmdb")
	if err := os.WriteFile(cityPath, []byte("existing-city"), 0o600); err != nil {
		t.Fatal(err)
	}
	updater := &Updater{
		MMDBPath:        countryPath,
		CityMMDBPath:    cityPath,
		CityDownloadURL: "https://geo.example/GeoLite2-City.mmdb",
		downloadDatabase: func(_ context.Context, _, _ string) error {
			return errors.New("city unavailable")
		},
	}

	if err := updater.EnsureInitialDatabases(context.Background()); err != nil {
		t.Fatalf("EnsureInitialDatabases failed: %v", err)
	}
	if _, err := os.Stat(countryPath); err != nil {
		t.Fatalf("expected Country fallback to remain available: %v", err)
	}
	data, err := os.ReadFile(cityPath)
	if err != nil || string(data) != "existing-city" {
		t.Fatalf("expected existing City database to remain untouched, data=%q err=%v", data, err)
	}
}

func TestUpdateDatabasesAttemptsCityAfterCountryFailure(t *testing.T) {
	var paths []string
	updater := &Updater{
		MMDBPath:        "/data/GeoLite2-Country.mmdb",
		DownloadURL:     "https://geo.example/GeoLite2-Country.mmdb",
		CityMMDBPath:    "/data/GeoLite2-City.mmdb",
		CityDownloadURL: "https://geo.example/GeoLite2-City.mmdb",
		downloadDatabase: func(_ context.Context, path, _ string) error {
			paths = append(paths, path)
			if path == "/data/GeoLite2-Country.mmdb" {
				return errors.New("country unavailable")
			}
			return nil
		},
	}

	err := updater.updateDatabases(context.Background())
	if err == nil || !slices.Equal(paths, []string{"/data/GeoLite2-Country.mmdb", "/data/GeoLite2-City.mmdb"}) {
		t.Fatalf("expected independent Country then City attempts, paths=%#v err=%v", paths, err)
	}
}
