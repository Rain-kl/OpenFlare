// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package repository

import (
	"context"
	"strconv"
	"sync"

	"Wavelet/core/contracts"
	"Wavelet/openflare/plugins/server/kernel/repository/logstore"

	"gorm.io/gorm"
)

var (
	dbMu  sync.RWMutex
	dbSvc contracts.DBService
)

// SetDBService injects the platform DBService.
func SetDBService(s contracts.DBService) {
	dbMu.Lock()
	defer dbMu.Unlock()
	dbSvc = s
	if s != nil {
		logstore.SetDBResolver(s.DB)
	} else {
		logstore.SetDBResolver(nil)
	}
}

type dbServiceAdapter struct {
	db *gorm.DB
}

func (a *dbServiceAdapter) DB(ctx context.Context) *gorm.DB {
	if a.db == nil {
		return nil
	}
	return a.db.WithContext(ctx)
}

func (a *dbServiceAdapter) GORM() *gorm.DB {
	return a.db
}

func (a *dbServiceAdapter) Named(string) *gorm.DB {
	return a.db
}

type defaultGormConfigService struct {
	db *gorm.DB
}

func (s *defaultGormConfigService) GetByKey(ctx context.Context, key string) (contracts.SystemConfigDTO, error) {
	var cfg contracts.SystemConfigDTO
	err := s.db.WithContext(ctx).Table("w_system_configs").Where("key = ?", key).First(&cfg).Error
	return cfg, err
}

func (s *defaultGormConfigService) ListByKeys(ctx context.Context, keys []string) (map[string]contracts.SystemConfigDTO, error) {
	var cfgs []contracts.SystemConfigDTO
	if err := s.db.WithContext(ctx).Table("w_system_configs").Where("key IN ?", keys).Find(&cfgs).Error; err != nil {
		return nil, err
	}
	res := make(map[string]contracts.SystemConfigDTO, len(cfgs))
	for _, c := range cfgs {
		res[c.Key] = c
	}
	return res, nil
}

func (s *defaultGormConfigService) ListVisible(ctx context.Context) ([]contracts.SystemConfigDTO, error) {
	var cfgs []contracts.SystemConfigDTO
	err := s.db.WithContext(ctx).Table("w_system_configs").Where("visibility = ?", 1).Find(&cfgs).Error
	return cfgs, err
}

func (s *defaultGormConfigService) ListByType(ctx context.Context, configType string) ([]contracts.SystemConfigDTO, error) {
	var cfgs []contracts.SystemConfigDTO
	err := s.db.WithContext(ctx).Table("w_system_configs").Where("type = ?", configType).Find(&cfgs).Error
	return cfgs, err
}

func (s *defaultGormConfigService) GetIntByKey(ctx context.Context, key string) (int, error) {
	var cfg contracts.SystemConfigDTO
	if err := s.db.WithContext(ctx).Table("w_system_configs").Where("key = ?", key).First(&cfg).Error; err != nil {
		return 0, err
	}
	return strconv.Atoi(cfg.Value)
}

func (s *defaultGormConfigService) GetBoolByKey(ctx context.Context, key string) (bool, error) {
	var cfg contracts.SystemConfigDTO
	if err := s.db.WithContext(ctx).Table("w_system_configs").Where("key = ?", key).First(&cfg).Error; err != nil {
		return false, err
	}
	return strconv.ParseBool(cfg.Value)
}

func (s *defaultGormConfigService) SaveOrUpdate(ctx context.Context, key, value string) error {
	var cfg contracts.SystemConfigDTO
	if err := s.db.WithContext(ctx).Table("w_system_configs").Where("key = ?", key).First(&cfg).Error; err != nil {
		cfg = contracts.SystemConfigDTO{Key: key, Value: value, Type: "system"}
		return s.db.WithContext(ctx).Table("w_system_configs").Create(&cfg).Error
	}
	return s.db.WithContext(ctx).Table("w_system_configs").Where("key = ?", key).Update("value", value).Error
}

func (s *defaultGormConfigService) InvalidateCache(ctx context.Context, key string) error {
	return nil
}

func (s *defaultGormConfigService) InvalidateAllCaches(ctx context.Context) error {
	return nil
}

// SetDBForTest configures a test GORM instance for repository tests.
func SetDBForTest(db *gorm.DB) {
	if db == nil {
		SetDBService(nil)
		SetSystemConfigService(nil)
	} else {
		SetDBService(&dbServiceAdapter{db: db})
		SetSystemConfigService(&defaultGormConfigService{db: db})
	}
}

// DB returns the GORM DB instance with context from the injected DBService.
func DB(ctx context.Context) *gorm.DB {
	dbMu.RLock()
	s := dbSvc
	dbMu.RUnlock()
	if s != nil {
		return s.DB(ctx)
	}
	return nil
}
