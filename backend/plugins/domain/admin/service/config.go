// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"Wavelet/core/contracts"
	"Wavelet/pkg/logger"
	mail "Wavelet/pkg/mail"
	"Wavelet/plugins/domain/admin/errs"
	"Wavelet/plugins/domain/admin/model"
	"Wavelet/plugins/domain/admin/repository"
	"context"
	"encoding/json"
	"errors"
	"fmt"
)

const maskedConfigValue = "******"

// PublicConfigAdapter exposes visibility=1 system configs as PublicConfigProvider.
type PublicConfigAdapter struct{}

// PublicConfig returns the unauthenticated public config map.
func (PublicConfigAdapter) PublicConfig(ctx context.Context) (map[string]string, error) {
	return PublicSystemConfigs(ctx)
}

// SystemConfigServiceImpl implements contracts.SystemConfigService.
type SystemConfigServiceImpl struct{}

// GetByKey retrieves a system config by its unique key.
func (SystemConfigServiceImpl) GetByKey(ctx context.Context, key string) (contracts.SystemConfigDTO, error) {
	cfg, err := repository.GetSystemConfigByKey(ctx, key)
	if err != nil {
		return contracts.SystemConfigDTO{}, err
	}
	return toSystemConfigDTO(cfg), nil
}

// ListByKeys retrieves multiple system configs by their keys.
func (SystemConfigServiceImpl) ListByKeys(ctx context.Context, keys []string) (map[string]contracts.SystemConfigDTO, error) {
	cfgs, err := repository.ListSystemConfigsByKeys(ctx, keys)
	if err != nil {
		return nil, err
	}
	res := make(map[string]contracts.SystemConfigDTO, len(cfgs))
	for k, v := range cfgs {
		res[k] = toSystemConfigDTO(v)
	}
	return res, nil
}

// ListVisible returns all user-visible system configs.
func (SystemConfigServiceImpl) ListVisible(ctx context.Context) ([]contracts.SystemConfigDTO, error) {
	cfgs, err := repository.ListVisibleSystemConfigs(ctx)
	if err != nil {
		return nil, err
	}
	res := make([]contracts.SystemConfigDTO, len(cfgs))
	for i, v := range cfgs {
		res[i] = toSystemConfigDTO(v)
	}
	return res, nil
}

// ListByType returns all system configs belonging to a specific configuration type.
func (SystemConfigServiceImpl) ListByType(ctx context.Context, configType string) ([]contracts.SystemConfigDTO, error) {
	cfgs, err := repository.ListAdminSystemConfigs(ctx, configType)
	if err != nil {
		return nil, err
	}
	res := make([]contracts.SystemConfigDTO, len(cfgs))
	for i, v := range cfgs {
		res[i] = toSystemConfigDTO(v)
	}
	return res, nil
}

// GetIntByKey retrieves an integer system config value.
func (SystemConfigServiceImpl) GetIntByKey(ctx context.Context, key string) (int, error) {
	return repository.GetIntByKey(ctx, key)
}

// GetBoolByKey retrieves a boolean system config value.
func (SystemConfigServiceImpl) GetBoolByKey(ctx context.Context, key string) (bool, error) {
	return repository.GetBoolByKey(ctx, key)
}

// SaveOrUpdate persists or updates a system config key-value pair.
func (SystemConfigServiceImpl) SaveOrUpdate(ctx context.Context, key, value string) error {
	return repository.SaveOrUpdateSystemConfig(ctx, key, value)
}

// InvalidateCache evicts the cache entry for the specified key.
func (SystemConfigServiceImpl) InvalidateCache(ctx context.Context, key string) error {
	return repository.InvalidateSystemConfigCache(ctx, key)
}

// InvalidateAllCaches purges all system configuration cache entries.
func (SystemConfigServiceImpl) InvalidateAllCaches(ctx context.Context) error {
	return repository.InvalidateAllSystemConfigCaches(ctx)
}

func toSystemConfigDTO(c model.SystemConfig) contracts.SystemConfigDTO {
	return contracts.SystemConfigDTO{
		Key:         c.Key,
		Value:       c.Value,
		Type:        c.Type,
		Visibility:  c.Visibility,
		Description: c.Description,
		UpdatedAt:   c.UpdatedAt,
		CreatedAt:   c.CreatedAt,
	}
}

// PublicSystemConfigs returns the key/value map exposed to unauthenticated clients.
func PublicSystemConfigs(ctx context.Context) (map[string]string, error) {
	configs, err := repository.ListVisibleSystemConfigs(ctx)
	if err != nil {
		return nil, err
	}

	resp := make(map[string]string, len(configs))
	for _, config := range configs {
		resp[config.Key] = config.Value
	}
	return resp, nil
}

// ListAdminSystemConfigs returns every config, optionally filtered by type, with secrets masked.
func ListAdminSystemConfigs(ctx context.Context, configType string) ([]model.SystemConfig, error) {
	configs, err := repository.ListAdminSystemConfigs(ctx, configType)
	if err != nil {
		return nil, err
	}
	for i := range configs {
		configs[i].Value = MaskSensitiveConfig(configs[i].Key, configs[i].Value)
	}
	return configs, nil
}

// GetAdminSystemConfig loads a single config with its secrets masked.
func GetAdminSystemConfig(ctx context.Context, key string) (model.SystemConfig, error) {
	config, err := repository.GetAdminSystemConfigByKey(ctx, key)
	if err != nil {
		return model.SystemConfig{}, translateNotFound(err, errs.ErrSystemConfigNotFound)
	}
	config.Value = MaskSensitiveConfig(config.Key, config.Value)
	return config, nil
}

// CreateAdminSystemConfig persists a new config key and refreshes the cache layer.
func CreateAdminSystemConfig(ctx context.Context, req model.CreateSystemConfigRequest) error {
	if isProtectedConfigKey(req.Key) {
		return errs.ErrProtectedConfigKey
	}
	exists, err := repository.SystemConfigExists(ctx, req.Key)
	if err != nil {
		return err
	}
	if exists {
		return errs.ErrConfigKeyExists
	}

	config := model.SystemConfig{
		Key:         req.Key,
		Value:       req.Value,
		Type:        req.Type,
		Visibility:  req.Visibility,
		Description: req.Description,
	}
	if err := repository.CreateSystemConfigRecord(ctx, &config); err != nil {
		return err
	}

	invalidateSystemConfigCaches(ctx, req.Key)
	if err := repository.InvalidateVisibleSystemConfigsCache(ctx); err != nil {
		logger.WarnF(ctx, "清理公共配置列表缓存失败: %v", err)
	}
	return nil
}

// UpdateAdminSystemConfig applies an update to a protected-aware config key inside a transaction.
func UpdateAdminSystemConfig(ctx context.Context, key string, req model.UpdateSystemConfigRequest) error {
	if isProtectedConfigKey(key) {
		return errs.ErrProtectedConfigKey
	}
	config, err := repository.GetAdminSystemConfigByKey(ctx, key)
	if err != nil {
		return translateNotFound(err, errs.ErrSystemConfigNotFound)
	}

	var originalDriver contracts.StorageDriver
	resolveTaskType := ""
	resolveResult := ""
	if key == model.ConfigKeyStorageConfig {
		var currentCfg contracts.StorageConfigDTO
		if err := json.Unmarshal([]byte(config.Value), &currentCfg); err == nil {
			originalDriver = currentCfg.Driver
		}

		validatedVal, err := validateAndMergeStorageConfig(ctx, req.Value, config.Value)
		if err != nil {
			return err
		}
		req.Value = validatedVal

		var newCfg contracts.StorageConfigDTO
		if err := json.Unmarshal([]byte(req.Value), &newCfg); err == nil {
			resolveTaskType, resolveResult = storageMigrationResolutionTask(originalDriver, newCfg.Driver)
		}
	}

	updates := map[string]any{
		"description": req.Description,
	}
	if req.Visibility != nil {
		updates["visibility"] = *req.Visibility
		config.Visibility = *req.Visibility
	}
	if key != model.ConfigKeySMTPPassword || req.Value != maskedConfigValue {
		updates["value"] = req.Value
		config.Value = req.Value
	}

	if err := repository.UpdateSystemConfigTx(ctx, &config, updates, resolveTaskType, resolveResult); err != nil {
		return err
	}

	invalidateCachesAfterConfigUpdate(ctx, key)
	return nil
}

// storageMigrationResolutionTask reports the failed-task resolution that a direct storage
// config rewrite implies. An empty task type means nothing has to be resolved.
func storageMigrationResolutionTask(
	originalDriver contracts.StorageDriver,
	newDriver contracts.StorageDriver,
) (string, string) {
	if originalDriver == "" || newDriver != originalDriver {
		return "", ""
	}
	return errs.StorageMigrationTaskType, errs.StorageDriverResolvedResult
}

func isProtectedConfigKey(key string) bool {
	return key == model.ConfigKeyLogDatabase || key == model.ConfigKeyLogDBMigration
}

func invalidateSystemConfigCaches(ctx context.Context, key string) {
	if err := repository.InvalidateSystemConfigCache(ctx, key); err != nil {
		logger.WarnF(ctx, "清理系统配置缓存失败: %v", err)
	}
	_ = EmitEvent(ctx, contracts.EventTopicConfigChanged, contracts.ConfigChangedEvent{Key: key})
}

func invalidateCachesAfterConfigUpdate(ctx context.Context, key string) {
	invalidateSystemConfigCaches(ctx, key)

	if err := repository.InvalidateVisibleSystemConfigsCache(ctx); err != nil {
		logger.WarnF(ctx, "清理公共配置列表缓存失败: %v", err)
	}
}

// TestSMTP sends a probe mail, resolving a masked password from the stored config.
func TestSMTP(ctx context.Context, req model.TestSMTPRequest) model.TestSMTPResponse {
	password := req.SMTPPassword
	if password == maskedConfigValue {
		if sc, err := repository.GetSystemConfigByKey(ctx, model.ConfigKeySMTPPassword); err == nil {
			password = sc.Value
		}
	}

	cfg := mail.Config{
		Host:     req.SMTPHost,
		Port:     req.SMTPPort,
		Username: req.SMTPUsername,
		Password: password,
	}

	subject := "Wavelet SMTP Test Mail"
	body := `<h3>SMTP Mail Connection Test</h3>
<p>If you received this message, your SMTP configuration is correct and mail sending is working properly.</p>
<p>Sent from Wavelet.</p>`

	logs, err := mail.SendMailWithLog(ctx, cfg, req.To, subject, body)
	resp := model.TestSMTPResponse{
		Success: err == nil,
		Log:     logs,
	}
	if err != nil {
		resp.Error = err.Error()
	}
	return resp
}

// MaskSensitiveConfig masks secret config values before exposing to clients.
func MaskSensitiveConfig(key, value string) string {
	if value == "" {
		return value
	}
	switch key {
	case model.ConfigKeySMTPPassword:
		return maskedConfigValue
	case model.ConfigKeyStorageConfig:
		return maskStorageConfig(value)
	}
	return value
}

func maskStorageConfig(value string) string {
	var cfg contracts.StorageConfigDTO
	if err := json.Unmarshal([]byte(value), &cfg); err != nil {
		return value
	}
	if cfg.S3.SecretAccessKey != "" {
		cfg.S3.SecretAccessKey = maskedConfigValue
	}
	if cfg.R2.SecretAccessKey != "" {
		cfg.R2.SecretAccessKey = maskedConfigValue
	}
	if cfg.MinIO.SecretAccessKey != "" {
		cfg.MinIO.SecretAccessKey = maskedConfigValue
	}
	if cfg.OSS.SecretAccessKey != "" {
		cfg.OSS.SecretAccessKey = maskedConfigValue
	}
	if cfg.WebDAV.Password != "" {
		cfg.WebDAV.Password = maskedConfigValue
	}
	val, err := json.Marshal(cfg)
	if err != nil {
		return value
	}
	return string(val)
}

// validateAndMergeStorageConfig parses, merges unmasked secrets, validates parameter values,
// and tests connectivity of the new storage configuration.
func validateAndMergeStorageConfig(ctx context.Context, value, currentConfig string) (string, error) {
	var currentCfg contracts.StorageConfigDTO
	if err := json.Unmarshal([]byte(currentConfig), &currentCfg); err != nil {
		return "", fmt.Errorf(errs.ErrParseCurrentStorageConfigFailed, err)
	}

	var newCfg contracts.StorageConfigDTO
	if err := json.Unmarshal([]byte(value), &newCfg); err != nil {
		return "", fmt.Errorf(errs.ErrParseTargetStorageConfigFailed, err)
	}

	// 合并被掩码屏蔽的敏感信息，获取完整的真实配置
	targetCfg := newCfg
	if targetCfg.S3.SecretAccessKey == maskedConfigValue {
		targetCfg.S3.SecretAccessKey = currentCfg.S3.SecretAccessKey
	}
	if targetCfg.R2.SecretAccessKey == maskedConfigValue {
		targetCfg.R2.SecretAccessKey = currentCfg.R2.SecretAccessKey
	}
	if targetCfg.MinIO.SecretAccessKey == maskedConfigValue {
		targetCfg.MinIO.SecretAccessKey = currentCfg.MinIO.SecretAccessKey
	}
	if targetCfg.OSS.SecretAccessKey == maskedConfigValue {
		targetCfg.OSS.SecretAccessKey = currentCfg.OSS.SecretAccessKey
	}
	if targetCfg.WebDAV.Password == maskedConfigValue {
		targetCfg.WebDAV.Password = currentCfg.WebDAV.Password
	}

	if err := validateMergedStorageConfig(ctx, currentCfg, newCfg, targetCfg); err != nil {
		return "", err
	}

	// 序列化为最终保存的真实明文配置，防止保存屏蔽的 ****** 字符
	unmaskedVal, err := json.Marshal(targetCfg)
	if err != nil {
		return "", fmt.Errorf(errs.ErrSerializeStorageConfigFailed, err)
	}

	return string(unmaskedVal), nil
}

func validateMergedStorageConfig(ctx context.Context, currentCfg, newCfg, _ contracts.StorageConfigDTO) error {
	if newCfg.Driver != "" && newCfg.Driver != currentCfg.Driver {
		uploadCount, err := repository.CountActiveUploads(ctx)
		if err != nil {
			return err
		}
		if uploadCount > 0 {
			return errors.New(errs.StorageDriverSwitchRequiresMigration)
		}
	}

	return nil
}
