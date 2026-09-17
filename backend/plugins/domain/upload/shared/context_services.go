// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package shared

import (
	"Wavelet/core"
	"Wavelet/core/contracts"
	"context"
	"sync"

	"gorm.io/gorm"
)

var (
	svcMu      sync.RWMutex
	dbSvc      contracts.DBService
	cacheSvc   contracts.CacheService
	storageSvc contracts.StorageService
	taskSvc    contracts.TaskService
	authSvc    contracts.AuthService
)

// SetDBService configures the DBService.
func SetDBService(s contracts.DBService) {
	svcMu.Lock()
	defer svcMu.Unlock()
	dbSvc = s
}

// SetCacheService configures the CacheService.
func SetCacheService(s contracts.CacheService) {
	svcMu.Lock()
	defer svcMu.Unlock()
	cacheSvc = s
}

// SetStorageService configures the StorageService.
func SetStorageService(s contracts.StorageService) {
	svcMu.Lock()
	defer svcMu.Unlock()
	storageSvc = s
}

// SetTaskService configures the TaskService.
func SetTaskService(s contracts.TaskService) {
	svcMu.Lock()
	defer svcMu.Unlock()
	taskSvc = s
}

// SetAuthService configures the AuthService.
func SetAuthService(s contracts.AuthService) {
	svcMu.Lock()
	defer svcMu.Unlock()
	authSvc = s
}

// ResetServices clears all injected services.
func ResetServices() {
	svcMu.Lock()
	defer svcMu.Unlock()
	dbSvc = nil
	cacheSvc = nil
	storageSvc = nil
	taskSvc = nil
	authSvc = nil
}

// GetDB resolves the GORM DB instance.
func GetDB(ctx context.Context) *gorm.DB {
	if s, err := core.InjectFrom[contracts.DBService](ctx); err == nil && s != nil {
		return s.DB(ctx)
	}
	svcMu.RLock()
	s := dbSvc
	svcMu.RUnlock()
	if s != nil {
		return s.DB(ctx)
	}
	return nil
}

// GetCache resolves the CacheService instance.
func GetCache(ctx context.Context) contracts.CacheService {
	if s, err := core.InjectFrom[contracts.CacheService](ctx); err == nil && s != nil {
		return s
	}
	svcMu.RLock()
	s := cacheSvc
	svcMu.RUnlock()
	return s
}

// GetStorage resolves the StorageService instance.
func GetStorage(ctx context.Context) contracts.StorageService {
	if s, err := core.InjectFrom[contracts.StorageService](ctx); err == nil && s != nil {
		return s
	}
	svcMu.RLock()
	s := storageSvc
	svcMu.RUnlock()
	return s
}

// GetTaskService resolves the TaskService instance.
func GetTaskService(ctx context.Context) contracts.TaskService {
	if s, err := core.InjectFrom[contracts.TaskService](ctx); err == nil && s != nil {
		return s
	}
	svcMu.RLock()
	defer svcMu.RUnlock()
	return taskSvc
}

// GetAuthService resolves the AuthService instance.
func GetAuthService(ctx context.Context) contracts.AuthService {
	if s, err := core.InjectFrom[contracts.AuthService](ctx); err == nil && s != nil {
		return s
	}
	svcMu.RLock()
	s := authSvc
	svcMu.RUnlock()
	return s
}
