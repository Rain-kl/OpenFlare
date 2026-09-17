// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package dao provides data access objects and caching for the auth domain plugin.
package dao

import (
	"Wavelet/core/contracts"
	"context"

	"gorm.io/gorm"
)

// DAO aggregates all data access objects for the auth domain plugin.
type DAO struct {
	dbSvc      contracts.DBService
	cacheSvc   contracts.CacheService
	limiterSvc contracts.LimiterService
}

// New creates a new DAO aggregate.
func New(dbSvc contracts.DBService, cacheSvc contracts.CacheService, limiterSvc contracts.LimiterService) *DAO {
	return &DAO{
		dbSvc:      dbSvc,
		cacheSvc:   cacheSvc,
		limiterSvc: limiterSvc,
	}
}

// SetDBService updates the DBService reference.
func (d *DAO) SetDBService(db contracts.DBService) {
	d.dbSvc = db
}

// SetCacheService updates the CacheService reference.
func (d *DAO) SetCacheService(cache contracts.CacheService) {
	d.cacheSvc = cache
}

// SetLimiterService updates the LimiterService reference.
func (d *DAO) SetLimiterService(limiter contracts.LimiterService) {
	d.limiterSvc = limiter
}

// DB returns the GORM DB instance associated with the request context.
func (d *DAO) DB(ctx context.Context) *gorm.DB {
	if d.dbSvc != nil {
		return d.dbSvc.DB(ctx)
	}
	return nil
}

// Cache returns the CacheService instance.
func (d *DAO) Cache() contracts.CacheService {
	return d.cacheSvc
}

// Limiter returns the LimiterService instance.
func (d *DAO) Limiter() contracts.LimiterService {
	return d.limiterSvc
}
