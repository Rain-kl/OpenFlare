// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package server 装载 OpenFlare 控制面业务路由：站点/区域、Cloudflare 接入、Pages 部署、
// WAF、节点与回源管理以及边缘协议接口。平台路径（user/admin/cap/health）由 Wavelet
// 域插件注册，本插件只挂 OpenFlare 自己的业务。
package server

import (
	"Wavelet/core"
	"Wavelet/core/contracts"
	"Wavelet/openflare/plugins/server/domain/observability/chwriter"
	ofrouter "Wavelet/openflare/plugins/server/httpapi"
	"Wavelet/openflare/plugins/server/kernel/credential"
	ofgeoip "Wavelet/openflare/plugins/server/kernel/geoip"
	"Wavelet/openflare/plugins/server/kernel/ofevents"
	"Wavelet/openflare/plugins/server/kernel/ofupload"
	"Wavelet/openflare/plugins/server/kernel/publicconfig"
	"Wavelet/openflare/plugins/server/kernel/repository"
	"Wavelet/openflare/plugins/server/kernel/repository/logstore"
	"Wavelet/openflare/plugins/server/kernel/runtimeconfig"
	oftask "Wavelet/openflare/plugins/server/kernel/task"
	"Wavelet/openflare/plugins/server/migrate"
	"Wavelet/pkg/logger"
	"context"
	"embed"
	"reflect"

	_ "Wavelet/docs"

	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

//go:embed migrate/postgres/*.sql migrate/sqlite/*.sql
var serverMigrations embed.FS

// Plugin 实现 core.Plugin，是 OpenFlare 控制面的装载入口。
type Plugin struct{}

// New 创建 server 插件。
func New() *Plugin { return &Plugin{} }

// Name 返回插件标识。
func (p *Plugin) Name() string { return "server" }

// Inject waits for platform services so Apply runs after Wavelet domain plugins.
func (p *Plugin) Inject() []reflect.Type {
	return []reflect.Type{
		reflect.TypeFor[contracts.DBService](),
		reflect.TypeFor[contracts.CacheService](),
		reflect.TypeFor[contracts.UserService](),
		reflect.TypeFor[contracts.AuthService](),
		reflect.TypeFor[contracts.TaskService](),
		reflect.TypeFor[contracts.StorageService](),
	}
}

// Apply 声明 OpenFlare 业务 HTTP 路由树、公共配置、推送事件与异步任务。
func (p *Plugin) Apply(ctx *core.Context) error {
	var chCfg runtimeconfig.ClickHouseConfig
	_ = ctx.Config().Bind("clickhouse", &chCfg)
	runtimeconfig.Set(runtimeconfig.Snapshot{
		SessionSecret:   ctx.Config().String("app.session_secret", ""),
		DatabaseEnabled: ctx.Config().Bool("database.enabled", false),
		ClickHouse:      chCfg,
	})
	credential.SetSessionSecret(runtimeconfig.SessionSecret())

	ctx.Migrations().Register("server", serverMigrations)
	if err := migrate.UpClickHouse(); err != nil {
		return err
	}

	core.Bind[contracts.DBService](ctx, repository.SetDBService)
	core.Bind[contracts.SystemConfigService](ctx, repository.SetSystemConfigService)
	core.Bind[contracts.TaskService](ctx, func(ts contracts.TaskService) {
		oftask.SetService(ts)
		repository.SetTaskService(ts)
	})
	core.Bind[contracts.UserService](ctx, repository.SetUserService)
	core.Bind[contracts.StorageService](ctx, ofupload.SetStorage)
	core.Bind[contracts.UploadService](ctx, ofupload.SetUploadService)

	core.Provide[contracts.PublicConfigProvider](ctx, publicconfig.New(ctx))
	if pr, err := core.Inject[contracts.PushRegistry](ctx); err == nil {
		for _, meta := range ofevents.All() {
			pr.RegisterBuiltInEvent(meta)
		}
		_ = pr.SyncEvents(ctx.GoContext())
	}

	registerOpenFlareTasks(ctx)
	bindLogstore(ctx)

	if err := ofgeoip.EnsureRuntimeProvider(ctx.GoContext()); err != nil {
		logger.ErrorF(ctx.GoContext(), "[server] init GeoIP provider failed: %v", err)
	}
	chwriter.Init(ctx.GoContext())
	ctx.OnDispose(func() error {
		return chwriter.Stop(context.Background())
	})

	var auth contracts.AuthService
	if err := core.Using[contracts.AuthService](ctx, func(s contracts.AuthService) { auth = s }); err != nil {
		return err
	}
	repository.SetAuthService(auth)
	ofrouter.RegisterV1Routes(ctx.Router().Group("/api/v1"), auth)
	ofrouter.RegisterRoutes(ctx.Router().Group("/api/v1"), auth)

	env := ctx.Config().String("app.env", "production")
	if env != "production" && env != "prod" {
		ctx.Router().GET("/api/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))
	}
	return nil
}

func bindLogstore(ctx *core.Context) {
	logstore.SetConfigReader(func(goCtx context.Context, key string) (string, error) {
		cfg, err := repository.GetSystemConfigByKey(goCtx, key)
		if err != nil {
			return "", err
		}
		return cfg.Value, nil
	})
	logstore.Init(ctx.GoContext())
}
