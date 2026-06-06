package middleware

import (
	"github.com/gin-gonic/gin"
	"github.com/rain-kl/openflare/openflare-server/common/response"
	"github.com/rain-kl/openflare/openflare-server/service"
)

func AgentAuth() func(c *gin.Context) {
	return func(c *gin.Context) {
		token := c.GetHeader("X-Agent-Token")
		node, err := service.AuthenticateAccessToken(token)
		if err != nil {
			response.RespondUnauthorized(c, "无权进行此操作，Agent Token 无效")
			c.Abort()
			return
		}
		c.Set("agent_node", node)
		c.Next()
	}
}

func AgentRegisterAuth() func(c *gin.Context) {
	return func(c *gin.Context) {
		token := c.GetHeader("X-Agent-Token")
		if node, err := service.AuthenticateAccessToken(token); err == nil {
			c.Set("agent_node", node)
			c.Next()
			return
		}
		if err := service.ValidateDiscoveryToken(token); err != nil {
			response.RespondUnauthorized(c, "无权进行此操作，注册 Token 无效")
			c.Abort()
			return
		}
		c.Set("discovery_enabled", true)
		c.Next()
	}
}
