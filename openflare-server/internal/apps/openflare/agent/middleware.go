// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package agent

import (
	"strings"

	"github.com/Rain-kl/Wavelet/internal/common/response"
	"github.com/Rain-kl/Wavelet/internal/model"
	"github.com/gin-gonic/gin"
)

const (
	agentTokenHeader    = "X-Agent-Token"
	agentNodeContextKey = "agent_node"
)

// AgentAuth validates X-Agent-Token against of_nodes.access_token.
func AgentAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		token := strings.TrimSpace(c.GetHeader(agentTokenHeader))
		node, err := AuthenticateAccessToken(c.Request.Context(), token)
		if err != nil {
			response.AbortUnauthorized(c, errInvalidAgentToken)
			return
		}
		c.Set(agentNodeContextKey, node)
		c.Next()
	}
}

// AgentRegisterAuth accepts either a node access token or the global discovery token.
func AgentRegisterAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		token := strings.TrimSpace(c.GetHeader(agentTokenHeader))
		if node, err := AuthenticateAccessToken(c.Request.Context(), token); err == nil {
			c.Set(agentNodeContextKey, node)
			c.Next()
			return
		}
		if err := ValidateDiscoveryToken(c.Request.Context(), token); err != nil {
			response.AbortUnauthorized(c, errInvalidDiscoveryToken)
			return
		}
		c.Set("discovery_enabled", true)
		c.Next()
	}
}

// AgentNodeFromContext returns the authenticated agent node.
func AgentNodeFromContext(c *gin.Context) (*model.OpenFlareNode, bool) {
	value, ok := c.Get(agentNodeContextKey)
	if !ok {
		return nil, false
	}
	node, ok := value.(*model.OpenFlareNode)
	return node, ok
}