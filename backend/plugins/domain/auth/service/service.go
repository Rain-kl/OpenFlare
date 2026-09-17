// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package service implements domain business services and orchestration for the auth plugin.
package service

import (
	"Wavelet/plugins/domain/auth/dao"
	"Wavelet/plugins/domain/auth/pow"
	"time"
)

// Service aggregates all domain services for the auth plugin.
type Service struct {
	DAO               *dao.DAO
	Session           *SessionService
	OAuth             *OAuthService
	OIDCProviderCache *OIDCProviderCache
	CapSettings       *CapSettingsManager
	CapManager        *CaptchaManager
	AuthSvc           *AuthServiceImpl
	AuthRegistry      *AuthRegistryImpl
}

// New creates a new Service container with all domain services wired up.
func New(d *dao.DAO, sessionCfg SessionConfig, capSecret []byte) *Service {
	sessionSvc := NewSessionService(sessionCfg, d)
	oidcCache := NewOIDCProviderCache()
	oauthSvc := NewOAuthService(d, oidcCache, sessionSvc)
	capSettings := NewCapSettingsManager(d)

	var capStore pow.Store
	if len(capSecret) > 0 {
		capStore = pow.NewMemoryStore(1 * time.Minute)
	}
	capMgr := NewCaptchaManager(capSecret, capStore, capSettings)

	authSvc := NewAuthService(d, nil, nil, nil, nil)
	authRegistry := NewAuthRegistry()

	return &Service{
		DAO:               d,
		Session:           sessionSvc,
		OAuth:             oauthSvc,
		OIDCProviderCache: oidcCache,
		CapSettings:       capSettings,
		CapManager:        capMgr,
		AuthSvc:           authSvc,
		AuthRegistry:      authRegistry,
	}
}
