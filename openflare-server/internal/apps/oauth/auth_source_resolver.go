// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package oauth

import (
	"context"
	"errors"
	"strings"

	"github.com/Rain-kl/Wavelet/internal/model"
	"github.com/Rain-kl/Wavelet/internal/repository"
	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"
)

func isOIDCLoginEnabled(ctx context.Context) bool {
	enabled, err := repository.GetBoolByKey(ctx, model.ConfigKeyOIDCLoginEnabled)
	if err != nil {
		return true
	}
	return enabled
}

func resolveAuthSource(ctx context.Context, sourceName string) (*model.AuthSource, error) {
	name := strings.TrimSpace(strings.ToLower(sourceName))
	if name == "" {
		sources, err := model.GetActiveAuthSources(ctx)
		if err != nil {
			return nil, err
		}
		if len(sources) == 0 {
			return nil, errors.New(errNoActiveAuthSource)
		}
		return &sources[0], nil
	}
	return model.GetAuthSourceByName(ctx, name)
}

func activeLoginSources(ctx context.Context) []AuthSourceView {
	enabled, err := repository.GetBoolByKey(ctx, model.ConfigKeyOIDCLoginEnabled)
	if err == nil && !enabled {
		return nil
	}

	dbSources, err := model.GetActiveAuthSources(ctx)
	if err != nil {
		return nil
	}
	sources := make([]AuthSourceView, 0, len(dbSources))
	for _, source := range dbSources {
		sources = append(sources, AuthSourceView{
			ID:                     source.ID,
			Name:                   source.Name,
			Type:                   source.Type,
			DisplayName:            source.DisplayName,
			IsActive:               source.IsActive,
			IconURL:                source.IconURL,
			ClientSecretConfigured: source.ClientSecretConfigured,
		})
	}
	return sources
}

func getFrontendLoginRedirectURL(ctx context.Context) (string, error) {
	sc, err := repository.GetSystemConfigByKey(ctx, model.ConfigKeyServerAddress)
	if err != nil || strings.TrimSpace(sc.Value) == "" {
		return "", errors.New(errServerAddressMissing)
	}
	return strings.TrimRight(sc.Value, "/") + "/login", nil
}

func buildOAuthConfig(ctx context.Context, source *model.AuthSource, redirectURL string) (*oauth2.Config, *oidc.IDTokenVerifier, error) {
	if source == nil {
		return nil, nil, errors.New(errAuthSourceRequired)
	}

	if source.OpenIDDiscoveryURL == "" {
		return nil, nil, errors.New(errDiscoveryURLRequired)
	}

	// Clean the issuer URL (trim /.well-known/openid-configuration if configured by mistake)
	issuer := strings.TrimSuffix(strings.TrimSpace(source.OpenIDDiscoveryURL), "/")
	issuer = strings.TrimSuffix(issuer, "/.well-known/openid-configuration")
	issuer = strings.TrimSuffix(issuer, "/.well-known/oauth-authorization-server")

	// 使用进程级缓存获取 provider，避免每次调用都向 issuer 发起
	// /.well-known/openid-configuration HTTP 请求。
	provider, err := globalOIDCProviderCache.get(ctx, issuer)
	if err != nil {
		return nil, nil, err
	}
	verifier := provider.Verifier(&oidc.Config{ClientID: source.ClientID})
	scopes := strings.Fields(source.Scopes)
	if len(scopes) == 0 {
		scopes = []string{oidc.ScopeOpenID, "profile", "email"}
	}
	if !containsScope(scopes, oidc.ScopeOpenID) {
		scopes = append([]string{oidc.ScopeOpenID}, scopes...)
	}

	return &oauth2.Config{
		ClientID:     source.ClientID,
		ClientSecret: source.ClientSecret,
		RedirectURL:  redirectURL,
		Scopes:       scopes,
		Endpoint:     provider.Endpoint(),
	}, verifier, nil
}

func containsScope(scopes []string, scope string) bool {
	for _, item := range scopes {
		if item == scope {
			return true
		}
	}
	return false
}
