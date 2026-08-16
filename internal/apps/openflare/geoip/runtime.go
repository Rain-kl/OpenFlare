// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package geoip

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"

	geodata "github.com/Rain-kl/Wavelet/internal/apps/openflare/geoip/data"
	"github.com/Rain-kl/Wavelet/internal/model"
	"github.com/Rain-kl/Wavelet/internal/repository"
	pkggeoip "github.com/Rain-kl/Wavelet/pkg/geoip"
	"github.com/Rain-kl/Wavelet/pkg/logger"
)

const (
	serverMMDBRelativePath = "data/GeoLite2-Country.mmdb"
	serverMMDBDirPerm      = 0o750
	serverMMDBFilePerm     = 0o644
)

var (
	runtimeOnce       sync.Once
	errRuntimeInit    error
	currentProviderMu sync.RWMutex
	currentProvider   string
)

// EnsureRuntimeProvider loads GeoIP provider config from SystemConfig.
func EnsureRuntimeProvider(ctx context.Context) error {
	runtimeOnce.Do(func() {
		errRuntimeInit = applyProviderFromSystemConfig(ctx)
	})
	return errRuntimeInit
}

// RefreshRuntimeProvider reapplies GeoIPProvider after config updates.
func RefreshRuntimeProvider(ctx context.Context) error {
	return applyProviderFromSystemConfig(ctx)
}

func applyProviderFromSystemConfig(ctx context.Context) error {
	config, err := repository.GetSystemConfigByKey(ctx, model.ConfigKeyGeoIPProvider)
	if err != nil {
		// 降级到默认值
		return ApplyProvider(ctx, "ipinfo")
	}
	provider := strings.TrimSpace(config.Value)
	if provider == "" {
		provider = "ipinfo"
	}
	return ApplyProvider(ctx, provider)
}

// ApplyProvider switches the process-wide GeoIP backend.
func ApplyProvider(ctx context.Context, provider string) error {
	normalized := strings.TrimSpace(strings.ToLower(provider))
	if normalized == "" {
		normalized = pkggeoip.ProviderDisabled
	}

	currentProviderMu.Lock()
	if currentProvider == normalized {
		currentProviderMu.Unlock()
		return nil
	}
	currentProvider = normalized
	currentProviderMu.Unlock()

	if normalized == pkggeoip.ProviderMaxMind {
		path, err := ensureServerMMDB()
		if err != nil {
			logger.WarnF(ctx, "[GeoIP] seed MaxMind database failed: %v", err)
		}
		if path != "" {
			pkggeoip.GeoIPFilePath = path
		}
	}

	pkggeoip.InitGeoIP(normalized)
	return nil
}

func ensureServerMMDB() (string, error) {
	path, err := filepath.Abs(serverMMDBRelativePath)
	if err != nil {
		return "", err
	}
	_, statErr := os.Stat(path)
	if statErr == nil {
		return path, nil
	}
	if !os.IsNotExist(statErr) {
		return "", statErr
	}

	// Control plane: seed Country from embedded asset (no City; Agent uses disk/image).
	data, err := fs.ReadFile(geodata.FS, geodata.DefaultMMDBName)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(path), serverMMDBDirPerm); err != nil {
		return "", err
	}
	if err := os.WriteFile(path, data, serverMMDBFilePerm); err != nil { //nolint:gosec // world-readable mmdb
		return "", err
	}
	return path, nil
}

// ResetRuntimeForTest clears lazy-init state for unit tests.
func ResetRuntimeForTest() {
	runtimeOnce = sync.Once{}
	errRuntimeInit = nil
	currentProviderMu.Lock()
	currentProvider = ""
	currentProviderMu.Unlock()
}
