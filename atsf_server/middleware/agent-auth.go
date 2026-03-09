package middleware

import (
	"gin-template/common"
	"github.com/gin-gonic/gin"
	"net/http"
)

func AgentAuth() func(c *gin.Context) {
	return func(c *gin.Context) {
		token := c.GetHeader("X-Agent-Token")
		if common.AgentToken == "" {
			c.JSON(http.StatusUnauthorized, gin.H{
				"success": false,
				"message": "Agent Token 未配置",
			})
			c.Abort()
			return
		}
		if token == "" || token != common.AgentToken {
			c.JSON(http.StatusUnauthorized, gin.H{
				"success": false,
				"message": "无权进行此操作，Agent Token 无效",
			})
			c.Abort()
			return
		}
		c.Next()
	}
}
