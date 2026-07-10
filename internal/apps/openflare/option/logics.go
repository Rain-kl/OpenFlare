// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package option

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/Rain-kl/Wavelet/internal/apps/openflare/geoip"
	oftasks "github.com/Rain-kl/Wavelet/internal/apps/openflare/tasks"
	"github.com/Rain-kl/Wavelet/internal/apps/openflare/uptimekuma"
	"github.com/Rain-kl/Wavelet/internal/buildinfo"
	"github.com/Rain-kl/Wavelet/internal/model"
	"github.com/Rain-kl/Wavelet/internal/repository"
)

type publicAuthSourceView struct {
	ID           uint64 `json:"id"`
	Name         string `json:"name"`
	Type         string `json:"type"`
	DisplayName  string `json:"display_name"`
	AuthorizeURL string `json:"authorize_url"`
	IconURL      string `json:"icon_url"`
}

type statusView struct {
	Version                 string                 `json:"version"`
	StartTime               int64                  `json:"start_time"`
	EmailVerification       bool                   `json:"email_verification"`
	ServerAddress           string                 `json:"server_address"`
	PasswordRegisterEnabled bool                   `json:"password_register_enabled"`
	CapLoginEnabled         bool                   `json:"cap_login_enabled"`
	AuthSources             []publicAuthSourceView `json:"auth_sources"`
}

type geoIPLookupRequest struct {
	Provider string `json:"provider"`
	IP       string `json:"ip"`
}

type geoIPLookupView struct {
	Provider  string   `json:"provider"`
	IP        string   `json:"ip"`
	ISOCode   string   `json:"iso_code"`
	Name      string   `json:"name"`
	Latitude  *float64 `json:"latitude,omitempty"`
	Longitude *float64 `json:"longitude,omitempty"`
}

type databaseCleanupInput struct {
	Target        string `json:"target"`
	RetentionDays *int   `json:"retention_days"`
}

type databaseCleanupResult struct {
	Target        string `json:"target"`
	TargetLabel   string `json:"target_label"`
	DeletedCount  int64  `json:"deleted_count"`
	EligibleCount int64  `json:"eligible_count,omitempty"`
	CleanupMode   string `json:"cleanup_mode,omitempty"`
	TableTTLDays  int    `json:"table_ttl_days,omitempty"`
	DeleteAll     bool   `json:"delete_all"`
	RetentionDays *int   `json:"retention_days,omitempty"`
}

type optionBatchPayload struct {
	Options []model.OpenFlareOption `json:"options"`
}

func listOptions(ctx context.Context) ([]model.OpenFlareOption, error) {
	// 从 SystemConfig 读取所有业务配置
	configs, err := repository.ListAdminSystemConfigs(ctx, "business")
	if err != nil {
		return nil, err
	}

	options := make([]model.OpenFlareOption, 0, len(configs))
	for _, config := range configs {
		// 跳过敏感配置（如密码、令牌）
		if config.Visibility == model.ConfigVisibilityHidden && isSecretConfigKey(config.Key) {
			continue
		}
		// 将 snake_case key 转换为 PascalCase 以保持向后兼容
		options = append(options, model.OpenFlareOption{
			Key:   config.Key,
			Value: config.Value,
		})
	}
	return options, nil
}

func updateOption(ctx context.Context, option model.OpenFlareOption) error {
	return updateOptions(ctx, []model.OpenFlareOption{option})
}

func updateOptionsBatch(ctx context.Context, payload optionBatchPayload) error {
	if len(payload.Options) == 0 {
		return errors.New(errInvalidParams)
	}
	return updateOptions(ctx, payload.Options)
}

func updateOptions(ctx context.Context, options []model.OpenFlareOption) error {
	if err := validateOptions(ctx, options); err != nil {
		return err
	}

	// 将每个 option 更新到 SystemConfig
	for _, opt := range options {
		if err := repository.SaveOrUpdateSystemConfig(ctx, opt.Key, opt.Value); err != nil {
			return fmt.Errorf("failed to update config %s: %w", opt.Key, err)
		}

		// 特殊处理：GeoIP 配置变更时刷新运行时
		if opt.Key == model.ConfigKeyGeoIPProvider {
			if err := geoip.RefreshRuntimeProvider(ctx); err != nil {
				return err
			}
		}
	}

	return nil
}

func getStatus(ctx context.Context, baseAPIPath string) (*statusView, error) {
	authSources, err := publicAuthSources(ctx, baseAPIPath)
	if err != nil {
		authSources = []publicAuthSourceView{}
	}

	// 从 SystemConfig 读取配置
	emailVerification, _ := repository.GetBoolByKey(ctx, model.ConfigKeyEmailLoginVerificationEnabled)
	serverAddress, _ := repository.GetSystemConfigByKey(ctx, model.ConfigKeyServerAddress)
	passwordRegisterEnabled, _ := repository.GetBoolByKey(ctx, model.ConfigKeyPasswordRegisterEnabled)
	capLoginEnabled, _ := repository.GetBoolByKey(ctx, model.ConfigKeyCapLoginEnabled)

	return &statusView{
		Version:                 buildinfo.Version,
		StartTime:               model.StartTime,
		EmailVerification:       emailVerification,
		ServerAddress:           serverAddress.Value,
		PasswordRegisterEnabled: passwordRegisterEnabled,
		CapLoginEnabled:         capLoginEnabled,
		AuthSources:             authSources,
	}, nil
}

func publicAuthSources(ctx context.Context, baseAPIPath string) ([]publicAuthSourceView, error) {
	sources, err := model.GetActiveAuthSources(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]publicAuthSourceView, 0, len(sources))
	base := strings.TrimRight(baseAPIPath, "/")
	for _, source := range sources {
		result = append(result, publicAuthSourceView{
			ID:           source.ID,
			Name:         source.Name,
			Type:         source.Type,
			DisplayName:  source.DisplayName,
			AuthorizeURL: fmt.Sprintf("%s/oauth/%s/authorize", base, source.Name),
			IconURL:      source.IconURL,
		})
	}
	return result, nil
}

func lookupGeoIP(_ context.Context, provider, rawIP string) (*geoIPLookupView, error) {
	view, err := geoip.Lookup(provider, rawIP)
	if err != nil {
		return nil, err
	}
	return &geoIPLookupView{
		Provider:  view.Provider,
		IP:        view.IP,
		ISOCode:   view.ISOCode,
		Name:      view.Name,
		Latitude:  view.Latitude,
		Longitude: view.Longitude,
	}, nil
}

func cleanupDatabaseObservability(ctx context.Context, input databaseCleanupInput) (*databaseCleanupResult, error) {
	target := strings.TrimSpace(input.Target)
	if target == "" {
		return nil, errors.New(errInvalidParams)
	}

	result, err := oftasks.CleanupDatabaseObservability(ctx, oftasks.DatabaseCleanupInput{
		Target:        target,
		RetentionDays: input.RetentionDays,
	})
	if err != nil {
		return nil, err
	}

	return &databaseCleanupResult{
		Target:        result.Target,
		TargetLabel:   result.TargetLabel,
		DeletedCount:  result.DeletedCount,
		EligibleCount: result.EligibleCount,
		CleanupMode:   result.CleanupMode,
		TableTTLDays:  result.TableTTLDays,
		DeleteAll:     result.DeleteAll,
		RetentionDays: result.RetentionDays,
	}, nil
}

func syncUptimeKuma(ctx context.Context) error {
	return uptimekuma.SyncToUptimeKuma(ctx)
}

// isSecretConfigKey 判断 SystemConfig 的 key 是否为敏感配置
func isSecretConfigKey(key string) bool {
	return strings.Contains(key, "token") ||
		strings.Contains(key, "secret") ||
		strings.Contains(key, "password")
}
