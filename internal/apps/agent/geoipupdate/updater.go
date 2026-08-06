// Package geoipupdate schedules local MaxMind GeoIP database updates for the agent.
package geoipupdate

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/Rain-kl/Wavelet/pkg/geoip"
)

// Updater periodically downloads a fresh GeoIP MMDB file and seeds missing
// databases via download (or relies on image-provided files under data_dir).
type Updater struct {
	MMDBPath         string
	DownloadURL      string
	CityMMDBPath     string
	CityDownloadURL  string
	UpdateInterval   time.Duration
	downloadDatabase func(context.Context, string, string) error
}

// EnsureInitialDatabases downloads any missing Country/City MMDB once.
// When files already exist (e.g. Docker image COPY), this is a no-op.
// Network is used only when a managed path is absent — not for binary embeds.
func (u *Updater) EnsureInitialDatabases(ctx context.Context) error {
	if u == nil {
		return nil
	}
	return u.ensureMissingDatabases(ctx)
}

func (u *Updater) ensureMissingDatabases(ctx context.Context) error {
	databases := u.managedDatabases()
	var errs []error
	for _, database := range databases {
		if database.path == "" || database.downloadURL == "" {
			continue
		}
		exists, err := fileExists(database.path)
		if err != nil {
			errs = append(errs, fmt.Errorf("stat GeoIP %s mmdb failed: %w", database.name, err))
			continue
		}
		if exists {
			continue
		}
		if err := u.download(ctx, database.path, database.downloadURL); err != nil {
			errs = append(errs, fmt.Errorf("seed GeoIP %s mmdb failed: %w", database.name, err))
			continue
		}
		slog.Info("seeded GeoIP mmdb via download", "database", database.name, "path", database.path)
	}
	return errors.Join(errs...)
}

func fileExists(path string) (bool, error) {
	path = filepath.Clean(path)
	if path == "" || path == "." {
		return false, nil
	}
	info, err := os.Stat(path)
	if err == nil {
		if !info.Mode().IsRegular() {
			return false, fmt.Errorf("GeoIP MMDB path is not a regular file: %s", path)
		}
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

func (u *Updater) download(ctx context.Context, path string, downloadURL string) error {
	if u.downloadDatabase != nil {
		return u.downloadDatabase(ctx, path, downloadURL)
	}
	return geoip.DownloadMaxMindDatabase(ctx, path, downloadURL)
}

func (u *Updater) managedDatabases() []struct {
	name        string
	path        string
	downloadURL string
} {
	return []struct {
		name        string
		path        string
		downloadURL string
	}{
		{name: "Country", path: u.MMDBPath, downloadURL: u.DownloadURL},
		{name: "City", path: u.CityMMDBPath, downloadURL: u.CityDownloadURL},
	}
}

func (u *Updater) updateDatabases(ctx context.Context) error {
	var errs []error
	for _, database := range u.managedDatabases() {
		if database.path == "" || database.downloadURL == "" {
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
