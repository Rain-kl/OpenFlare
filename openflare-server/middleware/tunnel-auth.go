package middleware

import (
	"github.com/rain-kl/openflare/openflare-server/common/response"
	"github.com/rain-kl/openflare/openflare-server/service"

	"github.com/gin-gonic/gin"
)

// TunnelAuth authenticates OpenFlared client requests using the per-node
// tunnel_token carried in the X-Tunnel-Token header, and verifies the node is
// of the tunnel_client type.
func TunnelAuth() func(c *gin.Context) {
	return func(c *gin.Context) {
		token := c.GetHeader("X-Tunnel-Token")
		node, err := service.AuthenticateAccessToken(token)
		if err != nil {
			response.RespondUnauthorized(c, "无权进行此操作，Tunnel Token 无效")
			c.Abort()
			return
		}
		if node.NodeType != "tunnel_client" {
			response.RespondForbidden(c, "此节点不是 TunnelClient 类型")
			c.Abort()
			return
		}
		c.Set("flared_node", node)
		c.Next()
	}
}
