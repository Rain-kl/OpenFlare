// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package option

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/Rain-kl/Wavelet/internal/apps/openflare/geoip"
	"github.com/Rain-kl/Wavelet/internal/model"
	"github.com/Rain-kl/Wavelet/internal/repository"
)

const maxOpenRestyGzipCompLevel = 9

var (
	openRestySizePattern          = regexp.MustCompile(`^\d+[kKmMgG]?$`)
	openRestyProxyBuffersPattern  = regexp.MustCompile(`^\d+\s+\d+[kKmMgG]?$`)
	openRestyCacheLevelsPattern   = regexp.MustCompile(`^\d{1,2}(?::\d{1,2}){0,2}$`)
	openRestyDurationTokenPattern = regexp.MustCompile(`^\d+[smhdwSMHDW]$`)
)

const optionValueTrue = "true"

// protectedConfigKeyMessage 命中受保护 key 时返回给管理员的业务错误文案。
const protectedConfigKeyMessage = "this configuration is managed by system tasks and cannot be changed manually"

// protectedConfigKeys 仅允许内部（迁移任务/bootstrap）写入的 key。
var protectedConfigKeys = map[string]bool{
	model.ConfigKeyLogDatabase:    true,
	model.ConfigKeyLogDBMigration: true,
}

func isProtectedConfigKey(key string) bool { return protectedConfigKeys[key] }

func buildOptionValidationState(ctx context.Context, options []model.OpenFlareOption) map[string]string {
	// 从 SystemConfig 读取所有业务配置构建状态
	configs, err := repository.ListAdminSystemConfigs(ctx, "business")
	state := make(map[string]string, len(configs)+len(options))

	if err == nil {
		for _, config := range configs {
			state[config.Key] = config.Value
		}
	}

	// 应用待验证的新值
	for _, option := range options {
		state[option.Key] = option.Value
	}
	return state
}

func validateOptionWithState(ctx context.Context, option model.OpenFlareOption, state map[string]string) error {

	if err := validateOpenRestyOption(option.Key, option.Value); err != nil {
		return err
	}
	if err := validateGeoIPOption(option.Key, option.Value); err != nil {
		return err
	}
	if err := validateLogRetentionOption(option.Key, option.Value); err != nil {
		return err
	}
	if err := validateAgentOption(option.Key, option.Value); err != nil {
		return err
	}
	if err := validatePagesOption(option.Key, option.Value); err != nil {
		return err
	}
	return validateUptimeKumaOption(ctx, option.Key, option.Value, state)
}

func validatePositiveIntegerOption(key, value string) error {
	intValue, err := strconv.Atoi(value)
	if err != nil || intValue <= 0 {
		return fmt.Errorf("%s must be an integer greater than 0", key)
	}
	return nil
}

func validateNonNegativeIntegerOption(key, value string) error {
	intValue, err := strconv.Atoi(value)
	if err != nil || intValue < 0 {
		return fmt.Errorf("%s must be a nonnegative integer", key)
	}
	return nil
}

func validateBooleanOption(key, value string) error {
	switch value {
	case optionValueTrue, "false":
		return nil
	default:
		return fmt.Errorf("%s must be true or false", key)
	}
}

func validateGeoIPOption(key, value string) error {
	if key != model.ConfigKeyGeoIPProvider {
		return nil
	}
	if geoip.IsValidProvider(value) {
		return nil
	}
	return fmt.Errorf("%s must be disabled, mmdb, ip-api, geojs or ipinfo", key)
}

func validateLogRetentionOption(key, value string) error {
	switch key {
	case model.ConfigKeyLogRetentionDaysPostgres, model.ConfigKeyLogRetentionDaysSQLite, model.ConfigKeyLogRetentionDaysClickHouse:
		intValue, err := strconv.Atoi(value)
		if err != nil || intValue < 1 {
			return fmt.Errorf("%s must be a whole number of days greater than or equal to 1", key)
		}
	}
	return nil
}

func validateAgentOption(key, value string) error {
	if key == model.ConfigKeyAgentWebsocketUpgradeEnabled {
		return validateBooleanOption(key, strings.TrimSpace(value))
	}
	return nil
}

func validatePagesOption(key, value string) error {
	trimmed := strings.TrimSpace(value)
	switch key {
	case model.ConfigKeyPagesMaxPackageSizeMB:
		intValue, err := strconv.Atoi(trimmed)
		if err != nil || intValue < 1 || intValue > 2048 {
			return fmt.Errorf("%s must be an integer between 1 and 2048 (MiB)", key)
		}
	case model.ConfigKeyPagesMaxHistoryCount:
		intValue, err := strconv.Atoi(trimmed)
		if err != nil || intValue < 0 {
			return fmt.Errorf("%s must be a nonnegative integer (0 means unlimited)", key)
		}
	}
	return nil
}

func validateUptimeKumaOption(ctx context.Context, key, value string, state map[string]string) error {
	trimmed := strings.TrimSpace(value)
	switch key {
	case model.ConfigKeyUptimeKumaEnabled:
		return validateUptimeKumaEnabled(ctx, key, trimmed, state)
	case model.ConfigKeyUptimeKumaUsername:
		return validateUptimeKumaUsername(trimmed, state)
	case model.ConfigKeyUptimeKumaURL:
		return validateUptimeKumaURL(trimmed)
	case model.ConfigKeyUptimeKumaMonitorScope:
		return validateUptimeKumaMonitorScope(trimmed)
	case model.ConfigKeyUptimeKumaSyncInterval, model.ConfigKeyUptimeKumaInterval, model.ConfigKeyUptimeKumaRetryInterval, model.ConfigKeyUptimeKumaTimeout:
		return validatePositiveIntegerOption(key, trimmed)
	case model.ConfigKeyUptimeKumaRetry:
		return validateUptimeKumaRetry(key, trimmed)
	}
	return nil
}

func validateUptimeKumaEnabled(ctx context.Context, key, trimmed string, state map[string]string) error {
	if err := validateBooleanOption(key, trimmed); err != nil {
		return err
	}
	if trimmed != optionValueTrue {
		return nil
	}
	url := strings.TrimSpace(state[model.ConfigKeyUptimeKumaURL])
	username := strings.TrimSpace(state[model.ConfigKeyUptimeKumaUsername])
	password := strings.TrimSpace(state[model.ConfigKeyUptimeKumaPassword])
	if url == "" {
		return errors.New("uptime Kuma address is required when the integration is enabled")
	}
	if username == "" {
		return errors.New("uptime Kuma username is required when the integration is enabled")
	}
	// 如果待验证的密码为空，且当前配置中也没有密码，则报错
	if password == "" {
		existingPwd, _ := repository.GetSystemConfigByKey(ctx, model.ConfigKeyUptimeKumaPassword)
		if strings.TrimSpace(existingPwd.Value) == "" {
			return errors.New("uptime Kuma password is required when the integration is enabled")
		}
	}
	return nil
}

func validateUptimeKumaUsername(trimmed string, state map[string]string) error {
	if trimmed == "" && state[model.ConfigKeyUptimeKumaEnabled] == optionValueTrue {
		return errors.New("uptime Kuma username is required when the integration is enabled")
	}
	return nil
}

func validateUptimeKumaURL(trimmed string) error {
	if trimmed != "" && !strings.HasPrefix(trimmed, "http://") && !strings.HasPrefix(trimmed, "https://") {
		return errors.New("uptime Kuma address must start with http:// or https://")
	}
	return nil
}

func validateUptimeKumaMonitorScope(trimmed string) error {
	if trimmed != "all" && trimmed != "selected" {
		return errors.New("monitor scope must be all or selected sites")
	}
	return nil
}

func validateUptimeKumaRetry(key, trimmed string) error {
	intValue, err := strconv.Atoi(trimmed)
	if err != nil || intValue < 0 {
		return fmt.Errorf("%s must be a nonnegative integer", key)
	}
	return nil
}

func validateOptions(ctx context.Context, options []model.OpenFlareOption) error {
	if len(options) == 0 {
		return errors.New(errInvalidParams)
	}

	state := buildOptionValidationState(ctx, options)
	for _, option := range options {
		if strings.TrimSpace(option.Key) == "" {
			return errors.New(errInvalidParams)
		}
		if isProtectedConfigKey(option.Key) {
			return errors.New(protectedConfigKeyMessage)
		}
		if err := validateOptionWithState(ctx, option, state); err != nil {
			return err
		}
	}
	return nil
}
