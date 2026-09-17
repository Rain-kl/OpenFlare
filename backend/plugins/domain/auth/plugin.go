// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package auth provides the authentication, OAuth, session management, and access token domain plugin for Cordis.
package auth

import (
	"Wavelet/core"
	"Wavelet/core/contracts"
	"Wavelet/core/extpoints"
	"Wavelet/plugins/domain/auth/controller"
	"Wavelet/plugins/domain/auth/dao"
	"Wavelet/plugins/domain/auth/service"
	"context"
	"embed"
	"reflect"
)

//go:embed migrations/*/*.sql
var authMigrations embed.FS

// Option configures the auth plugin.
type Option func(*Plugin)

// WithAuthService sets a custom AuthService implementation.
func WithAuthService(svc contracts.AuthService) Option {
	return func(p *Plugin) {
		p.authSvc = svc
	}
}

// WithAuthRegistry sets a custom AuthRegistry implementation.
func WithAuthRegistry(reg contracts.AuthRegistry) Option {
	return func(p *Plugin) {
		p.authRegistry = reg
	}
}

// Plugin implements core.Plugin to provide authentication and OAuth domain services.
type Plugin struct {
	authSvc      contracts.AuthService
	authRegistry contracts.AuthRegistry
}

// New creates a new auth domain plugin.
func New(opts ...Option) *Plugin {
	p := &Plugin{}
	for _, opt := range opts {
		if opt != nil {
			opt(p)
		}
	}
	return p
}

// Name returns the unique identifier for the auth domain plugin.
func (p *Plugin) Name() string {
	return "auth"
}

// Inject declares required dependencies for the auth domain plugin.
func (p *Plugin) Inject() []reflect.Type {
	return []reflect.Type{
		reflect.TypeFor[contracts.DBService](),
		reflect.TypeFor[contracts.CacheService](),
	}
}

// Manifest returns the plugin metadata.
func (p *Plugin) Manifest() core.Manifest {
	return core.Manifest{
		Name:        "auth",
		Version:     "1.0.0",
		Description: "Authentication, OAuth, Session and Passkey domain plugin",
		Author:      "Wavelet Team",
	}
}

// DeclareConfig declares configuration bindings for the auth plugin.
func (p *Plugin) DeclareConfig() []core.ConfigBinding {
	return []core.ConfigBinding{
		{Prefix: "app", Target: &SessionConfig{}},
	}
}

// Apply registers the auth migrations, services, routes, and settings into the Context.
func (p *Plugin) Apply(ctx *core.Context) error {
	var cfg SessionConfig
	if err := ctx.Config().Bind("app", &cfg); err != nil {
		cfg = SessionConfig{
			SessionCookieName: "wavelet_session",
			SessionAge:        86400,
			SessionHTTPOnly:   true,
		}
	}

	d := dao.New(nil, nil, nil)
	core.Bind[contracts.DBService](ctx, d.SetDBService)
	core.Bind[contracts.CacheService](ctx, d.SetCacheService)
	core.Bind[contracts.LimiterService](ctx, d.SetLimiterService)
	ctx.OnDispose(func() error {
		d.SetDBService(nil)
		d.SetCacheService(nil)
		d.SetLimiterService(nil)
		return nil
	})

	var capSecret []byte
	if cfg.SessionSecret != "" {
		capSecret = []byte(cfg.SessionSecret)
	}
	svc := service.New(d, cfg, capSecret)

	if p.authSvc != nil {
		// Custom injected auth service override
		core.Provide[contracts.AuthService](ctx, p.authSvc)
	} else {
		core.Provide[contracts.AuthService](ctx, svc.AuthSvc)
	}

	if p.authRegistry != nil {
		core.Provide[contracts.AuthRegistry](ctx, p.authRegistry)
	} else {
		core.Provide[contracts.AuthRegistry](ctx, svc.AuthRegistry)
	}

	ctrl := controller.New(svc)
	setDefaultRuntime(d, svc, ctrl)

	// Register CaptchaService
	captchaSvc := service.NewCaptchaService(
		svc.CapManager,
		func(scope string) any { return ctrl.VerifyCaptcha(scope) },
		ctrl.Captcha.Challenge,
		ctrl.Captcha.Redeem,
	)
	core.Provide[contracts.CaptchaService](ctx, captchaSvc)

	// 1. Register migrations
	ctx.Migrations().Register("auth", authMigrations)

	// 2. Register Public / Auth Whitelist Endpoints
	publicEndpoints := []string{
		"/api/v1/oauth/sources",
		"/api/v1/oauth/login",
		"/api/v1/oauth/*/authorize",
		"/api/v1/oauth/:source/authorize",
		"/api/v1/oauth/callback",
		"/api/v1/user/login",
		"/api/v1/user/register",
		"/api/v1/user/send-email-code",
		"/api/v1/config/public",
		"/api/v1/cap/challenge",
		"/api/v1/cap/redeem",
		"/api/healthz",
		"/metrics",
	}
	ctrl.RegisterWhitelist(publicEndpoints...)
	ctx.Router().RegisterWhitelist(publicEndpoints...)

	// 3. Register HTTP Routes
	ctrl.RegisterRoutes(ctx.Router())

	// 4. Register Settings Schemas
	const (
		settingTypeInteger      = "integer"
		settingCategorySecurity = "security"
	)

	ctx.Settings().Register(extpoints.SettingSchema{
		Key:         "auth.session_age",
		Default:     86400 * 7,
		Description: "Default session lifetime in seconds",
		Type:        settingTypeInteger,
		Category:    settingCategorySecurity,
	})
	ctx.Settings().Register(extpoints.SettingSchema{
		Key:         "auth.login_rate_limit_max_attempts",
		Default:     5,
		Description: "Max login failure attempts before temporary IP lock",
		Type:        settingTypeInteger,
		Category:    settingCategorySecurity,
	})
	ctx.Settings().Register(extpoints.SettingSchema{
		Key:         "cap.login_enabled",
		Default:     false,
		Description: "Whether to require CAPTCHA verification for user login",
		Type:        "boolean",
		Category:    settingCategorySecurity,
	})
	ctx.Settings().Register(extpoints.SettingSchema{
		Key:         "cap.challenge_count",
		Default:     1,
		Description: "Number of PoW puzzle challenges to solve",
		Type:        settingTypeInteger,
		Category:    settingCategorySecurity,
	})

	// 5. Register Event Listeners for domain events
	ctx.Events().On(contracts.EventTopicUserStatusChanged, func(c context.Context, e contracts.UserStatusChangedEvent) error {
		svc.DAO.InvalidateCachedUser(c, e.UserID)
		return nil
	})

	ctx.Events().On(contracts.EventTopicUserDeleted, func(c context.Context, e contracts.UserDeletedEvent) error {
		svc.DAO.InvalidateCachedUser(c, e.TargetUserID)
		return nil
	})

	ctx.Events().On(contracts.EventTopicConfigChanged, func(_ any) {
		svc.CapSettings.Invalidate()
	})

	return nil
}
