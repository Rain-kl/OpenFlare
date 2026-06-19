// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package flared

import (
	"context"
	"errors"
	"strings"

	"github.com/Rain-kl/Wavelet/internal/common/response"
	"github.com/Rain-kl/Wavelet/internal/model"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

const ctxFlaredNodeKey = "flared_node"

// TunnelAuth authenticates flared requests using X-Tunnel-Token and verifies tunnel_client type.
func TunnelAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		token := strings.TrimSpace(c.GetHeader("X-Tunnel-Token"))
		node, err := authenticateAccessToken(c.Request.Context(), token)
		if err != nil {
			response.AbortUnauthorized(c, errTunnelTokenInvalid)
			return
		}
		if node.NodeType != "tunnel_client" {
			response.AbortForbidden(c, errTunnelNodeTypeMismatch)
			return
		}
		c.Set(ctxFlaredNodeKey, node)
		c.Next()
	}
}

func authenticateAccessToken(ctx context.Context, token string) (*model.OpenFlareNode, error) {
	if token == "" {
		return nil, errors.New("missing tunnel token")
	}
	node, err := model.GetOpenFlareNodeByAccessToken(ctx, token)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("invalid tunnel token")
		}
		return nil, err
	}
	return node, nil
}