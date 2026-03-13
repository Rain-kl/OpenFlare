package main

import (
	"atsflare/common"
	_ "atsflare/docs"
	"atsflare/middleware"
	"atsflare/model"
	"atsflare/router"
	"embed"
	"fmt"
	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-contrib/sessions/redis"
	"github.com/gin-gonic/gin"
	"os"
	"strconv"
)

//go:embed all:web/build
var buildFS embed.FS

//go:embed web/build/index.html
var indexPage []byte

// @title ATSFlare Server API
// @version 3.0
// @description ATSFlare Server 管理端与 Agent API 文档。
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
	common.SysLog("ATSFlare " + common.Version + " started")
	if os.Getenv("GIN_MODE") != "debug" {
		gin.SetMode(gin.ReleaseMode)
	}
	// Initialize SQL Database
	err := model.InitDB()
	if err != nil {
		common.FatalLog(err)
	}
	defer func() {
		err := model.CloseDB()
		if err != nil {
			common.FatalLog(err)
		}
	}()

	// Initialize Redis
	err = common.InitRedisClient()
	if err != nil {
		common.FatalLog(err)
	}

	// Initialize options
	model.InitOptionMap()

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
	common.SysLog(fmt.Sprintf("server config: port=%s gin_mode=%s log_level=%s sqlite_path=%s redis_enabled=%t upload_path=%s log_dir=%s agent_token_configured=%t node_offline_threshold=%s", port, gin.Mode(), common.GetLogLevel(), common.SQLitePath, common.RedisEnabled, common.UploadPath, valueOrDefault(*common.LogDir, "stdout"), common.AgentToken != "", common.NodeOfflineThreshold))
	common.SysLog(fmt.Sprintf("server listening on :%s", port))
	err = server.Run(":" + port)
	if err != nil {
		common.SysError(err.Error())
	}
}

func valueOrDefault(value string, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}
