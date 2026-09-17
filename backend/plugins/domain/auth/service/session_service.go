// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package service implements domain business services and orchestration for the auth plugin.
package service

import (
	"Wavelet/core/contracts"
	"Wavelet/plugins/domain/auth/consts"
	"Wavelet/plugins/domain/auth/dao"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"strconv"
	"strings"
	"sync"

	"github.com/gin-contrib/sessions"
	"github.com/google/uuid"
	gsessions "github.com/gorilla/sessions"
)

// SessionConfig defines session settings.
type SessionConfig struct {
	SessionCookieName string `config:"session_cookie_name" env:"APP_SESSION_COOKIE_NAME" default:"wavelet_session"`
	SessionSecret     string `config:"session_secret" env:"APP_SESSION_SECRET" secret:"true"`
	SessionDomain     string `config:"session_domain" env:"APP_SESSION_DOMAIN"`
	SessionAge        int    `config:"session_age" env:"APP_SESSION_AGE" default:"86400"`
	SessionHTTPOnly   bool   `config:"session_http_only" env:"APP_SESSION_HTTP_ONLY" default:"true"`
	SessionSecure     bool   `config:"session_secure" env:"APP_SESSION_SECURE"`
}

// SessionService manages HTTP session operations, cookies, and tokens.
type SessionService struct {
	mu     sync.RWMutex
	config SessionConfig
	dao    *dao.DAO
}

// NewSessionService creates a new SessionService.
func NewSessionService(cfg SessionConfig, d *dao.DAO) *SessionService {
	return &SessionService{
		config: cfg,
		dao:    d,
	}
}

// SetConfig updates the active session configuration.
func (s *SessionService) SetConfig(cfg SessionConfig) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.config = cfg
}

// Config returns the current session configuration.
func (s *SessionService) Config() SessionConfig {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.config
}

// GetSessionOptions 根据配置构建 Session 选项
func (s *SessionService) GetSessionOptions(maxAge int) sessions.Options {
	cfg := s.Config()
	return sessions.Options{
		Path:     "/",
		Domain:   cfg.SessionDomain,
		MaxAge:   maxAge,
		HttpOnly: cfg.SessionHTTPOnly,
		Secure:   cfg.SessionSecure,
		SameSite: http.SameSiteLaxMode,
	}
}

// StripCookieMaxAgeAndExpires 从 Set-Cookie 响应头中移除 Max-Age 和 Expires，从而使其成为浏览器会话 Cookie
func (s *SessionService) StripCookieMaxAgeAndExpires(header http.Header, cookieName string) {
	headers := header["Set-Cookie"]
	if len(headers) == 0 {
		return
	}

	newHeaders := make([]string, 0, len(headers))
	for _, h := range headers {
		if strings.HasPrefix(h, cookieName+"=") {
			parts := strings.Split(h, ";")
			newParts := make([]string, 0, len(parts))
			for _, p := range parts {
				trimmed := strings.TrimSpace(p)
				lower := strings.ToLower(trimmed)
				if strings.HasPrefix(lower, "max-age=") || strings.HasPrefix(lower, "expires=") {
					continue
				}
				newParts = append(newParts, p)
			}
			newHeaders = append(newHeaders, strings.Join(newParts, ";"))
		} else {
			newHeaders = append(newHeaders, h)
		}
	}
	header["Set-Cookie"] = newHeaders
}

// EnsureSessionToken returns or generates the session unique token.
func (s *SessionService) EnsureSessionToken(session sessions.Session) (string, bool) {
	token, ok := session.Get(consts.SessionTokenKey).(string)
	if !ok || token == "" {
		token = uuid.NewString()
		session.Set(consts.SessionTokenKey, token)
		return token, true
	}
	return token, false
}

// HashSessionToken hashes the session token using SHA-256.
func (s *SessionService) HashSessionToken(token string) string {
	h := sha256.New()
	h.Write([]byte(token))
	return hex.EncodeToString(h.Sum(nil))
}

// RotateSessionID forces session ID rotation to prevent session fixation attacks.
func (s *SessionService) RotateSessionID(session sessions.Session) {
	if inner, ok := session.(interface{ Session() *gsessions.Session }); ok {
		if sess := inner.Session(); sess != nil {
			sess.ID = ""
		}
	}
}

// CalculateSessionMaxAge dynamically calculates max age and whether it's a browser-session cookie.
func (s *SessionService) CalculateSessionMaxAge(ctx context.Context) (int, bool) {
	cfg := s.Config()
	maxAge := cfg.SessionAge
	isSessionCookie := false

	if s.dao != nil {
		val, err := s.dao.GetSystemConfigValue(ctx, "login_session_ttl_hours")
		if err == nil && val != "" {
			if ttlHours, err := strconv.Atoi(val); err == nil {
				switch {
				case ttlHours == -1:
					// 永不过期，设置为 10 年
					maxAge = 10 * 365 * 24 * 3600
				case ttlHours > 0:
					maxAge = ttlHours * 3600
				case ttlHours == 0:
					isSessionCookie = true
				}
			}
		}
	}
	return maxAge, isSessionCookie
}

// ApplyLoginSession writes the authenticated user into a freshly rotated session.
func (s *SessionService) ApplyLoginSession(ctx context.Context, session sessions.Session, user *contracts.UserDTO, extras ...map[string]any) (bool, error) {
	session.Clear()
	s.RotateSessionID(session)

	session.Set(consts.UserIDKey, strconv.FormatUint(user.ID, 10))
	session.Set(consts.UserNameKey, user.Username)
	if len(extras) > 0 {
		for key, value := range extras[0] {
			session.Set(key, value)
		}
	}

	maxAge, isSessionCookie := s.CalculateSessionMaxAge(ctx)
	session.Options(s.GetSessionOptions(maxAge))

	if err := session.Save(); err != nil {
		return false, err
	}

	return isSessionCookie, nil
}
