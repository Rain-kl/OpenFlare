// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package geoipupdate

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestEnsureInitialDatabasesDownloadsMissingOnly(t *testing.T) {
	tempDir := t.TempDir()
	countryPath := filepath.Join(tempDir, "GeoLite2-Country.mmdb")
	cityPath := filepath.Join(tempDir, "GeoLite2-City.mmdb")
	if err := os.WriteFile(cityPath, []byte("existing-city"), 0o600); err != nil {
		t.Fatal(err)
	}

	var downloaded []string
	updater := &Updater{
		MMDBPath:        countryPath,
		DownloadURL:     "https://geo.example/GeoLite2-Country.mmdb",
		CityMMDBPath:    cityPath,
		CityDownloadURL: "https://geo.example/GeoLite2-City.mmdb",
		downloadDatabase: func(_ context.Context, path, _ string) error {
			downloaded = append(downloaded, path)
			return os.WriteFile(path, []byte("downloaded"), 0o600)
		},
	}

	if err := updater.EnsureInitialDatabases(context.Background()); err != nil {
		t.Fatalf("EnsureInitialDatabases failed: %v", err)
	}
	if !slices.Equal(downloaded, []string{countryPath}) {
		t.Fatalf("expected only missing Country download, got %#v", downloaded)
	}
	if data, err := os.ReadFile(cityPath); err != nil || string(data) != "existing-city" {
		t.Fatalf("existing City must stay untouched, data=%q err=%v", data, err)
	}
	if data, err := os.ReadFile(countryPath); err != nil || string(data) != "downloaded" {
		t.Fatalf("Country should be seeded via download, data=%q err=%v", data, err)
	}
}

func TestEnsureInitialDatabasesNoOpWhenPresent(t *testing.T) {
	tempDir := t.TempDir()
	countryPath := filepath.Join(tempDir, "GeoLite2-Country.mmdb")
	cityPath := filepath.Join(tempDir, "GeoLite2-City.mmdb")
	if err := os.WriteFile(countryPath, []byte("c"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cityPath, []byte("city"), 0o600); err != nil {
		t.Fatal(err)
	}
	updater := &Updater{
		MMDBPath:        countryPath,
		DownloadURL:     "https://geo.example/GeoLite2-Country.mmdb",
		CityMMDBPath:    cityPath,
		CityDownloadURL: "https://geo.example/GeoLite2-City.mmdb",
		downloadDatabase: func(_ context.Context, path, downloadURL string) error {
			t.Fatalf("must not download when files exist: %s %s", path, downloadURL)
			return nil
		},
	}
	if err := updater.EnsureInitialDatabases(context.Background()); err != nil {
		t.Fatalf("EnsureInitialDatabases failed: %v", err)
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

func TestEnsureInitialDatabasesRejectsDirectoryPath(t *testing.T) {
	tempDir := t.TempDir()
	// Point Country path at a directory so fileExists must not treat it as seeded.
	updater := &Updater{
		MMDBPath:    tempDir,
		DownloadURL: "https://geo.example/GeoLite2-Country.mmdb",
		downloadDatabase: func(_ context.Context, path, downloadURL string) error {
			t.Fatalf("must not download when path is a directory: %s %s", path, downloadURL)
			return nil
		},
	}

	err := updater.EnsureInitialDatabases(context.Background())
	if err == nil {
		t.Fatal("expected error when MMDB path is a directory")
	}
	if !strings.Contains(err.Error(), "not a regular file") {
		t.Fatalf("expected regular-file error, got %v", err)
	}
}
