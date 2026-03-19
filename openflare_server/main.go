package main

import (
	"context"
	"embed"
	"fmt"
	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-contrib/sessions/redis"
	"github.com/gin-gonic/gin"
	"log/slog"
	"openflare/common"
	_ "openflare/docs"
	"openflare/middleware"
	"openflare/model"
	"openflare/router"
	"openflare/service"
	"openflare/utils/geoip"
	"os"
	"strconv"
)

//go:embed all:web/build
var buildFS embed.FS

//go:embed web/build/index.html
var indexPage []byte

// @title OpenFlare Server API
// @version 3.0
// @description OpenFlare Server 管理端与 Agent API 文档。
// @BasePath /
// @schemes http https
// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
// @description 管理端可使用 Bearer Token，例如：Bearer <token>
// @securityDefinitions.apikey AgentTokenAuth
// @in header
// @name X-Agent-Token
// @description Agent API 使用节点专属 Agent Token 或全局 Discovery Token
func main() {
	common.SetupGinLog()
	slog.Info("OpenFlare started", "version", common.Version)
	if os.Getenv("GIN_MODE") != "debug" {
		gin.SetMode(gin.ReleaseMode)
	}
	// Initialize SQL Database
	err := model.InitDB()
	if err != nil {
		slog.Error("initialize database failed", "error", err)
		os.Exit(1)
	}
	defer func() {
		err := model.CloseDB()
		if err != nil {
			slog.Error("close database failed", "error", err)
			os.Exit(1)
		}
	}()

	// Initialize Redis
	err = common.InitRedisClient()
	if err != nil {
		slog.Error("initialize redis failed", "error", err)
		os.Exit(1)
	}

	// Initialize options
	model.InitOptionMap()
	geoip.InitGeoIP()
	backgroundCtx, cancelBackgroundTasks := context.WithCancel(context.Background())
	defer cancelBackgroundTasks()
	service.StartDatabaseAutoCleanupScheduler(backgroundCtx)

	// Initialize HTTP server
	server := gin.Default()
	//server.Use(gzip.Gzip(gzip.DefaultCompression))
	server.Use(middleware.CORS())

	// Initialize session store
	if common.RedisEnabled {
		opt := common.ParseRedisOption()
		store, _ := redis.NewStore(opt.MinIdleConns, opt.Network, opt.Addr, opt.Password, []byte(common.SessionSecret))
		server.Use(sessions.Sessions("session", store))
	} else {
		store := cookie.NewStore([]byte(common.SessionSecret))
		server.Use(sessions.Sessions("session", store))
	}

	router.SetRouter(server, buildFS, indexPage)
	var port = os.Getenv("PORT")
	if port == "" {
		port = strconv.Itoa(*common.Port)
	}
	dbBackend := "sqlite"
	if common.SQLDSN != "" {
		dbBackend = "postgres"
	}
	slog.Info("server config", "port", port, "gin_mode", gin.Mode(), "log_level", common.GetLogLevel(), "db_backend", dbBackend, "sqlite_path", common.SQLitePath, "redis_enabled", common.RedisEnabled, "upload_path", common.UploadPath, "log_dir", valueOrDefault(*common.LogDir, "stdout"), "agent_token_configured", common.AgentToken != "", "node_offline_threshold", common.NodeOfflineThreshold)
	slog.Info("server listening", "address", fmt.Sprintf(":%s", port))
	err = server.Run(":" + port)
	if err != nil {
		slog.Error("server run failed", "error", err)
	}
}

func valueOrDefault(value string, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}
