// Copyright 2025 linux.do
// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package router

import (
	"context"
	"errors"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/Rain-kl/Wavelet/internal/apps/risk_control"
	"github.com/Rain-kl/Wavelet/internal/platform/bootstrap"
	router_root "github.com/Rain-kl/Wavelet/internal/router/root"
	v1 "github.com/Rain-kl/Wavelet/internal/router/v1"

	"github.com/Rain-kl/Wavelet/internal/apps/oauth"
	"github.com/Rain-kl/Wavelet/internal/infra/config"
	otel_trace "github.com/Rain-kl/Wavelet/pkg/trace"
	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/redis"
	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
)

// Serve 启动 HTTP API 服务。onStarted 仅会在 HTTP 地址成功绑定后调用。
func Serve(onStarted func()) {
	// 运行模式
	if config.Config.App.IsProduction() {
		gin.SetMode(gin.ReleaseMode)
	}

	// 初始化路由
	r := gin.New()
	// Legacy OpenFlare list endpoints register both /resource and /resource/; disable auto slash redirects.
	r.RedirectTrailingSlash = false
	r.Use(gin.Recovery())
	r.Use(corsMiddleware())

	cfg := config.Config.Redis
	addrs := cfg.Addrs
	sessionAddr := "localhost:6379"
	if len(addrs) > 0 {
		sessionAddr = addrs[0]
	}

	sessionStore, err := redis.NewStoreWithDB(
		cfg.MinIdleConn,
		"tcp",
		sessionAddr,
		cfg.Username,
		cfg.Password,
		strconv.Itoa(cfg.DB),
		[]byte(config.Config.App.SessionSecret),
	)
	if err != nil {
		log.Fatalf("[API] init session store failed: %v\n", err)
	}

	// 设置 Session Redis Key 前缀
	if cfg.KeyPrefix != "" {
		if err := redis.SetKeyPrefix(sessionStore, cfg.KeyPrefix+"session:"); err != nil {
			log.Printf("[API] set session key prefix failed: %v\n", err)
		}
	}

	sessionStore.Options(oauth.GetSessionOptions(config.Config.App.SessionAge))

	r.Use(sessions.Sessions(config.Config.App.SessionCookieName, sessionStore))

	// 补充中间件
	r.Use(otelgin.Middleware(config.Config.App.AppName), errorHandlerMiddleware(), loggerMiddleware(), risk_control.RiskControlMiddleware())

	registerRoutes(r)

	srv := &http.Server{
		Addr:              config.Config.App.Addr,
		Handler:           r,
		ReadHeaderTimeout: 10 * time.Second,
	}

	listener, err := (&net.ListenConfig{}).Listen(context.Background(), "tcp", config.Config.App.Addr)
	if err != nil {
		log.Fatalf("[API] server failed to listen on %s: %v\n", config.Config.App.Addr, err)
	}
	if onStarted != nil {
		onStarted()
	}

	go func() {
		log.Printf("[API] server listening on %s\n", config.Config.App.Addr)
		if err := srv.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("[API] server failed: %v\n", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	shutdownCtx, cancel := context.WithTimeout(context.Background(), time.Duration(config.Config.App.GracefulShutdownTimeout)*time.Second)

	otel_trace.Shutdown(shutdownCtx)

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("[API] server forced to shutdown: %v\n", err)
		bootstrap.Stop(shutdownCtx)
		cancel()
		os.Exit(1)
	}
	bootstrap.Stop(shutdownCtx)
	cancel()

	log.Println("[API] server exited")
}

func registerRoutes(r *gin.Engine) {
	// Register custom root routes, Swagger, and frontend serving
	router_root.RegisterRootRoutes(r)

	apiGroup := r.Group(config.Config.App.APIPrefix)
	{
		apiV1Router := apiGroup.Group("/v1")
		{
			v1.RegisterV1Routes(apiV1Router, apiGroup)
		}
	}
}
