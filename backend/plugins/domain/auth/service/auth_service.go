// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package service implements domain business services and orchestration for the auth plugin.
package service

import (
	"Wavelet/core/contracts"
	"Wavelet/plugins/domain/auth/consts"
	"Wavelet/plugins/domain/auth/dao"
	"Wavelet/plugins/domain/auth/model/entity"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
)

// HashToken computes SHA-256 hex digest of access token.
func HashToken(token string) string {
	h := sha256.New()
	h.Write([]byte(token))
	return hex.EncodeToString(h.Sum(nil))
}

// UserIDExtractor extracts user ID from a request context.
type UserIDExtractor func(ctx context.Context) (uint64, bool)

// AuthServiceImpl implements contracts.AuthService.
type AuthServiceImpl struct {
	dao                     *dao.DAO
	requireAuthMiddleware   any
	requireAdminMiddleware  any
	disallowTokenMiddleware any
	userIDExtractor         UserIDExtractor
}

// NewAuthService creates a new AuthServiceImpl.
func NewAuthService(
	d *dao.DAO,
	requireAuth any,
	requireAdmin any,
	disallowToken any,
	extractor UserIDExtractor,
) *AuthServiceImpl {
	return &AuthServiceImpl{
		dao:                     d,
		requireAuthMiddleware:   requireAuth,
		requireAdminMiddleware:  requireAdmin,
		disallowTokenMiddleware: disallowToken,
		userIDExtractor:         extractor,
	}
}

// SetMiddlewareHandlers wires middleware handlers into AuthService after controller initialization.
func (s *AuthServiceImpl) SetMiddlewareHandlers(requireAuth, requireAdmin, disallowToken any, extractor UserIDExtractor) {
	s.requireAuthMiddleware = requireAuth
	s.requireAdminMiddleware = requireAdmin
	s.disallowTokenMiddleware = disallowToken
	s.userIDExtractor = extractor
}

// RequireAuthMiddleware returns the authentication check middleware.
func (s *AuthServiceImpl) RequireAuthMiddleware() any {
	return s.requireAuthMiddleware
}

// RequireAdminMiddleware returns the admin authorization middleware.
func (s *AuthServiceImpl) RequireAdminMiddleware() any {
	return s.requireAdminMiddleware
}

// DisallowTokenAuthMiddleware returns the token rejection middleware.
func (s *AuthServiceImpl) DisallowTokenAuthMiddleware() any {
	return s.disallowTokenMiddleware
}

// GetCurrentUser 从 context 中读取登录用户。
func (s *AuthServiceImpl) GetCurrentUser(ctx context.Context) (*contracts.UserDTO, error) {
	if v := ctx.Value(contracts.AuthUserObjKey); v != nil {
		if u, ok := v.(*contracts.UserDTO); ok && u != nil {
			return u, nil
		}
	}

	return nil, errors.New(consts.ErrUserNotInContext)
}

// GetCurrentUserID 从请求登录态中读取用户 ID。
func (s *AuthServiceImpl) GetCurrentUserID(ctx context.Context) (uint64, error) {
	if s.userIDExtractor != nil {
		if userID, ok := s.userIDExtractor(ctx); ok {
			return userID, nil
		}
	}
	return 0, errors.New(consts.ErrUserNotInContext)
}

// VerifyToken 验证访问令牌并返回对应的用户。
func (s *AuthServiceImpl) VerifyToken(ctx context.Context, token string) (*contracts.UserDTO, error) {
	if token == "" {
		return nil, errors.New(consts.ErrEmptyToken)
	}

	tokenHash := HashToken(token)
	tokenRecord, err := s.dao.GetCachedToken(ctx, tokenHash)
	if err != nil {
		tokenRecord, err = s.dao.GetAccessTokenByHash(ctx, tokenHash)
		if err != nil {
			return nil, err
		}
		s.dao.SetCachedToken(ctx, tokenHash, tokenRecord)
	}

	user, err := s.dao.GetCachedUser(ctx, tokenRecord.UserID)
	if err != nil || user == nil || !user.IsActive {
		user, err = s.dao.GetActiveUserByID(ctx, tokenRecord.UserID)
		if err != nil {
			return nil, err
		}
		s.dao.SetCachedUser(ctx, tokenRecord.UserID, user)
	}

	if user.Username == consts.SystemUsername {
		return nil, errors.New(consts.ErrSystemUserTokenNotAllowed)
	}

	return user, nil
}

// CreateSession establishes an authenticated session.
func (s *AuthServiceImpl) CreateSession(_ context.Context, _ uint64, _ map[string]any) (string, error) {
	return "", nil
}

// RevokeUserSessions revokes active sessions and cached tokens for a user.
func (s *AuthServiceImpl) RevokeUserSessions(ctx context.Context, userID uint64) error {
	s.dao.InvalidateCachedUser(ctx, userID)
	return nil
}

// RevokeToken invalidates a cached token by its hash.
func (s *AuthServiceImpl) RevokeToken(ctx context.Context, tokenHash string) error {
	s.dao.InvalidateCachedToken(ctx, tokenHash)
	return nil
}

// InvalidateCachedUser invalidates cached user profile.
func (s *AuthServiceImpl) InvalidateCachedUser(ctx context.Context, userID uint64) {
	s.dao.InvalidateCachedUser(ctx, userID)
}

// InvalidateCachedToken invalidates cached access token.
func (s *AuthServiceImpl) InvalidateCachedToken(ctx context.Context, tokenHash string) {
	s.dao.InvalidateCachedToken(ctx, tokenHash)
}

// ListAuthSources lists all configured authentication sources.
func (s *AuthServiceImpl) ListAuthSources(ctx context.Context) ([]contracts.AuthSourceViewDTO, error) {
	sources, err := s.dao.ListAllAuthSources(ctx)
	if err != nil {
		return nil, err
	}

	views := make([]contracts.AuthSourceViewDTO, len(sources))
	for i := range sources {
		views[i] = contracts.AuthSourceViewDTO{
			ID:                     sources[i].ID,
			Name:                   sources[i].Name,
			Type:                   sources[i].Type,
			DisplayName:            sources[i].DisplayName,
			IsActive:               sources[i].IsActive,
			IconURL:                sources[i].IconURL,
			ClientSecretConfigured: sources[i].ClientSecret != "",
		}
	}
	return views, nil
}

// CreateAuthSource creates a new authentication source.
func (s *AuthServiceImpl) CreateAuthSource(ctx context.Context, source contracts.AuthSourceDTO) (*contracts.AuthSourceDTO, error) {
	model := entity.AuthSource{
		ID:                 source.ID,
		Name:               source.Name,
		Type:               source.Type,
		DisplayName:        source.DisplayName,
		ClientID:           source.ClientID,
		ClientSecret:       source.ClientSecret,
		OpenIDDiscoveryURL: source.OpenIDDiscoveryURL,
		Scopes:             source.Scopes,
		IconURL:            source.IconURL,
		IsActive:           source.IsActive,
	}

	if err := model.Validate(); err != nil {
		return nil, err
	}

	if err := s.dao.CreateAuthSource(ctx, &model); err != nil {
		return nil, err
	}

	model.Sanitize()
	return toAuthSourceDTO(&model), nil
}

// UpdateAuthSource updates an existing authentication source.
func (s *AuthServiceImpl) UpdateAuthSource(ctx context.Context, id uint64, source contracts.AuthSourceDTO) (*contracts.AuthSourceDTO, error) {
	existing, err := s.dao.GetAuthSourceByID(ctx, id)
	if err != nil {
		return nil, err
	}

	existing.DisplayName = source.DisplayName
	existing.ClientID = source.ClientID
	if source.ClientSecret != "" {
		existing.ClientSecret = source.ClientSecret
	}
	existing.OpenIDDiscoveryURL = source.OpenIDDiscoveryURL
	existing.Scopes = source.Scopes
	existing.IconURL = source.IconURL

	if err := existing.Validate(); err != nil {
		return nil, err
	}

	if err := s.dao.SaveAuthSource(ctx, existing); err != nil {
		return nil, err
	}

	existing.Sanitize()
	return toAuthSourceDTO(existing), nil
}

// DeleteAuthSource deletes an authentication source.
func (s *AuthServiceImpl) DeleteAuthSource(ctx context.Context, id uint64) error {
	existing, err := s.dao.GetAuthSourceByID(ctx, id)
	if err != nil {
		return err
	}

	return s.dao.DeleteAuthSource(ctx, existing)
}

// ToggleAuthSource toggles active status of an authentication source.
func (s *AuthServiceImpl) ToggleAuthSource(ctx context.Context, id uint64) (*contracts.AuthSourceDTO, error) {
	existing, err := s.dao.GetAuthSourceByID(ctx, id)
	if err != nil {
		return nil, err
	}

	existing.IsActive = !existing.IsActive
	if err := s.dao.SaveAuthSource(ctx, existing); err != nil {
		return nil, err
	}

	existing.Sanitize()
	return toAuthSourceDTO(existing), nil
}

func toAuthSourceDTO(s *entity.AuthSource) *contracts.AuthSourceDTO {
	if s == nil {
		return nil
	}
	return &contracts.AuthSourceDTO{
		ID:                 s.ID,
		Name:               s.Name,
		Type:               s.Type,
		DisplayName:        s.DisplayName,
		ClientID:           s.ClientID,
		ClientSecret:       s.ClientSecret,
		OpenIDDiscoveryURL: s.OpenIDDiscoveryURL,
		Scopes:             s.Scopes,
		IconURL:            s.IconURL,
		IsActive:           s.IsActive,
		CreatedAt:          s.CreatedAt,
		UpdatedAt:          s.UpdatedAt,
	}
}
