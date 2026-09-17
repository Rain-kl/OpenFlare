// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package admin provides the system management console, diagnostics, audit logging, and configuration hot-reloading domain plugin for Cordis.
package admin

import (
	"Wavelet/core"
	"Wavelet/core/contracts"
	"Wavelet/core/extpoints"
	"Wavelet/pkg/ginutil"
	"Wavelet/plugins/domain/admin/handler"
	"Wavelet/plugins/domain/admin/model"
	"Wavelet/plugins/domain/admin/service"
	"embed"
	"reflect"

	"github.com/gin-gonic/gin"
)

// SystemConfig aliases model.SystemConfig for external compatibility.
type SystemConfig = model.SystemConfig

// TaskExecution aliases model.TaskExecution for external compatibility.
type TaskExecution = model.TaskExecution

// Schedule aliases model.Schedule for external compatibility.
type Schedule = model.Schedule

//go:embed migrations/*/*.sql
var adminMigrations embed.FS

// Option configures the admin plugin.
type Option func(*Plugin)

// Plugin implements core.Plugin to provide system administration and management APIs.
type Plugin struct{}

// New creates a new admin domain plugin.
func New(opts ...Option) *Plugin {
	p := &Plugin{}
	for _, opt := range opts {
		if opt != nil {
			opt(p)
		}
	}
	return p
}

// Name returns the unique identifier for the admin domain plugin.
func (p *Plugin) Name() string {
	return "admin"
}

// Inject declares required dependencies for the admin domain plugin.
func (p *Plugin) Inject() []reflect.Type {
	return []reflect.Type{
		reflect.TypeFor[contracts.DBService](),
		reflect.TypeFor[contracts.CacheService](),
		reflect.TypeFor[contracts.UserService](),
		reflect.TypeFor[contracts.AuthService](),
	}
}

// Manifest returns the plugin metadata.
func (p *Plugin) Manifest() core.Manifest {
	return core.Manifest{
		Name:        "admin",
		Version:     "1.0.0",
		Description: "System administration console, diagnostic monitoring, and configuration hot-reload plugin",
		Author:      "Wavelet Team",
	}
}

// DeclareConfig declares configuration bindings consumed by the admin plugin.
func (p *Plugin) DeclareConfig() []core.ConfigBinding {
	return []core.ConfigBinding{
		{Prefix: "database", Target: &model.DatabaseConfig{}},
		{Prefix: "clickhouse", Target: &model.ClickHouseConfig{}},
	}
}

// Apply registers admin routes, tasks, schedules, and settings into the Context.
func (p *Plugin) Apply(ctx *core.Context) error {
	var dbCfg model.DatabaseConfig
	_ = ctx.Config().Bind("database", &dbCfg)
	service.SetDBConfig(dbCfg)

	var chCfg model.ClickHouseConfig
	_ = ctx.Config().Bind("clickhouse", &chCfg)
	service.SetClickHouseConfig(chCfg)

	core.Bind[contracts.DBService](ctx, service.SetDBService)
	core.Bind[contracts.CacheService](ctx, service.SetCacheService)
	core.Bind[contracts.UserService](ctx, service.SetUserService)
	core.Bind[contracts.AuthService](ctx, service.SetAuthService)
	core.Bind[contracts.TaskService](ctx, service.SetTaskService)
	core.Bind[contracts.StorageService](ctx, service.SetStorageService)
	core.Bind[contracts.RiskControlService](ctx, service.SetRiskControlService)
	service.SetEventEmitter(ctx.Events().Emit)
	core.Provide[contracts.PublicConfigProvider](ctx, service.PublicConfigAdapter{})
	core.Provide[contracts.SystemConfigService](ctx, service.SystemConfigServiceImpl{})

	ctx.OnDispose(func() error {
		service.ResetServices()
		return nil
	})

	// 0a. Dynamic Auth Middlewares
	denyAuth := ginutil.AuthUnavailable()
	var loginMW gin.HandlerFunc = func(c *gin.Context) {
		if authSvc := service.GetAuthService(c.Request.Context()); authSvc != nil {
			if mw, ok := authSvc.RequireAuthMiddleware().(gin.HandlerFunc); ok {
				mw(c)
				return
			}
		}
		denyAuth(c)
	}
	var adminMW gin.HandlerFunc = func(c *gin.Context) {
		if authSvc := service.GetAuthService(c.Request.Context()); authSvc != nil {
			if mw, ok := authSvc.RequireAdminMiddleware().(gin.HandlerFunc); ok {
				mw(c)
				return
			}
		}
		denyAuth(c)
	}

	// 0b. Register migrations
	ctx.Migrations().Register("admin", adminMigrations)

	// 1. Register Admin HTTP Routes
	adminRouter := ctx.Router().Group("/api/v1/admin", loginMW, adminMW)
	handler.RegisterRoutes(adminRouter)

	// Register robots.txt public route
	ctx.Router().GET("/robots.txt", handler.GetRobotsTXT)
	ctx.Router().RegisterWhitelist("/robots.txt")

	// 2. Register Background Tasks
	const defaultCleanupRetry = 3
	ctx.Task().Register(service.LogDBSwitchTask, &service.LogDBSwitchHandler{}, extpoints.WithTaskMeta(service.LogDBSwitchMeta))
	ctx.Task().Register(service.SystemCleanupTask, &service.SystemCleanupHandler{}, extpoints.WithTaskMeta(service.SystemCleanupMeta), extpoints.WithTaskRetry(defaultCleanupRetry))

	// 2.1 Register Cron Schedule
	ctx.Schedule().RegisterCron("0 3 * * *", service.SystemCleanupTask, nil)

	// 3. Register Settings Schemas
	ctx.Settings().Register(extpoints.SettingSchema{
		Key:         "admin.system_cleanup_cron",
		Default:     "0 3 * * *",
		Description: "Cron expression for nightly system logs and expired tokens cleanup",
		Type:        "string",
		Category:    "maintenance",
	})

	return nil
}
