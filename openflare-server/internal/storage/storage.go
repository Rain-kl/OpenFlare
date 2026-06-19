// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package storage

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/Rain-kl/Wavelet/internal/db"
	"github.com/Rain-kl/Wavelet/internal/model"
	"github.com/Rain-kl/Wavelet/internal/repository"
	"gorm.io/gorm"
)

const (
	defaultContentType = "application/octet-stream"
	storageDirPerm     = 0o750
	storageFilePerm    = 0o600
)

// Object describes a readable stored object.
type Object struct {
	CachePath     string
	Body          io.ReadCloser
	ContentLength int64
	ContentType   string
}

// PutResult describes the result of a successful Put operation.
type PutResult struct {
	Key    string
	Bucket string
}

// Backend defines storage operations used by the upload domain.
type Backend interface {
	Put(ctx context.Context, key string, body io.Reader, size int64, contentType string) (PutResult, error)
	Get(ctx context.Context, key string) (*Object, error)
	Delete(ctx context.Context, key string) error
	Test(ctx context.Context) error
}

var (
	// IsEnabledFunc preserves the legacy S3 test hook while tests migrate to backend injection.
	IsEnabledFunc = func() bool { return false }
	mockBackend   Backend

	activeBackend    Backend
	activeDriver     Driver
	activeConfigJSON string
	lastChecked      time.Time
	cacheMutex       sync.RWMutex
)

// ConfigInvalidationChannel is the Redis pub/sub channel used to evict storage caches cluster-wide.
const ConfigInvalidationChannel = "storage:config_invalidation"

var pubSubOnce sync.Once

// ResetCache clears the local cache for storage configuration and client singletons.
func ResetCache() {
	cacheMutex.Lock()
	defer cacheMutex.Unlock()
	activeBackend = nil
	activeDriver = ""
	activeConfigJSON = ""
	lastChecked = time.Time{}
}

// PublishCacheInvalidation broadcasts cache eviction to all nodes in the cluster via Redis.
func PublishCacheInvalidation(ctx context.Context) {
	if db.Redis != nil {
		_ = db.Redis.Publish(ctx, ConfigInvalidationChannel, "reset").Err()
	}
}

// startPubSubListener starts the background subscriber for cache invalidations.
func startPubSubListener() {
	if db.Redis == nil {
		return
	}
	go func() {
		pubsub := db.Redis.Subscribe(context.Background(), ConfigInvalidationChannel)
		defer func() {
			_ = pubsub.Close()
		}()

		ch := pubsub.Channel()
		for range ch {
			ResetCache()
		}
	}()
}

// Active returns the configured active driver and backend, using an in-memory cache with 5s TTL.
func Active(ctx context.Context) (Driver, Backend, error) {
	if IsEnabledFunc() && mockBackend != nil {
		return DriverS3, mockBackend, nil
	}

	pubSubOnce.Do(startPubSubListener)

	cacheMutex.RLock()
	isCacheValid := time.Since(lastChecked) < 5*time.Second && activeBackend != nil
	if isCacheValid {
		d, b := activeDriver, activeBackend
		cacheMutex.RUnlock()
		return d, b, nil
	}
	cacheMutex.RUnlock()

	cacheMutex.Lock()
	defer cacheMutex.Unlock()

	// Double-check under write lock
	if time.Since(lastChecked) < 5*time.Second && activeBackend != nil {
		return activeDriver, activeBackend, nil
	}

	sc, err := repository.GetSystemConfigByKey(ctx, model.ConfigKeyStorageConfig)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return "", nil, err
	}

	lastChecked = time.Now()

	// Reuse existing backend client singleton if configuration JSON matches
	if sc.Value == activeConfigJSON && activeBackend != nil {
		return activeDriver, activeBackend, nil
	}

	cfg := DefaultConfig()
	if strings.TrimSpace(sc.Value) != "" {
		if err := json.Unmarshal([]byte(sc.Value), &cfg); err != nil {
			return "", nil, fmt.Errorf("parse storage config: %w", err)
		}
	}

	backend, err := NewBackend(ctx, cfg, cfg.Driver)
	if err != nil {
		return "", nil, err
	}

	activeDriver = cfg.Driver
	activeBackend = backend
	activeConfigJSON = sc.Value

	return activeDriver, activeBackend, nil
}

type functionBackend struct {
	put    func(context.Context, string, io.Reader, int64, string) error
	get    func(context.Context, string) (*Object, error)
	delete func(context.Context, string) error
}

func (b *functionBackend) Put(ctx context.Context, key string, body io.Reader, size int64, contentType string) (PutResult, error) {
	if err := b.put(ctx, key, body, size, contentType); err != nil {
		return PutResult{}, err
	}
	return PutResult{Key: key}, nil
}

func (b *functionBackend) Get(ctx context.Context, key string) (*Object, error) {
	return b.get(ctx, key)
}

func (b *functionBackend) Delete(ctx context.Context, key string) error {
	return b.delete(ctx, key)
}

func (b *functionBackend) Test(context.Context) error {
	return nil
}

// MockStorage replaces object operations for package tests and returns a restore function.
func MockStorage(
	put func(context.Context, string, io.Reader, int64, string) error,
	get func(context.Context, string) (*Object, error),
	deleteObject func(context.Context, string) error,
) func() {
	previous := mockBackend
	mockBackend = &functionBackend{put: put, get: get, delete: deleteObject}
	return func() {
		mockBackend = previous
	}
}

// NewBackend constructs a concrete backend from configuration.
func NewBackend(ctx context.Context, cfg Config, driver Driver) (Backend, error) {
	if driver == DriverS3 && mockBackend != nil {
		return mockBackend, nil
	}
	switch driver {
	case DriverLocal:
		return newLocalBackend(cfg.Local)
	case DriverS3:
		return newS3Backend(ctx, cfg.S3)
	case DriverR2:
		return newR2Backend(ctx, cfg.R2)
	case DriverMinIO:
		return newS3Backend(ctx, cfg.MinIO)
	case DriverOSS:
		return newOSSBackend(cfg.OSS)
	case DriverWebDAV:
		return newWebDAVBackend(cfg.WebDAV)
	default:
		return nil, fmt.Errorf("unsupported storage driver %q", driver)
	}
}
