// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package repository

import (
	"context"
	"errors"
	"sync"

	"Wavelet/core/contracts"
	"Wavelet/openflare/plugins/server/kernel/model"
)

var (
	configMu  sync.RWMutex
	configSvc contracts.SystemConfigService
)

// SetSystemConfigService injects the platform SystemConfigService.
func SetSystemConfigService(s contracts.SystemConfigService) {
	configMu.Lock()
	defer configMu.Unlock()
	configSvc = s
}

func currentConfigService() contracts.SystemConfigService {
	configMu.RLock()
	defer configMu.RUnlock()
	return configSvc
}

func ensureConfigService() (contracts.SystemConfigService, error) {
	svc := currentConfigService()
	if svc == nil {
		return nil, errors.New("system config service not initialized")
	}
	return svc, nil
}

// GetSystemConfigByKey loads a config row by key through the system config service.
func GetSystemConfigByKey(ctx context.Context, key string) (model.SystemConfig, error) {
	svc, err := ensureConfigService()
	if err != nil {
		return model.SystemConfig{}, err
	}
	dto, err := svc.GetByKey(ctx, key)
	if err != nil {
		return model.SystemConfig{}, err
	}
	return model.FromSystemConfigDTO(dto), nil
}

// ListSystemConfigsByKeys loads multiple config keys through the system config service.
func ListSystemConfigsByKeys(ctx context.Context, keys []string) (map[string]model.SystemConfig, error) {
	svc, err := ensureConfigService()
	if err != nil {
		return nil, err
	}
	dtos, err := svc.ListByKeys(ctx, keys)
	if err != nil {
		return nil, err
	}
	res := make(map[string]model.SystemConfig, len(dtos))
	for k, v := range dtos {
		res[k] = model.FromSystemConfigDTO(v)
	}
	return res, nil
}

// ListVisibleSystemConfigs returns visibility=1 configs from the system config service.
func ListVisibleSystemConfigs(ctx context.Context) ([]model.SystemConfig, error) {
	svc, err := ensureConfigService()
	if err != nil {
		return nil, err
	}
	dtos, err := svc.ListVisible(ctx)
	if err != nil {
		return nil, err
	}
	res := make([]model.SystemConfig, len(dtos))
	for i, v := range dtos {
		res[i] = model.FromSystemConfigDTO(v)
	}
	return res, nil
}

// GetIntByKey queries config and converts to int.
func GetIntByKey(ctx context.Context, key string) (int, error) {
	svc, err := ensureConfigService()
	if err != nil {
		return 0, err
	}
	return svc.GetIntByKey(ctx, key)
}

// GetBoolByKey queries config and converts to bool.
func GetBoolByKey(ctx context.Context, key string) (bool, error) {
	svc, err := ensureConfigService()
	if err != nil {
		return false, err
	}
	return svc.GetBoolByKey(ctx, key)
}

// CreateSystemConfig persists a new system config row.
func CreateSystemConfig(ctx context.Context, config *model.SystemConfig) error {
	svc, err := ensureConfigService()
	if err != nil {
		return err
	}
	return svc.SaveOrUpdate(ctx, config.Key, config.Value)
}

// SaveOrUpdateSystemConfig creates or updates a config row.
func SaveOrUpdateSystemConfig(ctx context.Context, key, value string) error {
	svc, err := ensureConfigService()
	if err != nil {
		return err
	}
	return svc.SaveOrUpdate(ctx, key, value)
}

// InvalidateSystemConfigCache evicts one key from the system-config cache.
func InvalidateSystemConfigCache(ctx context.Context, key string) error {
	svc, err := ensureConfigService()
	if err != nil {
		return err
	}
	return svc.InvalidateCache(ctx, key)
}

// InvalidateAllSystemConfigCaches evicts the whole system-config cache.
func InvalidateAllSystemConfigCaches(ctx context.Context) error {
	svc, err := ensureConfigService()
	if err != nil {
		return err
	}
	return svc.InvalidateAllCaches(ctx)
}

// StopSystemConfigCacheListener is retained for test compatibility.
func StopSystemConfigCacheListener() {}

// ResetSystemConfigRAMCacheForTest clears the process-local admin config cache.
func ResetSystemConfigRAMCacheForTest() {
	if svc := currentConfigService(); svc != nil {
		_ = svc.InvalidateAllCaches(context.Background())
	}
}

// ListAdminSystemConfigs returns configs, optionally filtered by type.
func ListAdminSystemConfigs(ctx context.Context, configType string) ([]model.SystemConfig, error) {
	svc, err := ensureConfigService()
	if err != nil {
		return nil, err
	}
	dtos, err := svc.ListByType(ctx, configType)
	if err != nil {
		return nil, err
	}
	res := make([]model.SystemConfig, len(dtos))
	for i, v := range dtos {
		res[i] = model.FromSystemConfigDTO(v)
	}
	return res, nil
}
