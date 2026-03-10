package middleware

import (
	"atsflare/service"
	"github.com/gin-gonic/gin"
	"net/http"
)

func AgentAuth() func(c *gin.Context) {
	return func(c *gin.Context) {
		token := c.GetHeader("X-Agent-Token")
		node, err := service.AuthenticateAgentToken(token)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{
				"success": false,
				"message": "无权进行此操作，Agent Token 无效",
			})
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
		if node, err := service.AuthenticateAgentToken(token); err == nil {
			c.Set("agent_node", node)
			c.Next()
			return
		}
		if err := service.ValidateDiscoveryToken(token); err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{
				"success": false,
				"message": "无权进行此操作，注册 Token 无效",
			})
			c.Abort()
			return
		}
		c.Set("discovery_enabled", true)
		c.Next()
	}
}
