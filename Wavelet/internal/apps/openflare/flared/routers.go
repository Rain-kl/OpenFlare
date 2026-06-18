// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package flared

import (
	"github.com/Rain-kl/Wavelet/internal/apps/openflare/compat"
	ofws "github.com/Rain-kl/Wavelet/internal/apps/openflare/websocket"
	"github.com/Rain-kl/Wavelet/internal/model"
	"github.com/gin-gonic/gin"
)


// PostHeartbeat handles POST /flared/heartbeat.
func PostHeartbeat(c *gin.Context) {
	var payload HeartbeatPayload
	if !compat.BindJSON(c, &payload) {
		return
	}

	authNode, ok := c.Get(ctxFlaredNodeKey)
	if !ok {
		compat.Unauthorized(c, errTunnelTokenInvalid)
		return
	}
	node := authNode.(*model.OpenFlareNode)

	result, err := Heartbeat(c.Request.Context(), node, payload)
	if err != nil {
		compat.Fail(c, err.Error())
		return
	}
	compat.OK(c, result)
}

// GetActiveConfig handles GET /flared/config/active.
func GetActiveConfig(c *gin.Context) {
	authNode, ok := c.Get(ctxFlaredNodeKey)
	if !ok {
		compat.Unauthorized(c, errTunnelTokenInvalid)
		return
	}
	node := authNode.(*model.OpenFlareNode)

	config, err := GetTunnelConfig(c.Request.Context(), node)
	if err != nil {
		compat.Fail(c, "无法生成隧道配置: "+err.Error())
		return
	}
	compat.OK(c, config)
}

// PostApplyLog handles POST /flared/apply-log.
func PostApplyLog(c *gin.Context) {
	var payload ApplyLogPayload
	if !compat.BindJSON(c, &payload) {
		return
	}
	if authNode, ok := c.Get(ctxFlaredNodeKey); ok {
		payload.NodeID = authNode.(*model.OpenFlareNode).NodeID
	}

	log, err := ReportApplyLog(c.Request.Context(), payload)
	if err != nil {
		compat.Fail(c, err.Error())
		return
	}
	compat.OK(c, log)
}

// GetWebSocket handles GET /flared/ws.
func GetWebSocket(c *gin.Context) {
	authNode, ok := c.Get(ctxFlaredNodeKey)
	if !ok {
		compat.Unauthorized(c, errTunnelTokenInvalid)
		return
	}
	node := authNode.(*model.OpenFlareNode)
	ofws.ServeFlared(c, node.NodeID)
}
