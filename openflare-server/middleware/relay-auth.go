package middleware

import (
	"github.com/gin-gonic/gin"
	"github.com/rain-kl/openflare/openflare-server/common/response"
	"github.com/rain-kl/openflare/openflare-server/service"
)

// RelayAuth authenticates Relay requests using the shared agent token,
// and verifies the node is a tunnel_relay type.
func RelayAuth() func(c *gin.Context) {
	return func(c *gin.Context) {
		token := c.GetHeader("X-Agent-Token")
		node, err := service.AuthenticateAccessToken(token)
		if err != nil {
			response.RespondUnauthorized(c, "无权进行此操作，Agent Token 无效")
			c.Abort()
			return
		}
		if node.NodeType != "tunnel_relay" {
			response.RespondForbidden(c, "此节点不是 TunnelRelay 类型")
			c.Abort()
			return
		}
		c.Set("relay_node", node)
		c.Next()
	}
}
