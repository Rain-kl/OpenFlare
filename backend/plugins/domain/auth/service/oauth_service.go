// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package service implements domain business services and orchestration for the auth plugin.
package service

import (
	"Wavelet/core/contracts"
	"Wavelet/pkg/idgen"
	"Wavelet/plugins/domain/auth/consts"
	"Wavelet/plugins/domain/auth/dao"
	"Wavelet/plugins/domain/auth/model/dto"
	"Wavelet/plugins/domain/auth/model/entity"
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"
	"gorm.io/gorm"
)

// OAuthService orchestrates OAuth/OIDC operations.
type OAuthService struct {
	dao           *dao.DAO
	providerCache *OIDCProviderCache
	sessionSvc    *SessionService
}

// NewOAuthService creates a new OAuthService.
func NewOAuthService(d *dao.DAO, cache *OIDCProviderCache, sessSvc *SessionService) *OAuthService {
	return &OAuthService{
		dao:           d,
		providerCache: cache,
		sessionSvc:    sessSvc,
	}
}

// IsOIDCLoginEnabled checks if OIDC login is globally enabled.
func (s *OAuthService) IsOIDCLoginEnabled(ctx context.Context) bool {
	val, err := s.dao.GetSystemConfigValue(ctx, "oidc_login_enabled")
	if err != nil || val == "" {
		return true
	}
	b, err := strconv.ParseBool(val)
	if err != nil {
		return true
	}
	return b
}

// ResolveAuthSource retrieves the specified or default active auth source.
func (s *OAuthService) ResolveAuthSource(ctx context.Context, sourceName string) (*entity.AuthSource, error) {
	name := strings.TrimSpace(strings.ToLower(sourceName))
	if name == "" {
		sources, err := s.dao.ListActiveAuthSources(ctx)
		if err != nil {
			return nil, err
		}
		if len(sources) == 0 {
			return nil, errors.New(consts.ErrNoActiveAuthSource)
		}
		src, err := s.dao.GetAuthSourceByName(ctx, sources[0].Name)
		if err != nil {
			return nil, err
		}
		return src, nil
	}
	src, err := s.dao.GetAuthSourceByName(ctx, name)
	if err != nil {
		return nil, err
	}
	return src, nil
}

// ActiveLoginSources returns all active login sources formatted for display.
func (s *OAuthService) ActiveLoginSources(ctx context.Context) ([]dto.AuthSourceView, error) {
	if !s.IsOIDCLoginEnabled(ctx) {
		return nil, nil
	}

	dbSources, err := s.dao.ListActiveAuthSources(ctx)
	if err != nil {
		return nil, err
	}
	sources := make([]dto.AuthSourceView, 0, len(dbSources))
	for _, source := range dbSources {
		sources = append(sources, dto.AuthSourceView{
			ID:                     source.ID,
			Name:                   source.Name,
			Type:                   source.Type,
			DisplayName:            source.DisplayName,
			IsActive:               source.IsActive,
			IconURL:                source.IconURL,
			ClientSecretConfigured: source.ClientSecretConfigured,
		})
	}
	return sources, nil
}

// GetFrontendLoginRedirectURL constructs the OAuth frontend redirect URL.
func (s *OAuthService) GetFrontendLoginRedirectURL(ctx context.Context) (string, error) {
	val, err := s.dao.GetSystemConfigValue(ctx, "server_address")
	if err != nil || strings.TrimSpace(val) == "" {
		return "", errors.New(consts.ErrServerAddressMissing)
	}
	return strings.TrimRight(val, "/") + "/login", nil
}

// ReserveOAuthStateSlot ensures that a session does not abuse OAuth state generation.
func (s *OAuthService) ReserveOAuthStateSlot(ctx context.Context, sessionHash string) error {
	if sessionHash == "" {
		return nil
	}
	if limiter := s.dao.Limiter(); limiter != nil {
		key := fmt.Sprintf(consts.OAuthStateLimitKeyFormat, sessionHash)
		res, err := limiter.Allow(ctx, key, contracts.Rate{
			Limit:  consts.OAuthStateLimitMax,
			Period: consts.OAuthStateCacheKeyExpiration,
		})
		if err != nil {
			return err
		}
		if !res.Allowed {
			return errors.New(consts.ErrOAuthStateRateLimited)
		}
		return nil
	}

	cache := s.dao.Cache()
	if cache == nil {
		return nil
	}
	key := fmt.Sprintf(consts.OAuthStateLimitKeyFormat, sessionHash)
	var count int
	_ = cache.Get(ctx, key, &count)
	count++
	_ = cache.Set(ctx, key, count, consts.OAuthStateCacheKeyExpiration)
	if count > consts.OAuthStateLimitMax {
		return errors.New(consts.ErrOAuthStateRateLimited)
	}
	return nil
}

// BuildOAuthConfig builds oauth2.Config and oidc.IDTokenVerifier.
func (s *OAuthService) BuildOAuthConfig(ctx context.Context, source *entity.AuthSource, redirectURL string) (*oauth2.Config, *oidc.IDTokenVerifier, error) {
	if source == nil {
		return nil, nil, errors.New(consts.ErrAuthSourceRequired)
	}

	if source.OpenIDDiscoveryURL == "" {
		return nil, nil, errors.New(consts.ErrDiscoveryURLRequired)
	}

	issuer := strings.TrimSuffix(strings.TrimSpace(source.OpenIDDiscoveryURL), "/")
	issuer = strings.TrimSuffix(issuer, "/.well-known/openid-configuration")
	issuer = strings.TrimSuffix(issuer, "/.well-known/oauth-authorization-server")

	provider, err := s.providerCache.Get(ctx, issuer)
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

// BuildAuthorizeURL generates the redirect authorize URL for the source and state.
func (s *OAuthService) BuildAuthorizeURL(ctx context.Context, source *entity.AuthSource, state string) (string, error) {
	redirectURL, err := s.GetFrontendLoginRedirectURL(ctx)
	if err != nil {
		return "", err
	}
	authConfig, verifier, err := s.BuildOAuthConfig(ctx, source, redirectURL)
	if err != nil {
		return "", err
	}
	if verifier != nil {
		return authConfig.AuthCodeURL(state, oidc.Nonce(state)), nil
	}
	return authConfig.AuthCodeURL(state), nil
}

// BuildOAuthUserInfo exchanges the auth code and retrieves user identity claims.
func (s *OAuthService) BuildOAuthUserInfo(ctx context.Context, source *entity.AuthSource, code, nonce, redirectURL string) (*contracts.OAuthUserInfoDTO, error) {
	authConfig, verifier, err := s.BuildOAuthConfig(ctx, source, redirectURL)
	if err != nil {
		return nil, err
	}

	token, err := authConfig.Exchange(ctx, code)
	if err != nil {
		return nil, err
	}

	userInfo := &contracts.OAuthUserInfoDTO{Active: true}
	if verifier != nil {
		if verifyErr := s.verifyIDToken(ctx, verifier, token, nonce, userInfo); verifyErr != nil {
			return nil, verifyErr
		}
	}

	if userInfo.Username == "" && userInfo.PreferredUsername != "" {
		userInfo.Username = userInfo.PreferredUsername
	}
	if userInfo.Username == "" && userInfo.Email != "" {
		userInfo.Username = strings.Split(userInfo.Email, "@")[0]
	}
	if userInfo.Username == "" && userInfo.Sub != "" {
		userInfo.Username = userInfo.Sub
	}
	if userInfo.Name == "" {
		userInfo.Name = userInfo.Username
	}

	return userInfo, nil
}

func (s *OAuthService) verifyIDToken(ctx context.Context, verifier *oidc.IDTokenVerifier, token *oauth2.Token, nonce string, userInfo *contracts.OAuthUserInfoDTO) error {
	rawIDToken, ok := token.Extra("id_token").(string)
	if !ok {
		return nil
	}
	idToken, verifyErr := verifier.Verify(ctx, rawIDToken)
	if verifyErr != nil {
		return fmt.Errorf(consts.ErrIDTokenVerifyFailedFormat, consts.ErrIDTokenVerifyFailed, verifyErr)
	}
	if nonce != "" && idToken.Nonce != nonce {
		return errors.New(consts.ErrNonceMismatch)
	}
	if claimsErr := idToken.Claims(userInfo); claimsErr != nil {
		return claimsErr
	}
	return nil
}

// NormalizeOAuthUserInfo sanitizes user claims.
func (s *OAuthService) NormalizeOAuthUserInfo(userInfo *contracts.OAuthUserInfoDTO) error {
	userInfo.Username = strings.TrimSpace(userInfo.Username)
	userInfo.PreferredUsername = strings.TrimSpace(userInfo.PreferredUsername)
	userInfo.Email = strings.TrimSpace(userInfo.Email)
	userInfo.Name = strings.TrimSpace(userInfo.Name)
	userInfo.AvatarURL = strings.TrimSpace(userInfo.AvatarURL)

	if userInfo.Username == "" && userInfo.PreferredUsername != "" {
		userInfo.Username = userInfo.PreferredUsername
	}
	if userInfo.Username == "" && userInfo.Email != "" {
		userInfo.Username = strings.Split(userInfo.Email, "@")[0]
	}
	if userInfo.Username == "" && userInfo.Sub != "" {
		userInfo.Username = userInfo.Sub
	}
	if userInfo.Username == "" {
		return errors.New(consts.ErrUsernameFromSourceFailed)
	}
	if userInfo.Name == "" {
		userInfo.Name = userInfo.Username
	}
	if !userInfo.Active {
		userInfo.Active = true
	}
	return nil
}

// UniqueUsername generates a unique username given a base candidate.
func (s *OAuthService) UniqueUsername(ctx context.Context, base string) (string, error) {
	base = strings.TrimSpace(base)
	if base == "" {
		base = "user"
	}

	existingUsernames, err := s.dao.ListSimilarUsernames(ctx, base)
	if err != nil {
		return "", err
	}

	exists := make(map[string]bool, len(existingUsernames))
	for _, u := range existingUsernames {
		exists[strings.ToLower(u)] = true
	}

	if !exists[strings.ToLower(base)] {
		return base, nil
	}

	for i := 1; i <= 1000; i++ {
		candidate := fmt.Sprintf("%s-%d", base, i)
		if !exists[strings.ToLower(candidate)] {
			return candidate, nil
		}
	}

	return "", errors.New(consts.ErrUsernameGenerateFailed)
}

// BindExternalAccount binds an external identity to an existing user.
func (s *OAuthService) BindExternalAccount(ctx context.Context, sourceID, userID uint64, userInfo *contracts.OAuthUserInfoDTO) error {
	user, err := s.dao.GetUserByID(ctx, userID)
	if err != nil {
		return err
	}
	if err := s.dao.BindExternalAccount(ctx, &entity.ExternalAccount{
		AuthSourceID:     sourceID,
		UserID:           user.ID,
		ExternalID:       userInfo.Sub,
		ExternalUsername: userInfo.Username,
		Email:            userInfo.Email,
	}); err != nil {
		return err
	}
	user.LastLoginAt = time.Now()
	_ = s.dao.TouchUserLastLogin(ctx, user.ID, user.LastLoginAt)
	return nil
}

// AuthenticateOrRegisterUser finds existing binding or creates a new user.
func (s *OAuthService) AuthenticateOrRegisterUser(ctx context.Context, source *entity.AuthSource, userInfo *contracts.OAuthUserInfoDTO) (*contracts.UserDTO, bool, error) {
	account, err := s.dao.FindExternalAccount(ctx, source.ID, userInfo.Sub)
	if err == nil {
		user, loadErr := s.dao.GetUserByID(ctx, account.UserID)
		if loadErr != nil {
			return nil, false, loadErr
		}
		user.LastLoginAt = time.Now()
		_ = s.dao.TouchUserLastLogin(ctx, user.ID, user.LastLoginAt)
		return user, true, nil
	}

	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, false, err
	}

	// Not found -> check registration
	registrationEnabled := true
	val, cfgErr := s.dao.GetSystemConfigValue(ctx, "registration_enabled")
	if cfgErr == nil && val != "" {
		if b, err := strconv.ParseBool(val); err == nil {
			registrationEnabled = b
		}
	}

	if !registrationEnabled {
		return nil, false, nil // registration disabled -> need bind
	}

	username, uniqueErr := s.UniqueUsername(ctx, userInfo.Username)
	if uniqueErr != nil {
		return nil, false, uniqueErr
	}
	userInfo.Username = username

	now := time.Now()
	user := contracts.UserDTO{
		ID:          idgen.NextUint64ID(),
		Username:    userInfo.Username,
		Nickname:    userInfo.Name,
		Email:       userInfo.Email,
		AvatarURL:   userInfo.AvatarURL,
		IsActive:    userInfo.Active,
		LastLoginAt: now,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	if err := s.dao.InsertUser(ctx, &user); err != nil {
		return nil, false, err
	}

	if err := s.dao.BindExternalAccount(ctx, &entity.ExternalAccount{
		AuthSourceID:     source.ID,
		UserID:           user.ID,
		ExternalID:       userInfo.Sub,
		ExternalUsername: userInfo.Username,
		Email:            userInfo.Email,
	}); err != nil {
		return nil, false, err
	}

	return &user, true, nil
}

// ListExternalAccounts returns sanitized external account bindings.
func (s *OAuthService) ListExternalAccounts(ctx context.Context, userID uint64) ([]dto.ExternalAccountView, error) {
	accounts, err := s.dao.ListExternalAccountsByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}
	views := make([]dto.ExternalAccountView, len(accounts))
	for i, acc := range accounts {
		source, _ := s.dao.GetAuthSourceByID(ctx, acc.AuthSourceID)
		sourceName, sourceType, sourceLabel := "", "", ""
		if source != nil {
			sourceName = source.Name
			sourceType = source.Type
			sourceLabel = source.DisplayName
		}
		views[i] = dto.ExternalAccountView{
			ID:               acc.ID,
			AuthSourceID:     acc.AuthSourceID,
			AuthSourceName:   sourceName,
			AuthSourceType:   sourceType,
			AuthSourceLabel:  sourceLabel,
			ExternalUsername: acc.ExternalUsername,
			Email:            acc.Email,
			CreatedAt:        acc.CreatedAt.Format(time.RFC3339),
		}
	}
	return views, nil
}

// DeleteExternalAccount unbinds an external account.
func (s *OAuthService) DeleteExternalAccount(ctx context.Context, id, userID uint64) error {
	return s.dao.UnbindExternalAccount(ctx, id, userID)
}
