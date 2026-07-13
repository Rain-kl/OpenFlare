// Package geoipupdate schedules local MaxMind GeoIP database updates for the agent.
package geoipupdate

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/Rain-kl/Wavelet/internal/apps/agent/geoipdata"
	"github.com/Rain-kl/Wavelet/pkg/geoip"
)

const (
	mmdbDirPerm  = 0o750
	mmdbFilePerm = 0o600
)

// Updater periodically downloads a fresh GeoIP MMDB file and seeds the
// initial embedded database when none is present on disk.
type Updater struct {
	MMDBPath         string
	DownloadURL      string
	CityMMDBPath     string
	CityDownloadURL  string
	UpdateInterval   time.Duration
	downloadDatabase func(context.Context, string, string) error
}

// EnsureInitialDatabase seeds the Country MMDB file from the embedded database if it does not exist on disk.
func (u *Updater) EnsureInitialDatabase() error {
	return ensureEmbeddedDatabase(u.MMDBPath, geoipdata.DefaultMMDBName, "Country")
}

func ensureEmbeddedDatabase(targetPath string, embeddedName string, databaseName string) error {
	path := filepath.Clean(targetPath)
	if path == "" || path == "." {
		return nil
	}
	if _, err := os.Stat(path); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("stat mmdb file failed: %w", err)
	}
	data, err := fs.ReadFile(geoipdata.FS, embeddedName)
	if err != nil {
		return fmt.Errorf("read embedded %s mmdb failed: %w", databaseName, err)
	}
	if err := os.MkdirAll(filepath.Dir(path), mmdbDirPerm); err != nil {
		return fmt.Errorf("create mmdb directory failed: %w", err)
	}
	if err := os.WriteFile(path, data, mmdbFilePerm); err != nil {
		return fmt.Errorf("write initial mmdb failed: %w", err)
	}
	slog.Info("initialized GeoIP mmdb from embedded database", "database", databaseName, "path", path, "size", len(data))
	return nil
}

// EnsureInitialDatabases seeds both Country and City from embedded databases
// when either managed file is absent. Network downloads are reserved for the periodic updater.
func (u *Updater) EnsureInitialDatabases(_ context.Context) error {
	var errs []error
	if err := u.EnsureInitialDatabase(); err != nil {
		errs = append(errs, err)
	}
	if err := ensureEmbeddedDatabase(u.CityMMDBPath, geoipdata.DefaultCityMMDBName, "City"); err != nil {
		errs = append(errs, err)
	}
	return errors.Join(errs...)
}

func (u *Updater) download(ctx context.Context, path string, downloadURL string) error {
	if u.downloadDatabase != nil {
		return u.downloadDatabase(ctx, path, downloadURL)
	}
	return geoip.DownloadMaxMindDatabase(ctx, path, downloadURL)
}

func (u *Updater) updateDatabases(ctx context.Context) error {
	databases := []struct {
		name        string
		path        string
		downloadURL string
	}{
		{name: "Country", path: u.MMDBPath, downloadURL: u.DownloadURL},
		{name: "City", path: u.CityMMDBPath, downloadURL: u.CityDownloadURL},
	}
	var errs []error
	for _, database := range databases {
		if database.path == "" || (database.name == "City" && database.downloadURL == "") {
			continue
		}
		if err := u.download(ctx, database.path, database.downloadURL); err != nil {
			errs = append(errs, fmt.Errorf("update GeoIP %s mmdb failed: %w", database.name, err))
			continue
		}
		slog.Info("GeoIP mmdb updated", "database", database.name, "path", database.path)
	}
	return errors.Join(errs...)
}

// Run starts the periodic GeoIP update loop and blocks until ctx is cancelled.
func (u *Updater) Run(ctx context.Context) {
	if u == nil || u.MMDBPath == "" || u.UpdateInterval <= 0 {
		return
	}
	if err := u.EnsureInitialDatabases(ctx); err != nil {
		slog.Warn("initialize GeoIP databases failed", "country_path", u.MMDBPath, "city_path", u.CityMMDBPath, "error", err)
	}
	ticker := time.NewTicker(u.UpdateInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := u.updateDatabases(ctx); err != nil {
				slog.Warn("update GeoIP databases failed", "error", err)
			}
		}
	}
}
