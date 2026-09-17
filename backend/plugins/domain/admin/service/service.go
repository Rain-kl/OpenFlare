// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package service provides business logic and orchestration for the admin domain.
package service

import (
	"Wavelet/core"
	"Wavelet/core/contracts"
	"Wavelet/plugins/domain/admin/errs"
	"Wavelet/plugins/domain/admin/repository"
	"context"
	"errors"
	"sync"

	"gorm.io/gorm"
)

var (
	servicesMu         sync.RWMutex
	dbService          contracts.DBService
	cacheService       contracts.CacheService
	userService        contracts.UserService
	authService        contracts.AuthService
	taskService        contracts.TaskService
	storageSvc         contracts.StorageService
	riskControlService contracts.RiskControlService
	eventEmitter       func(ctx context.Context, topic string, payload any) error
)

// SetDBService injects the DBService contract.
func SetDBService(s contracts.DBService) {
	servicesMu.Lock()
	defer servicesMu.Unlock()
	dbService = s
	repository.SetDBService(s)
}

// SetCacheService injects the CacheService contract.
func SetCacheService(s contracts.CacheService) {
	servicesMu.Lock()
	defer servicesMu.Unlock()
	cacheService = s
	repository.SetCacheService(s)
}

// SetUserService injects the UserService contract.
func SetUserService(s contracts.UserService) {
	servicesMu.Lock()
	defer servicesMu.Unlock()
	userService = s
}

// SetAuthService injects the AuthService contract.
func SetAuthService(s contracts.AuthService) {
	servicesMu.Lock()
	defer servicesMu.Unlock()
	authService = s
}

// SetTaskService injects the TaskService contract.
func SetTaskService(s contracts.TaskService) {
	servicesMu.Lock()
	defer servicesMu.Unlock()
	taskService = s
}

// SetStorageService injects the StorageService contract.
func SetStorageService(s contracts.StorageService) {
	servicesMu.Lock()
	defer servicesMu.Unlock()
	storageSvc = s
}

// SetRiskControlService injects the RiskControlService contract.
func SetRiskControlService(s contracts.RiskControlService) {
	servicesMu.Lock()
	defer servicesMu.Unlock()
	riskControlService = s
}

// SetEventEmitter sets the event emission callback.
func SetEventEmitter(fn func(ctx context.Context, topic string, payload any) error) {
	servicesMu.Lock()
	defer servicesMu.Unlock()
	eventEmitter = fn
}

// EmitEvent publishes a domain event if an emitter is registered.
func EmitEvent(ctx context.Context, topic string, payload any) error {
	servicesMu.RLock()
	defer servicesMu.RUnlock()
	if eventEmitter == nil {
		return nil
	}
	return eventEmitter(ctx, topic, payload)
}

// ResetServices clears all injected services (used on disposal and testing).
func ResetServices() {
	servicesMu.Lock()
	defer servicesMu.Unlock()
	dbService = nil
	cacheService = nil
	userService = nil
	authService = nil
	taskService = nil
	storageSvc = nil
	riskControlService = nil
	eventEmitter = nil
	repository.ResetServices()
}

// GetDB returns the GORM DB instance bound to the context if available.
func GetDB(ctx context.Context) *gorm.DB {
	if s, err := core.InjectFrom[contracts.DBService](ctx); err == nil && s != nil {
		return s.DB(ctx)
	}
	servicesMu.RLock()
	defer servicesMu.RUnlock()
	if dbService == nil {
		return nil
	}
	return dbService.DB(ctx)
}

// GetCache returns the unified CacheService instance.
func GetCache(ctx context.Context) contracts.CacheService {
	if s, err := core.InjectFrom[contracts.CacheService](ctx); err == nil && s != nil {
		return s
	}
	servicesMu.RLock()
	defer servicesMu.RUnlock()
	return cacheService
}

// GetUserService returns the UserService instance.
func GetUserService(ctx context.Context) contracts.UserService {
	if s, err := core.InjectFrom[contracts.UserService](ctx); err == nil && s != nil {
		return s
	}
	servicesMu.RLock()
	defer servicesMu.RUnlock()
	return userService
}

// GetAuthService returns the AuthService instance.
func GetAuthService(ctx context.Context) contracts.AuthService {
	if s, err := core.InjectFrom[contracts.AuthService](ctx); err == nil && s != nil {
		return s
	}
	servicesMu.RLock()
	defer servicesMu.RUnlock()
	return authService
}

// GetTaskService returns the TaskService instance.
func GetTaskService(ctx context.Context) contracts.TaskService {
	if s, err := core.InjectFrom[contracts.TaskService](ctx); err == nil && s != nil {
		return s
	}
	servicesMu.RLock()
	defer servicesMu.RUnlock()
	return taskService
}

// GetStorageService returns the StorageService instance.
func GetStorageService(ctx context.Context) contracts.StorageService {
	if s, err := core.InjectFrom[contracts.StorageService](ctx); err == nil && s != nil {
		return s
	}
	servicesMu.RLock()
	defer servicesMu.RUnlock()
	return storageSvc
}

// GetRiskControlService returns the RiskControlService instance.
func GetRiskControlService(ctx context.Context) contracts.RiskControlService {
	if s, err := core.InjectFrom[contracts.RiskControlService](ctx); err == nil && s != nil {
		return s
	}
	servicesMu.RLock()
	defer servicesMu.RUnlock()
	return riskControlService
}

// translateNotFound collapses the persistence layer's record-not-found sentinel into
// the plugin's own domain error so that no layer above the repository has to import gorm.
func translateNotFound(err error, notFound error) error {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return notFound
	}
	return err
}

// isRecordMissing reports whether err originates from a missing persistence row.
func isRecordMissing(err error) bool {
	return errors.Is(err, gorm.ErrRecordNotFound)
}

// requireUserService resolves the injected user contract service.
func requireUserService(ctx context.Context) (contracts.UserService, error) {
	userSvc := GetUserService(ctx)
	if userSvc == nil {
		return nil, errs.ErrUserServiceUnavailable
	}
	return userSvc, nil
}

// requireAuthService resolves the injected auth contract service.
func requireAuthService(ctx context.Context) (contracts.AuthService, error) {
	authSvc := GetAuthService(ctx)
	if authSvc == nil {
		return nil, errs.ErrAuthServiceUnavailable
	}
	return authSvc, nil
}

// requireTaskService resolves the injected task contract service.
func requireTaskService(ctx context.Context) (contracts.TaskService, error) {
	taskSvc := GetTaskService(ctx)
	if taskSvc == nil {
		return nil, errs.ErrTaskServiceUnavailable
	}
	return taskSvc, nil
}
