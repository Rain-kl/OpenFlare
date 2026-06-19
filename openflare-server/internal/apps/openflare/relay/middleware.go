// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package relay

import (
	"context"
	"errors"
	"strings"

	"github.com/Rain-kl/Wavelet/internal/common/response"
	"github.com/Rain-kl/Wavelet/internal/model"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

const ctxRelayNodeKey = "relay_node"

// RelayAuth authenticates relay requests using X-Agent-Token and verifies tunnel_relay type.
func RelayAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		token := strings.TrimSpace(c.GetHeader("X-Agent-Token"))
		node, err := authenticateAccessToken(c.Request.Context(), token)
		if err != nil {
			response.AbortUnauthorized(c, errAgentTokenInvalid)
			return
		}
		if node.NodeType != "tunnel_relay" {
			response.AbortForbidden(c, errRelayNodeTypeMismatch)
			return
		}
		c.Set(ctxRelayNodeKey, node)
		c.Next()
	}
}

func authenticateAccessToken(ctx context.Context, token string) (*model.OpenFlareNode, error) {
	if token == "" {
		return nil, errors.New("missing agent token")
	}
	node, err := model.GetOpenFlareNodeByAccessToken(ctx, token)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("invalid agent token")
		}
		return nil, err
	}
	return node, nil
}