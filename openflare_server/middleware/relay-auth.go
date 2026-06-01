package middleware

import (
	"github.com/gin-gonic/gin"
	"net/http"
	"openflare/service"
)

// RelayAuth authenticates Relay requests using the shared agent token,
// and verifies the node is a tunnel_relay type.
func RelayAuth() func(c *gin.Context) {
	return func(c *gin.Context) {
		token := c.GetHeader("X-Agent-Token")
		node, err := service.AuthenticateAccessToken(token)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{
				"success": false,
				"message": "无权进行此操作，Agent Token 无效",
			})
			c.Abort()
			return
		}
		if node.NodeType != "tunnel_relay" {
			c.JSON(http.StatusForbidden, gin.H{
				"success": false,
				"message": "此节点不是 TunnelRelay 类型",
			})
			c.Abort()
			return
		}
		c.Set("relay_node", node)
		c.Next()
	}
}
