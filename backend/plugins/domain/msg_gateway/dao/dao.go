// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package dao provides database persistence and caching for the msg_gateway plugin.
package dao

import (
	"Wavelet/core"
	"Wavelet/core/contracts"
	"Wavelet/plugins/domain/msg_gateway/consts"
	"context"
	"errors"
	"sync"

	"gorm.io/gorm"
)

var (
	dbMu     sync.RWMutex
	dbSvc    contracts.DBService
	cacheMu  sync.RWMutex
	cacheSvc contracts.CacheService
)

// SetDBService sets the database service singleton.
func SetDBService(s contracts.DBService) {
	dbMu.Lock()
	defer dbMu.Unlock()
	dbSvc = s
}

// SetDBServiceForTest injects a DBService for tests.
func SetDBServiceForTest(s contracts.DBService) {
	SetDBService(s)
}

// SetCacheService sets the cache service singleton.
func SetCacheService(s contracts.CacheService) {
	cacheMu.Lock()
	defer cacheMu.Unlock()
	cacheSvc = s
}

// GetDB resolves the persistence handle for the current call.
func GetDB(ctx context.Context) *gorm.DB {
	if c, ok := ctx.(*core.Context); ok && c != nil {
		if s, err := core.Inject[contracts.DBService](c); err == nil && s != nil {
			return s.DB(ctx)
		}
	}
	dbMu.RLock()
	s := dbSvc
	dbMu.RUnlock()
	if s != nil {
		return s.DB(ctx)
	}
	return nil
}

// GetCache resolves the cache service for the current call.
func GetCache(ctx context.Context) contracts.CacheService {
	if c, ok := ctx.(*core.Context); ok && c != nil {
		if s, err := core.Inject[contracts.CacheService](c); err == nil && s != nil {
			return s
		}
	}
	cacheMu.RLock()
	s := cacheSvc
	cacheMu.RUnlock()
	return s
}

// mapNotFound translates GORM's missing-row sentinel into the plugin-level
// consts.ErrRecordNotFound so the service and controller layers stay free of gorm imports.
func mapNotFound(err error) error {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return consts.ErrRecordNotFound
	}
	return err
}
