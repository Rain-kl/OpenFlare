package middleware

import (
	"strings"

	"github.com/rain-kl/openflare/openflare-server/common"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

func CORS() gin.HandlerFunc {
	config := cors.DefaultConfig()
	config.AllowCredentials = true
	config.AllowHeaders = []string{"Origin", "Content-Length", "Content-Type", "Authorization", "OpenFlare-Token", "X-Agent-Token", "Accept"}
	config.AllowOriginFunc = func(origin string) bool {
		serverAddr := strings.TrimRight(common.ServerAddress, "/")
		if serverAddr == "" {
			return true
		}
		if origin == serverAddr {
			return true
		}
		return false
	}
	return cors.New(config)
}
