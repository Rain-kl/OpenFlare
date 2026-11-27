// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package netintel manages runtime loading, hot reload, synchronization, and queries
// for OpenFlare's network intelligence databases (GeoIP/ASN MMDB, bgp.tools, optional CAIDA).
package netintel

import (
	"context"
	"io/fs"
	"strings"
	"sync"

	geodata "github.com/Rain-kl/Wavelet/internal/apps/openflare/geoip/data"
	db "github.com/Rain-kl/Wavelet/internal/infra/persistence"
	"github.com/Rain-kl/Wavelet/internal/model"
	"github.com/Rain-kl/Wavelet/internal/platform/lifecycle"
	"github.com/Rain-kl/Wavelet/internal/repository"
	"github.com/Rain-kl/Wavelet/pkg/logger"
	pkgnetintel "github.com/Rain-kl/Wavelet/pkg/netintel"
	"github.com/google/uuid"
)

const (
	defaultDataDir      = "data/netintel"
	reloadPubSubChannel = "netintel:reloaded"
)

var (
	managerMu      sync.RWMutex
	defaultManager = pkgnetintel.NewManager()
	initOnce       sync.Once
	reloadSubOnce  sync.Once
	instancePubID  = uuid.NewString()
	lastInitErr    error
)

func currentManager() *pkgnetintel.Manager {
	managerMu.RLock()
	mgr := defaultManager
	managerMu.RUnlock()
	if mgr != nil {
		return mgr
	}
	return pkgnetintel.NewManager()
}

// SetManagerForTest swaps the active NetIntel manager for tests and returns a restore callback.
func SetManagerForTest(mgr *pkgnetintel.Manager) func() {
	managerMu.Lock()
	previous := defaultManager
	if mgr == nil {
		defaultManager = pkgnetintel.NewManager()
	} else {
		defaultManager = mgr
	}
	managerMu.Unlock()
	return func() {
		managerMu.Lock()
		defaultManager = previous
		managerMu.Unlock()
	}
}

func resolveDataDir(ctx context.Context) string {
	if db.DB(ctx) != nil {
		cfg, err := repository.GetSystemConfigByKey(ctx, model.ConfigKeyNetIntelDataDir)
		if err == nil && strings.TrimSpace(cfg.Value) != "" {
			return strings.TrimSpace(cfg.Value)
		}
	}
	return defaultDataDir
}

func readFallbackCountryMMDB() []byte {
	raw, err := fs.ReadFile(geodata.FS, geodata.DefaultMMDBName)
	if err != nil {
		return nil
	}
	return raw
}

// Init loads NetIntel databases from the configured directory (with embedded GeoIP fallback)
// and subscribes to Redis hot-reload notifications across controller replicas.
func Init(ctx context.Context) error {
	initOnce.Do(func() {
		dir := resolveDataDir(ctx)
		if err := currentManager().LoadFromDir(dir, readFallbackCountryMMDB()); err != nil {
			lastInitErr = err
			logger.WarnF(ctx, "[NetIntel] initial load from %q failed: %v", dir, err)
		} else {
			logger.InfoF(ctx, "[NetIntel] loaded datasets from %q (%d entries tracked)", dir, len(Statuses()))
		}
		startReloadSubscriber(ctx)
	})
	return lastInitErr
}

// Reload reloads the active NetIntel databases from disk and broadcasts the update over Redis.
func Reload(ctx context.Context) error {
	if err := reloadLocal(ctx, resolveDataDir(ctx)); err != nil {
		return err
	}
	if db.Redis != nil {
		if err := db.Redis.Publish(ctx, db.PrefixedKey(reloadPubSubChannel), instancePubID).Err(); err != nil {
			logger.WarnF(ctx, "[NetIntel] failed to publish reload notification: %v", err)
		}
	}
	return nil
}

// ReloadFromDir reloads datasets from an explicit directory without broadcasting over Redis.
func ReloadFromDir(ctx context.Context, dir string) error {
	return reloadLocal(ctx, dir)
}

func reloadLocal(ctx context.Context, dir string) error {
	cleanDir := strings.TrimSpace(dir)
	if cleanDir == "" {
		cleanDir = defaultDataDir
	}
	if err := currentManager().LoadFromDir(cleanDir, readFallbackCountryMMDB()); err != nil {
		logger.WarnF(ctx, "[NetIntel] reload from %q failed: %v", cleanDir, err)
		return err
	}
	logger.InfoF(ctx, "[NetIntel] reloaded datasets from %q", cleanDir)
	return nil
}

func startReloadSubscriber(ctx context.Context) {
	client := db.Redis
	if client == nil {
		return
	}
	reloadSubOnce.Do(func() {
		subCtx, cancel := context.WithCancel(context.WithoutCancel(ctx))
		channel := db.PrefixedKey(reloadPubSubChannel)
		sub := client.Subscribe(subCtx, channel)
		lifecycle.OnShutdown("netintel dataset reload subscriber", func(context.Context) error {
			cancel()
			return sub.Close()
		})
		go func() {
			defer func() {
				if r := recover(); r != nil {
					logger.ErrorF(subCtx, "[NetIntel] reload subscriber panic recovered: %v", r)
				}
			}()
			for msg := range sub.Channel() {
				if strings.TrimSpace(msg.Payload) == instancePubID {
					continue
				}
				_ = reloadLocal(subCtx, resolveDataDir(subCtx))
			}
		}()
	})
}

// LookupIP resolves geolocation, ASN, tags, and BGP routing prefix metadata for rawIP.
func LookupIP(rawIP string) *pkgnetintel.IPLookupResult {
	return currentManager().LookupIP(rawIP)
}

// LookupASN returns metadata and prefix counts for asn.
func LookupASN(asn uint32) *pkgnetintel.ASNInfo {
	return currentManager().LookupASN(asn)
}

// ListASNPrefixes returns paginated prefixes announced by asn.
func ListASNPrefixes(asn uint32, family string, page, pageSize int) ([]pkgnetintel.PrefixRecord, int) {
	return currentManager().ListASNPrefixes(asn, family, page, pageSize)
}

// LookupASNeighbors returns CAIDA-derived neighbor relationships enriched with bgp.tools metadata.
func LookupASNeighbors(asn uint32, limit int) []pkgnetintel.ASRelation {
	return currentManager().LookupASNeighbors(asn, limit)
}

// EstimateASPath computes a shortest AS hop path merging observed traceroute edges and CAIDA relationships.
func EstimateASPath(srcASN, dstASN uint32, observedAdj map[uint32][]uint32) (int, []uint32, bool) {
	return currentManager().EstimateASPath(srcASN, dstASN, observedAdj)
}

// Statuses returns status metadata for every tracked dataset file.
func Statuses() []pkgnetintel.DatasetFileStatus {
	return currentManager().Statuses()
}

// DataDir returns the currently loaded NetIntel directory path.
func DataDir() string {
	return currentManager().DataDir()
}

// EnrichNodeASN populates ASN and ASNOrg on node using the active NetIntel databases.
// Returns true when either field changed.
func EnrichNodeASN(node *model.OpenFlareNode) bool {
	if node == nil {
		return false
	}
	ip := strings.TrimSpace(node.IP)
	if ip == "" {
		return false
	}
	res := LookupIP(ip)
	if res == nil || res.ASN == 0 {
		return false
	}
	org := strings.TrimSpace(res.ASNName)
	if node.ASN == res.ASN && strings.TrimSpace(node.ASNOrg) == org {
		return false
	}
	node.ASN = res.ASN
	node.ASNOrg = org
	return true
}
