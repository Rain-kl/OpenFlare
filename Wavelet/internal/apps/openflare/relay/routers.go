// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package relay

import (
	"github.com/Rain-kl/Wavelet/internal/apps/openflare/compat"
	ofws "github.com/Rain-kl/Wavelet/internal/apps/openflare/websocket"
	"github.com/Rain-kl/Wavelet/internal/model"
	"github.com/gin-gonic/gin"
)


// PostHeartbeat handles POST /relay/heartbeat.
func PostHeartbeat(c *gin.Context) {
	var payload HeartbeatPayload
	if !compat.BindJSON(c, &payload) {
		return
	}
	payload.IP = resolveReportedNodeIP(payload.IP, c.Request.RemoteAddr)

	authNode, ok := c.Get(ctxRelayNodeKey)
	if !ok {
		compat.Unauthorized(c, errAgentTokenInvalid)
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

// GetWebSocket handles GET /relay/ws.
func GetWebSocket(c *gin.Context) {
	authNode, ok := c.Get(ctxRelayNodeKey)
	if !ok {
		compat.Unauthorized(c, errAgentTokenInvalid)
		return
	}
	node := authNode.(*model.OpenFlareNode)
	ofws.ServeRelay(c, node.NodeID)
}
