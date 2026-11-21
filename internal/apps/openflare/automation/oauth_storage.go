// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package automation

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"time"

	"github.com/Rain-kl/Wavelet/internal/model"
	"github.com/Rain-kl/Wavelet/internal/repository"
	"github.com/ory/fosite"
	"gorm.io/gorm"
)

const (
	sessionCode    = "code"
	sessionPKCE    = "pkce"
	sessionAccess  = "access"
	sessionRefresh = "refresh"
)

type oauthStorage struct {
	site          instance
	cliRedirect   string
	revokeAccess  bool
	revokeRefresh bool
}

func (s *oauthStorage) GetClient(_ context.Context, id string) (fosite.Client, error) {
	redirect, err := s.clientRedirect(id)
	if err != nil {
		return nil, err
	}
	var redirects []string
	if redirect != "" {
		redirects = []string{redirect}
	}
	return &fosite.DefaultClient{
		ID: id, Public: true, RedirectURIs: redirects,
		GrantTypes: []string{grantCode, grantRefresh}, ResponseTypes: []string{oauthCode},
		Scopes: availableScopes(), Audience: []string{s.site.clientResource(id)},
	}, nil
}

func (s *oauthStorage) clientRedirect(id string) (string, error) {
	if id == cliClientID && s.cliRedirect == "" {
		return "", nil
	}
	if id == cliClientID {
		if _, ok := s.site.client(id, s.cliRedirect); !ok {
			return "", fosite.ErrNotFound
		}
		return s.cliRedirect, nil
	}
	var client publicClient
	if !s.site.unpack("automation-mcp-client:"+s.site.origin, id, &client) || !validMCPRedirect(client.RedirectURI) {
		return "", fosite.ErrNotFound
	}
	return client.RedirectURI, nil
}

// Only public clients are registered. JWT client assertions are disabled.
func (*oauthStorage) ClientAssertionJWTValid(context.Context, string) error {
	return fosite.ErrInvalidClient
}

func (*oauthStorage) SetClientAssertionJWT(context.Context, string, time.Time) error {
	return fosite.ErrInvalidClient
}

func saveOAuthSession(ctx context.Context, kind, signature string, request fosite.Requester, expiry fosite.TokenType) error {
	userID, err := strconv.ParseUint(request.GetSession().GetSubject(), 10, 64)
	if err != nil {
		return err
	}
	data, err := json.Marshal(request.Sanitize(requestFields(kind)))
	if err != nil {
		return err
	}
	return repository.CreateAutomationSession(ctx, model.AutomationSession{
		Kind: kind, Signature: digest(signature), RequestID: request.GetID(), UserID: userID,
		Data: string(data), Active: true, ExpiresAt: request.GetSession().GetExpiresAt(expiry),
	})
}

func requestFields(kind string) []string {
	switch kind {
	case sessionCode:
		return []string{"redirect_uri"}
	case sessionPKCE:
		return []string{"code_challenge", "code_challenge_method"}
	default:
		return nil
	}
}

func loadOAuthSession(ctx context.Context, kind, signature string) (model.AutomationSession, *fosite.Request, error) {
	row, err := repository.GetAutomationSession(ctx, kind, digest(signature), time.Now().UTC())
	if err != nil {
		return row, nil, oauthStorageError(err)
	}
	request := fosite.NewRequest()
	request.Session = &fosite.DefaultSession{}
	if err := json.Unmarshal([]byte(row.Data), request); err != nil {
		return row, nil, err
	}
	return row, request, nil
}

func oauthStorageError(err error) error {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return fosite.ErrNotFound
	}
	return err
}

func consumeOAuthSession(ctx context.Context, kind, signature string) error {
	err := repository.ConsumeAutomationSession(ctx, kind, digest(signature), time.Now().UTC())
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return fosite.ErrSerializationFailure
	}
	return err
}

func (*oauthStorage) CreateAuthorizeCodeSession(ctx context.Context, signature string, requester fosite.Requester) error {
	return saveOAuthSession(ctx, sessionCode, signature, requester, fosite.AuthorizeCode)
}

func (*oauthStorage) GetAuthorizeCodeSession(ctx context.Context, signature string, _ fosite.Session) (fosite.Requester, error) {
	row, request, err := loadOAuthSession(ctx, sessionCode, signature)
	if err == nil && !row.Active {
		return request, fosite.ErrInvalidatedAuthorizeCode
	}
	return request, err
}

func (*oauthStorage) InvalidateAuthorizeCodeSession(ctx context.Context, signature string) error {
	return consumeOAuthSession(ctx, sessionCode, signature)
}

func (*oauthStorage) CreatePKCERequestSession(ctx context.Context, signature string, requester fosite.Requester) error {
	return saveOAuthSession(ctx, sessionPKCE, signature, requester, fosite.AuthorizeCode)
}

func (*oauthStorage) GetPKCERequestSession(ctx context.Context, signature string, _ fosite.Session) (fosite.Requester, error) {
	_, request, err := loadOAuthSession(ctx, sessionPKCE, signature)
	return request, err
}

func (*oauthStorage) DeletePKCERequestSession(ctx context.Context, signature string) error {
	return repository.DeleteAutomationSession(ctx, sessionPKCE, digest(signature))
}

func (*oauthStorage) CreateAccessTokenSession(ctx context.Context, signature string, request fosite.Requester) error {
	return saveOAuthSession(ctx, sessionAccess, signature, request, fosite.AccessToken)
}

func (*oauthStorage) GetAccessTokenSession(ctx context.Context, signature string, _ fosite.Session) (fosite.Requester, error) {
	_, request, err := loadOAuthSession(ctx, sessionAccess, signature)
	return request, err
}

func (*oauthStorage) DeleteAccessTokenSession(ctx context.Context, signature string) error {
	return repository.DeleteAutomationSession(ctx, sessionAccess, digest(signature))
}

func (*oauthStorage) CreateRefreshTokenSession(ctx context.Context, signature, _ string, request fosite.Requester) error {
	return saveOAuthSession(ctx, sessionRefresh, signature, request, fosite.RefreshToken)
}

func (*oauthStorage) GetRefreshTokenSession(ctx context.Context, signature string, _ fosite.Session) (fosite.Requester, error) {
	row, request, err := loadOAuthSession(ctx, sessionRefresh, signature)
	if err == nil && !row.Active {
		return request, fosite.ErrInactiveToken
	}
	return request, err
}

func (*oauthStorage) DeleteRefreshTokenSession(ctx context.Context, signature string) error {
	return repository.DeleteAutomationSession(ctx, sessionRefresh, digest(signature))
}

func (*oauthStorage) RotateRefreshToken(ctx context.Context, requestID, signature string) error {
	if err := consumeOAuthSession(ctx, sessionRefresh, signature); err != nil {
		return err
	}
	return repository.RevokeAutomationSessions(ctx, requestID, sessionAccess)
}

func (s *oauthStorage) RevokeRefreshToken(ctx context.Context, requestID string) error {
	err := repository.RevokeAutomationSessions(ctx, requestID, sessionRefresh)
	s.revokeRefresh = err == nil
	return err
}

func (s *oauthStorage) RevokeAccessToken(ctx context.Context, requestID string) error {
	err := repository.RevokeAutomationSessions(ctx, requestID, sessionAccess)
	s.revokeAccess = err == nil
	return err
}
