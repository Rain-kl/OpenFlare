// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package storage provides the object storage and ingestion infrastructure plugin for Cordis.
package storage

import (
	"Wavelet/core"
	"Wavelet/core/contracts"
	"Wavelet/plugins/infra/storage/diskcache"
	"Wavelet/plugins/infra/storage/objectstore"
	"context"
	"errors"
	"fmt"
	"io"
)

// Option configures the storage plugin.
type Option func(*Plugin)

// WithBackend sets an explicit storage backend instance (useful for testing or custom engines).
func WithBackend(b objectstore.Backend) Option {
	return func(p *Plugin) {
		p.backend = b
	}
}

// Plugin implements core.Plugin to provide contracts.StorageService.
type Plugin struct {
	backend objectstore.Backend
}

// New creates a new storage infrastructure plugin.
func New(opts ...Option) *Plugin {
	p := &Plugin{}
	for _, opt := range opts {
		if opt != nil {
			opt(p)
		}
	}
	return p
}

// Name returns the unique identifier of the storage plugin.
func (p *Plugin) Name() string {
	return "storage"
}

// Apply mounts the storage service into the Context.
func (p *Plugin) Apply(ctx *core.Context) error {
	core.Bind[contracts.DBService](ctx, func(db contracts.DBService) {
		objectstore.SetDBService(db)
		diskcache.SetDBService(db)
	})
	core.Bind[contracts.CacheService](ctx, objectstore.SetCacheService)

	ctx.OnDispose(func() error {
		objectstore.SetDBService(nil)
		diskcache.SetDBService(nil)
		objectstore.SetCacheService(nil)
		return nil
	})

	svc := &storageServiceImpl{
		backend: p.backend,
	}
	core.Provide[contracts.StorageService](ctx, svc)
	return nil
}

type storageServiceImpl struct {
	backend objectstore.Backend
}

func (s *storageServiceImpl) getBackend(ctx context.Context) (objectstore.Backend, error) {
	if s.backend != nil {
		return s.backend, nil
	}
	_, b, err := objectstore.Active(ctx)
	if err != nil {
		return nil, fmt.Errorf("storage: get active backend failed: %w", err)
	}
	return b, nil
}

func (s *storageServiceImpl) Put(ctx context.Context, key string, body io.Reader, size int64, contentType string) (contracts.StoragePutResult, error) {
	b, err := s.getBackend(ctx)
	if err != nil {
		return contracts.StoragePutResult{}, err
	}

	res, err := b.Put(ctx, key, body, size, contentType)
	if err != nil {
		return contracts.StoragePutResult{}, err
	}

	return contracts.StoragePutResult{
		Key:    res.Key,
		Bucket: res.Bucket,
	}, nil
}

func (s *storageServiceImpl) Get(ctx context.Context, key string) (*contracts.StorageObject, error) {
	b, err := s.getBackend(ctx)
	if err != nil {
		return nil, err
	}

	obj, err := b.Get(ctx, key)
	if err != nil {
		return nil, err
	}

	return &contracts.StorageObject{
		Key:           key,
		CachePath:     obj.CachePath,
		Body:          obj.Body,
		ContentLength: obj.ContentLength,
		ContentType:   obj.ContentType,
	}, nil
}

func (s *storageServiceImpl) Delete(ctx context.Context, key string) error {
	b, err := s.getBackend(ctx)
	if err != nil {
		return err
	}
	return b.Delete(ctx, key)
}

func (s *storageServiceImpl) Ingest(_ context.Context, _ io.Reader, _ contracts.IngestOptions) (*contracts.IngestResult, error) {
	return nil, errors.New("storage: programmatic ingest is managed by domain/upload plugin")
}
