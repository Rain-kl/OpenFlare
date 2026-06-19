// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package option

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/Rain-kl/Wavelet/internal/apps/openflare/geoip"
	oftasks "github.com/Rain-kl/Wavelet/internal/apps/openflare/tasks"
	"github.com/Rain-kl/Wavelet/internal/apps/openflare/uptimekuma"
	"github.com/Rain-kl/Wavelet/internal/buildinfo"
	"github.com/Rain-kl/Wavelet/internal/model"
)

var (
	initOnce sync.Once
	initErr  error
)

// EnsureInitialized loads OptionMap from defaults and database once per process.
func EnsureInitialized(ctx context.Context) error {
	initOnce.Do(func() {
		initErr = model.InitOptionMap(ctx)
	})
	return initErr
}

// ResetInitializationForTest clears lazy-init state for unit tests.
func ResetInitializationForTest() {
	initOnce = sync.Once{}
	initErr = nil
	model.ResetOptionMapForTest()
}

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
	GitHubOAuth             bool                   `json:"github_oauth"`
	GitHubClientID          string                 `json:"github_client_id"`
	SystemName              string                 `json:"system_name"`
	HomePageLink            string                 `json:"home_page_link"`
	FooterHTML              string                 `json:"footer_html"`
	WeChatQRCode            string                 `json:"wechat_qrcode"`
	WeChatLogin             bool                   `json:"wechat_login"`
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
	DeleteAll     bool   `json:"delete_all"`
	RetentionDays *int   `json:"retention_days,omitempty"`
}

type optionBatchPayload struct {
	Options []model.OpenFlareOption `json:"options"`
}

func listOptions(ctx context.Context) ([]model.OpenFlareOption, error) {
	if err := EnsureInitialized(ctx); err != nil {
		return nil, err
	}

	model.OptionMapRWMutex.RLock()
	defer model.OptionMapRWMutex.RUnlock()

	options := make([]model.OpenFlareOption, 0, len(model.OptionMap))
	for key, value := range model.OptionMap {
		if isSecretOptionKey(key) {
			continue
		}
		options = append(options, model.OpenFlareOption{
			Key:   key,
			Value: value,
		})
	}
	return options, nil
}

func updateOption(ctx context.Context, option model.OpenFlareOption) error {
	if err := EnsureInitialized(ctx); err != nil {
		return err
	}
	return updateOptions(ctx, []model.OpenFlareOption{option})
}

func updateOptionsBatch(ctx context.Context, payload optionBatchPayload) error {
	if err := EnsureInitialized(ctx); err != nil {
		return err
	}
	if len(payload.Options) == 0 {
		return errors.New(errInvalidParams)
	}
	return updateOptions(ctx, payload.Options)
}

func updateOptions(ctx context.Context, options []model.OpenFlareOption) error {
	if err := validateOptions(options); err != nil {
		return err
	}
	return model.UpdateOpenFlareOptions(ctx, options)
}

func getNotice(ctx context.Context) (string, error) {
	if err := EnsureInitialized(ctx); err != nil {
		return "", err
	}
	return model.OptionValue("Notice"), nil
}

func getStatus(ctx context.Context, baseAPIPath string) (*statusView, error) {
	if err := EnsureInitialized(ctx); err != nil {
		return nil, err
	}

	authSources, err := publicAuthSources(ctx, baseAPIPath)
	if err != nil {
		authSources = []publicAuthSourceView{}
	}

	return &statusView{
		Version:                 buildinfo.Version,
		StartTime:               model.StartTime,
		EmailVerification:       model.EmailVerificationEnabled,
		GitHubOAuth:             model.GitHubOAuthEnabled,
		GitHubClientID:          model.GitHubClientId,
		SystemName:              model.SystemName,
		HomePageLink:            model.HomePageLink,
		FooterHTML:              model.Footer,
		WeChatQRCode:            model.WeChatAccountQRCodeImageURL,
		WeChatLogin:             model.WeChatAuthEnabled,
		ServerAddress:           model.ServerAddress,
		PasswordRegisterEnabled: model.PasswordRegisterEnabled,
		CapLoginEnabled:         model.CapLoginEnabled,
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
		DeleteAll:     result.DeleteAll,
		RetentionDays: result.RetentionDays,
	}, nil
}

func syncUptimeKuma(ctx context.Context) error {
	return uptimekuma.SyncToUptimeKuma(ctx)
}

func isSecretOptionKey(key string) bool {
	return strings.Contains(key, "Token") ||
		strings.Contains(key, "Secret") ||
		strings.Contains(key, "Password")
}
