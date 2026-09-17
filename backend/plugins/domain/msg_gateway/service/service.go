// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package service implements domain business logic, bot gateway adapters, and push notification services for msg_gateway.
package service

import (
	"Wavelet/core"
	"Wavelet/core/contracts"
	"context"
	"sync"
)

// Platform service dependencies resolved from Cordis context or global fallbacks.
var (
	cacheMu  sync.RWMutex
	cacheSvc contracts.CacheService
	taskMu   sync.RWMutex
	taskSvc  contracts.TaskService
	userMu   sync.RWMutex
	userSvc  contracts.UserService
)

// SetCacheService sets the cache service.
func SetCacheService(s contracts.CacheService) {
	cacheMu.Lock()
	defer cacheMu.Unlock()
	cacheSvc = s
}

// SetTaskService sets the task service.
func SetTaskService(s contracts.TaskService) {
	taskMu.Lock()
	defer taskMu.Unlock()
	taskSvc = s
}

// SetUserService sets the user service.
func SetUserService(s contracts.UserService) {
	userMu.Lock()
	defer userMu.Unlock()
	userSvc = s
}

// GetCache resolves the cache service for the context.
func GetCache(ctx context.Context) contracts.CacheService {
	if s, err := core.InjectFrom[contracts.CacheService](ctx); err == nil && s != nil {
		return s
	}
	cacheMu.RLock()
	s := cacheSvc
	cacheMu.RUnlock()
	return s
}

// GetTaskService resolves the task service for the context.
func GetTaskService(ctx context.Context) contracts.TaskService {
	if s, err := core.InjectFrom[contracts.TaskService](ctx); err == nil && s != nil {
		return s
	}
	taskMu.RLock()
	defer taskMu.RUnlock()
	return taskSvc
}

// GetUserService resolves the user service for the context.
func GetUserService(ctx context.Context) contracts.UserService {
	if s, err := core.InjectFrom[contracts.UserService](ctx); err == nil && s != nil {
		return s
	}
	userMu.RLock()
	s := userSvc
	userMu.RUnlock()
	return s
}
