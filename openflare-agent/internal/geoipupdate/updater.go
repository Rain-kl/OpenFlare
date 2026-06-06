package geoipupdate

import (
	"context"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/rain-kl/openflare/openflare-agent/internal/geoipdata"
	"github.com/rain-kl/openflare/openflare-server/utils/geoip"
)

type Updater struct {
	MMDBPath       string
	DownloadURL    string
	UpdateInterval time.Duration
}

func (u *Updater) EnsureInitialDatabase() error {
	path := filepath.Clean(u.MMDBPath)
	if path == "" || path == "." {
		return nil
	}
	if _, err := os.Stat(path); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("stat mmdb file failed: %w", err)
	}
	data, err := fs.ReadFile(geoipdata.FS, geoipdata.DefaultMMDBName)
	if err != nil {
		return fmt.Errorf("read embedded mmdb failed: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create mmdb directory failed: %w", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write initial mmdb failed: %w", err)
	}
	slog.Info("initialized GeoIP mmdb from embedded database", "path", path, "size", len(data))
	return nil
}

func (u *Updater) Run(ctx context.Context) {
	if u == nil || u.MMDBPath == "" || u.UpdateInterval <= 0 {
		return
	}
	if err := u.EnsureInitialDatabase(); err != nil {
		slog.Warn("initialize GeoIP mmdb failed", "path", u.MMDBPath, "error", err)
	}
	ticker := time.NewTicker(u.UpdateInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := geoip.DownloadMaxMindDatabase(u.MMDBPath, u.DownloadURL); err != nil {
				slog.Warn("update GeoIP mmdb failed", "path", u.MMDBPath, "error", err)
				continue
			}
			slog.Info("GeoIP mmdb updated", "path", u.MMDBPath)
		}
	}
}
