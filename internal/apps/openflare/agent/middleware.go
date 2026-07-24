// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package agent

import (
	"strings"

	"github.com/Rain-kl/Wavelet/internal/model"
	"github.com/Rain-kl/Wavelet/internal/shared/response"
	"github.com/gin-gonic/gin"
)

const (
	agentTokenHeader    = "X-Agent-Token" //nolint:gosec // HTTP header name, not a credential value
	agentNodeContextKey = "agent_node"
)

// Auth validates X-Agent-Token against of_nodes.access_token.
func Auth() gin.HandlerFunc {
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

// RegisterAuth accepts either a node access token or the global discovery token.
func RegisterAuth() gin.HandlerFunc {
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

// NodeFromContext returns the authenticated agent node.
func NodeFromContext(c *gin.Context) (*model.OpenFlareNode, bool) {
	value, ok := c.Get(agentNodeContextKey)
	if !ok {
		return nil, false
	}
	node, ok := value.(*model.OpenFlareNode)
	return node, ok
}
