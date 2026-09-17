// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package testhelper

import (
	"context"

	"Wavelet/core/contracts"

	"github.com/gin-gonic/gin"
)

// StubAuth is a contracts.AuthService that admits every request.
type StubAuth struct {
	User    *contracts.UserDTO
	Sources []contracts.AuthSourceViewDTO
}

var _ contracts.AuthService = StubAuth{}

func passThrough() gin.HandlerFunc {
	return func(c *gin.Context) { c.Next() }
}

// RequireAuthMiddleware returns a passthrough middleware.
func (s StubAuth) RequireAuthMiddleware() any { return passThrough() }

// RequireAdminMiddleware returns a passthrough middleware.
func (s StubAuth) RequireAdminMiddleware() any { return passThrough() }

// DisallowTokenAuthMiddleware returns a passthrough middleware.
func (s StubAuth) DisallowTokenAuthMiddleware() any {
	return passThrough()
}

// GetCurrentUser returns the stub user.
func (s StubAuth) GetCurrentUser(context.Context) (*contracts.UserDTO, error) {
	return s.User, nil
}

// GetCurrentUserID returns the stub user ID.
func (s StubAuth) GetCurrentUserID(context.Context) (uint64, error) {
	if s.User == nil {
		return 0, nil
	}
	return s.User.ID, nil
}

// VerifyToken returns the stub user.
func (s StubAuth) VerifyToken(context.Context, string) (*contracts.UserDTO, error) {
	return s.User, nil
}

// CreateSession creates a stub session.
func (s StubAuth) CreateSession(context.Context, uint64, map[string]any) (string, error) {
	return "", nil
}

// RevokeToken revokes a stub token.
func (s StubAuth) RevokeToken(context.Context, string) error { return nil }

// RevokeUserSessions revokes stub user sessions.
func (s StubAuth) RevokeUserSessions(context.Context, uint64) error { return nil }

// InvalidateCachedUser invalidates stub cached user.
func (s StubAuth) InvalidateCachedUser(context.Context, uint64) {}

// InvalidateCachedToken invalidates stub cached token.
func (s StubAuth) InvalidateCachedToken(context.Context, string) {}
func (s StubAuth) ListAuthSources(context.Context) ([]contracts.AuthSourceViewDTO, error) {
	return s.Sources, nil
}
func (s StubAuth) CreateAuthSource(context.Context, contracts.AuthSourceDTO) (*contracts.AuthSourceDTO, error) {
	return nil, nil
}
func (s StubAuth) UpdateAuthSource(context.Context, uint64, contracts.AuthSourceDTO) (*contracts.AuthSourceDTO, error) {
	return nil, nil
}
func (s StubAuth) DeleteAuthSource(context.Context, uint64) error { return nil }
func (s StubAuth) ToggleAuthSource(context.Context, uint64) (*contracts.AuthSourceDTO, error) {
	return nil, nil
}
