// Copyright 2025 linux.do
// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package system_config

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/Rain-kl/Wavelet/internal/apps/cap"
	"github.com/Rain-kl/Wavelet/internal/apps/upload"
	"github.com/Rain-kl/Wavelet/internal/common/response"
	"github.com/Rain-kl/Wavelet/internal/db"
	"github.com/Rain-kl/Wavelet/internal/model"
	"github.com/Rain-kl/Wavelet/internal/repository"
	"github.com/Rain-kl/Wavelet/internal/storage"
	"github.com/Rain-kl/Wavelet/pkg/logger"
	mail "github.com/Rain-kl/Wavelet/pkg/mail"
)

const maskedConfigValue = "******"

// CreateSystemConfigRequest 创建系统配置请求
type CreateSystemConfigRequest struct {
	Key         string `json:"key" binding:"required,max=64"`
	Value       string `json:"value" binding:"required"`
	Type        string `json:"type" binding:"required,oneof=system business"`
	Visibility  int    `json:"visibility" binding:"oneof=0 1"`
	Description string `json:"description" binding:"max=255"`
}

// UpdateSystemConfigRequest 更新系统配置请求
type UpdateSystemConfigRequest struct {
	Value       string `json:"value" binding:"required"`
	Visibility  *int   `json:"visibility" binding:"omitempty,oneof=0 1"`
	Description string `json:"description" binding:"max=255"`
}

// CreateSystemConfig 创建系统配置
// @Summary 创建系统配置
// @Description 创建一条新的系统配置项，配置键不可重复，同时将新配置同步到 Redis，需要管理员权限
// @Tags admin
// @Accept json
// @Produce json
// @Security SessionCookie
// @Param request body system_config.CreateSystemConfigRequest true "创建请求参数"
// @Success 200 {object} response.Any{data=string} "创建成功"
// @Failure 400 {object} response.Any "参数错误或配置键已存在"
// @Failure 401 {object} response.Any "未登录"
// @Failure 403 {object} response.Any "无管理员权限"
// @Failure 500 {object} response.Any "内部错误"
// @Router /api/v1/admin/system-configs [post]
func CreateSystemConfig(c *gin.Context) {
	var req CreateSystemConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.AbortBadRequest(c, err.Error())
		return
	}

	if err := createSystemConfig(c.Request.Context(), req); err != nil {
		if err.Error() == ConfigKeyExists {
			response.AbortBadRequest(c, ConfigKeyExists)
			return
		}
		response.AbortInternal(c, err.Error())
		return
	}

	c.JSON(http.StatusOK, response.OKNil())
}

// ListSystemConfigs 获取系统配置列表
// @Summary 获取系统配置列表
// @Description 返回所有系统配置列表，支持按配置类型（system/business）过滤，需要管理员权限
// @Tags admin
// @Produce json
// @Security SessionCookie
// @Param type query string false "配置类型（system/business）"
// @Success 200 {object} response.Any{data=[]model.SystemConfig} "系统配置列表"
// @Failure 401 {object} response.Any "未登录"
// @Failure 403 {object} response.Any "无管理员权限"
// @Failure 500 {object} response.Any "内部错误"
// @Router /api/v1/admin/system-configs [get]
func ListSystemConfigs(c *gin.Context) {
	configs, err := listSystemConfigs(c.Request.Context(), c.Query("type"))
	if err != nil {
		response.AbortInternal(c, err.Error())
		return
	}

	for i := range configs {
		configs[i].Value = maskSensitiveConfig(configs[i].Key, configs[i].Value)
	}

	c.JSON(http.StatusOK, response.OK(configs))
}

// GetSystemConfig 获取单个系统配置
// @Summary 获取单个系统配置
// @Description 根据配置键获取对应的系统配置详情，需要管理员权限
// @Tags admin
// @Produce json
// @Security SessionCookie
// @Param key path string true "配置键"
// @Success 200 {object} response.Any{data=model.SystemConfig} "系统配置详情"
// @Failure 401 {object} response.Any "未登录"
// @Failure 403 {object} response.Any "无管理员权限"
// @Failure 404 {object} response.Any "配置不存在"
// @Failure 500 {object} response.Any "内部错误"
// @Router /api/v1/admin/system-configs/{key} [get]
func GetSystemConfig(c *gin.Context) {
	config, err := getSystemConfig(c.Request.Context(), c.Param("key"))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			response.AbortNotFound(c, SystemConfigNotFound)
		} else {
			response.AbortInternal(c, err.Error())
		}
		return
	}

	config.Value = maskSensitiveConfig(config.Key, config.Value)

	c.JSON(http.StatusOK, response.OK(config))
}

// UpdateSystemConfig 更新系统配置
// @Summary 更新系统配置
// @Description 根据配置键更新对应的配置内容，同时将更新同步到 Redis，需要管理员权限
// @Tags admin
// @Accept json
// @Produce json
// @Security SessionCookie
// @Param key path string true "配置键"
// @Param request body system_config.UpdateSystemConfigRequest true "更新请求参数"
// @Success 200 {object} response.Any{data=string} "更新成功"
// @Failure 400 {object} response.Any "参数错误"
// @Failure 401 {object} response.Any "未登录"
// @Failure 403 {object} response.Any "无管理员权限"
// @Failure 404 {object} response.Any "配置不存在"
// @Failure 500 {object} response.Any "内部错误"
// @Router /api/v1/admin/system-configs/{key} [put]
func UpdateSystemConfig(c *gin.Context) {
	var req UpdateSystemConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.AbortBadRequest(c, err.Error())
		return
	}

	key := c.Param("key")
	if err := updateSystemConfig(c.Request.Context(), key, req); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			response.AbortNotFound(c, SystemConfigNotFound)
			return
		}
		if isStorageConfigValidationError(err) {
			response.AbortBadRequest(c, err.Error())
			return
		}
		response.AbortInternal(c, err.Error())
		return
	}

	c.JSON(http.StatusOK, response.OKNil())
}

func invalidateSystemConfigCaches(ctx context.Context, key string) {
	if err := repository.InvalidateSystemConfigCache(ctx, key); err != nil {
		logger.WarnF(ctx, "清理系统配置缓存失败: %v", err)
	}
	if cap.IsRuntimeConfigKey(key) {
		cap.InvalidateRuntimeSettings()
	}
}

func invalidateCachesAfterConfigUpdate(ctx context.Context, key string) {
	invalidateSystemConfigCaches(ctx, key)

	if key == model.ConfigKeyStorageConfig {
		upload.ResetAccessCaches()
		upload.PublishAccessCacheInvalidation(ctx)
		storage.ResetCache()
		storage.PublishCacheInvalidation(ctx)
	}
	if key == model.ConfigKeyFileAccessWhitelist {
		upload.ResetAccessCaches()
		upload.PublishAccessCacheInvalidation(ctx)
	}

	if err := repository.InvalidateVisibleSystemConfigsCache(ctx); err != nil {
		logger.WarnF(ctx, "清理公共配置列表缓存失败: %v", err)
	}
}

// TestSMTPRequest 测试 SMTP 配置请求
type TestSMTPRequest struct {
	SMTPHost     string `json:"smtp_host" binding:"required,max=255"`
	SMTPPort     int    `json:"smtp_port" binding:"required"`
	SMTPUsername string `json:"smtp_username" binding:"required,max=255"`
	SMTPPassword string `json:"smtp_password" binding:"required,max=255"`
	To           string `json:"to" binding:"required,email"`
}

// TestSMTPResponse 测试 SMTP 配置响应
type TestSMTPResponse struct {
	Success bool   `json:"success"`
	Log     string `json:"log"`
	Error   string `json:"error"`
}

// TestSMTP 测试 SMTP 邮件发送
// @Summary 测试 SMTP 邮件发送
// @Description 使用传入的配置进行 SMTP 邮件发送测试，支持使用 ****** 占位符使用保存的数据库密码
// @Tags admin
// @Accept json
// @Produce json
// @Security SessionCookie
// @Param request body system_config.TestSMTPRequest true "测试请求参数"
// @Success 200 {object} response.Any{data=system_config.TestSMTPResponse} "测试执行完毕"
// @Failure 400 {object} response.Any "参数错误"
// @Router /api/v1/admin/system-configs/smtp/test [post]
func TestSMTP(c *gin.Context) {
	var req TestSMTPRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.AbortBadRequest(c, err.Error())
		return
	}

	password := req.SMTPPassword
	if password == maskedConfigValue {
		if sc, err := repository.GetSystemConfigByKey(c.Request.Context(), model.ConfigKeySMTPPassword); err == nil {
			password = sc.Value
		}
	}

	cfg := mail.Config{
		Host:     req.SMTPHost,
		Port:     req.SMTPPort,
		Username: req.SMTPUsername,
		Password: password,
	}

	subject := "OpenFlare SMTP Test Mail"
	body := `<h3>SMTP Mail Connection Test</h3>
<p>If you received this message, your SMTP configuration is correct and mail sending is working properly.</p>
<p>Sent from OpenFlare.</p>`

	logs, err := mail.SendMailWithLog(c.Request.Context(), cfg, req.To, subject, body)
	resp := TestSMTPResponse{
		Success: err == nil,
		Log:     logs,
	}
	if err != nil {
		resp.Error = err.Error()
	}

	c.JSON(http.StatusOK, response.OK(resp))
}

func isStorageConfigValidationError(err error) bool {
	msg := err.Error()
	return msg == StorageDriverSwitchRequiresMigration ||
		strings.HasPrefix(msg, "解析") ||
		strings.HasPrefix(msg, "验证") ||
		strings.HasPrefix(msg, "初始化测试") ||
		strings.HasPrefix(msg, "存储连通性") ||
		strings.HasPrefix(msg, "序列化") ||
		strings.HasPrefix(msg, "检查存量文件")
}

func maskSensitiveConfig(key, value string) string {
	if value == "" {
		return value
	}
	switch key {
	case model.ConfigKeySMTPPassword:
		return maskedConfigValue
	case model.ConfigKeyStorageConfig:
		var cfg storage.Config
		if err := json.Unmarshal([]byte(value), &cfg); err == nil {
			masked := storage.MaskSecrets(cfg)
			if val, err := json.Marshal(masked); err == nil {
				return string(val)
			}
		}
	}
	return value
}

// validateAndMergeStorageConfig parses, merges unmasked secrets, validates parameter values,
// and tests connectivity of the new storage configuration.
func validateAndMergeStorageConfig(ctx context.Context, value string, currentConfig string) (string, error) {
	var currentCfg storage.Config
	if err := json.Unmarshal([]byte(currentConfig), &currentCfg); err != nil {
		return "", fmt.Errorf("解析当前存储配置失败: %w", err)
	}

	var newCfg storage.Config
	if err := json.Unmarshal([]byte(value), &newCfg); err != nil {
		return "", fmt.Errorf("解析目标存储配置失败: %w", err)
	}

	// 合并被掩码屏蔽的敏感信息，获取完整的真实配置
	targetCfg := storage.MergeMaskedSecrets(newCfg, currentCfg)
	if err := validateMergedStorageConfig(ctx, currentCfg, newCfg, targetCfg); err != nil {
		return "", err
	}

	// 序列化为最终保存的真实明文配置，防止保存屏蔽的 ****** 字符
	unmaskedVal, err := json.Marshal(targetCfg)
	if err != nil {
		return "", fmt.Errorf("序列化存储配置失败: %w", err)
	}

	return string(unmaskedVal), nil
}

func validateMergedStorageConfig(ctx context.Context, currentCfg, newCfg, targetCfg storage.Config) error {
	if newCfg.Driver != "" && newCfg.Driver != currentCfg.Driver {
		var uploadCount int64
		if err := db.DB(ctx).Model(&model.Upload{}).
			Where("status != ?", model.UploadStatusDeleted).
			Count(&uploadCount).Error; err != nil {
			return fmt.Errorf("检查存量文件失败: %w", err)
		}
		if uploadCount > 0 {
			return errors.New(StorageDriverSwitchRequiresMigration)
		}
		if err := validateDriverConfig(targetCfg, newCfg.Driver); err != nil {
			return fmt.Errorf("验证目标存储配置参数失败: %w", err)
		}
		pendingCfg := targetCfg
		pendingCfg.Driver = newCfg.Driver
		return testStorageBackend(ctx, pendingCfg, newCfg.Driver)
	}

	if err := storage.ValidateConfig(targetCfg); err != nil {
		return fmt.Errorf("验证存储配置参数失败: %w", err)
	}
	return testStorageBackend(ctx, targetCfg, targetCfg.Driver)
}

func validateDriverConfig(cfg storage.Config, driver storage.Driver) error {
	cfg.Driver = driver
	return storage.ValidateConfig(cfg)
}

func testStorageBackend(ctx context.Context, cfg storage.Config, driver storage.Driver) error {
	testBackend, err := storage.NewBackend(ctx, cfg, driver)
	if err != nil {
		return fmt.Errorf("初始化测试存储实例失败: %w", err)
	}
	if err := testBackend.Test(ctx); err != nil {
		return fmt.Errorf("存储连通性测试失败: %w", err)
	}
	return nil
}
