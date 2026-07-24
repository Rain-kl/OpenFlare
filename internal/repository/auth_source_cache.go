// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package repository

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	db "github.com/Rain-kl/Wavelet/internal/infra/persistence"
	"github.com/Rain-kl/Wavelet/internal/model"
	"github.com/Rain-kl/Wavelet/pkg/cache/ram"
)

const (
	authSourceActiveRedisKey      = "oauth:auth_sources:active"
	authSourceByNameRedisKeyFmt   = "oauth:auth_sources:by_name:%s"
	authSourceByNameRedisPattern  = "oauth:auth_sources:by_name:*"
	authSourceActiveRAMKey        = "active"
	authSourceCacheTTL            = time.Hour
	authSourceRAMMaximumSize      = 64
	authSourceInvalidationChannel = "oauth:auth_source_invalidation"
)

// authSourceRedisRecord persists full auth source credentials in Redis.
type authSourceRedisRecord struct {
	ID                     uint64    `json:"id"`
	Name                   string    `json:"name"`
	Type                   string    `json:"type"`
	DisplayName            string    `json:"display_name"`
	IsActive               bool      `json:"is_active"`
	ClientID               string    `json:"client_id"`
	ClientSecret           string    `json:"client_secret"`
	OpenIDDiscoveryURL     string    `json:"openid_discovery_url"`
	Scopes                 string    `json:"scopes"`
	IconURL                string    `json:"icon_url"`
	CreatedAt              time.Time `json:"created_at"`
	UpdatedAt              time.Time `json:"updated_at"`
	ClientSecretConfigured bool      `json:"client_secret_configured"`
}

var (
	authSourceActiveRAM      = ram.MustNew[string, []model.AuthSource](ram.Options{MaximumSize: authSourceRAMMaximumSize})
	authSourceByNameRAM      = ram.MustNew[string, model.AuthSource](ram.Options{MaximumSize: authSourceRAMMaximumSize})
	authSourceListenerOnce   sync.Once
	authSourceListenerCtx    context.Context
	authSourceListenerCancel context.CancelFunc
)

func cloneAuthSources(sources []model.AuthSource) []model.AuthSource {
	if len(sources) == 0 {
		return nil
	}
	cloned := make([]model.AuthSource, len(sources))
	copy(cloned, sources)
	return cloned
}

func cloneAuthSource(source model.AuthSource) model.AuthSource {
	return source
}

func normalizeAuthSourceName(name string) string {
	return strings.TrimSpace(strings.ToLower(name))
}

func authSourceByNameRedisKey(name string) string {
	return fmt.Sprintf(authSourceByNameRedisKeyFmt, normalizeAuthSourceName(name))
}

func authSourceToRedisRecord(source model.AuthSource) authSourceRedisRecord {
	return authSourceRedisRecord{
		ID:                     source.ID,
		Name:                   source.Name,
		Type:                   source.Type,
		DisplayName:            source.DisplayName,
		IsActive:               source.IsActive,
		ClientID:               source.ClientID,
		ClientSecret:           source.ClientSecret,
		OpenIDDiscoveryURL:     source.OpenIDDiscoveryURL,
		Scopes:                 source.Scopes,
		IconURL:                source.IconURL,
		CreatedAt:              source.CreatedAt,
		UpdatedAt:              source.UpdatedAt,
		ClientSecretConfigured: source.ClientSecretConfigured,
	}
}

func redisRecordToAuthSource(record authSourceRedisRecord) model.AuthSource {
	return model.AuthSource{
		ID:                     record.ID,
		Name:                   record.Name,
		Type:                   record.Type,
		DisplayName:            record.DisplayName,
		IsActive:               record.IsActive,
		ClientID:               record.ClientID,
		ClientSecret:           record.ClientSecret,
		OpenIDDiscoveryURL:     record.OpenIDDiscoveryURL,
		Scopes:                 record.Scopes,
		IconURL:                record.IconURL,
		CreatedAt:              record.CreatedAt,
		UpdatedAt:              record.UpdatedAt,
		ClientSecretConfigured: record.ClientSecretConfigured,
	}
}

func ensureAuthSourceCacheListener() {
	if db.Redis == nil {
		return
	}
	authSourceListenerOnce.Do(startAuthSourceCacheInvalidationListener)
}

func startAuthSourceCacheInvalidationListener() {
	authSourceListenerCtx, authSourceListenerCancel = context.WithCancel(context.Background())

	go func() {
		pubsub := db.Redis.Subscribe(authSourceListenerCtx, authSourceInvalidationChannel)
		defer func() {
			_ = pubsub.Close()
		}()

		go func() {
			<-authSourceListenerCtx.Done()
			_ = pubsub.Close()
		}()

		for range pubsub.Channel() {
			authSourceActiveRAM.InvalidateAll()
			authSourceByNameRAM.InvalidateAll()
		}
	}()
}

func publishAuthSourceRAMInvalidation(ctx context.Context) {
	if db.Redis == nil {
		return
	}
	_ = db.Redis.Publish(ctx, authSourceInvalidationChannel, "reset").Err()
}

func populateActiveAuthSourceCache(ctx context.Context, sources []model.AuthSource) {
	cloned := cloneAuthSources(sources)
	authSourceActiveRAM.Set(authSourceActiveRAMKey, cloned)
	if db.Redis != nil {
		_ = db.SetJSON(ctx, authSourceActiveRedisKey, cloned, authSourceCacheTTL)
	}
}

func populateAuthSourceByNameCache(ctx context.Context, name string, source *model.AuthSource) {
	if source == nil {
		return
	}
	cloned := cloneAuthSource(*source)
	authSourceByNameRAM.Set(normalizeAuthSourceName(name), cloned)
	if db.Redis != nil {
		record := authSourceToRedisRecord(cloned)
		_ = db.SetJSON(ctx, authSourceByNameRedisKey(name), record, authSourceCacheTTL)
	}
}

// GetActiveAuthSourcesCached returns active auth sources from RAM, Redis, or the database.
func GetActiveAuthSourcesCached(ctx context.Context) ([]model.AuthSource, error) {
	ensureAuthSourceCacheListener()

	if sources, ok := authSourceActiveRAM.GetIfPresent(authSourceActiveRAMKey); ok {
		return cloneAuthSources(sources), nil
	}

	if db.Redis != nil {
		var sources []model.AuthSource
		if err := db.GetJSON(ctx, authSourceActiveRedisKey, &sources); err == nil {
			populateActiveAuthSourceCache(ctx, sources)
			return cloneAuthSources(sources), nil
		}
	}

	sources, err := model.GetActiveAuthSources(ctx)
	if err != nil {
		return nil, err
	}
	populateActiveAuthSourceCache(ctx, sources)
	return cloneAuthSources(sources), nil
}

// GetAuthSourceByNameCached returns an auth source by name from RAM, Redis, or the database.
func GetAuthSourceByNameCached(ctx context.Context, name string) (*model.AuthSource, error) {
	ensureAuthSourceCacheListener()

	normalized := normalizeAuthSourceName(name)
	if normalized == "" {
		return model.GetAuthSourceByName(ctx, name)
	}

	if source, ok := authSourceByNameRAM.GetIfPresent(normalized); ok {
		cloned := cloneAuthSource(source)
		return &cloned, nil
	}

	if db.Redis != nil {
		var record authSourceRedisRecord
		if err := db.GetJSON(ctx, authSourceByNameRedisKey(name), &record); err == nil {
			source := redisRecordToAuthSource(record)
			populateAuthSourceByNameCache(ctx, name, &source)
			cloned := cloneAuthSource(source)
			return &cloned, nil
		}
	}

	source, err := model.GetAuthSourceByName(ctx, name)
	if err != nil {
		return nil, err
	}
	populateAuthSourceByNameCache(ctx, name, source)
	cloned := cloneAuthSource(*source)
	return &cloned, nil
}

// InvalidateAuthSourceCache clears active and per-name auth source caches from RAM and Redis.
func InvalidateAuthSourceCache(ctx context.Context) error {
	ensureAuthSourceCacheListener()

	authSourceActiveRAM.InvalidateAll()
	authSourceByNameRAM.InvalidateAll()

	if db.Redis == nil {
		return nil
	}

	if err := db.Redis.Del(ctx, db.PrefixedKey(authSourceActiveRedisKey)).Err(); err != nil {
		return err
	}

	pattern := db.PrefixedKey(authSourceByNameRedisPattern)
	iter := db.Redis.Scan(ctx, 0, pattern, 0).Iterator()
	var keys []string
	for iter.Next(ctx) {
		keys = append(keys, iter.Val())
	}
	if err := iter.Err(); err != nil {
		return err
	}
	if len(keys) > 0 {
		if err := db.Redis.Del(ctx, keys...).Err(); err != nil {
			return err
		}
	}

	publishAuthSourceRAMInvalidation(ctx)
	return nil
}

// StopAuthSourceCacheListener stops the Redis Pub/Sub subscription listener and resets the sync.Once guard.
func StopAuthSourceCacheListener() {
	if authSourceListenerCancel != nil {
		authSourceListenerCancel()
		authSourceListenerCancel = nil
	}
	authSourceListenerOnce = sync.Once{}
}

// ResetAuthSourceRAMCacheForTest clears only the process-local RAM cache.
func ResetAuthSourceRAMCacheForTest() {
	authSourceActiveRAM.InvalidateAll()
	authSourceByNameRAM.InvalidateAll()
}
