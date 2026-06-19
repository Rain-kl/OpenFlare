// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package relay

import (
	"net/http"

	"github.com/Rain-kl/Wavelet/internal/apps/openflare/apiutil"
	ofws "github.com/Rain-kl/Wavelet/internal/apps/openflare/websocket"
	"github.com/Rain-kl/Wavelet/internal/common/response"
	"github.com/Rain-kl/Wavelet/internal/model"
	"github.com/gin-gonic/gin"
)

// PostHeartbeat handles POST /relay/heartbeat.
// @Summary 上报 Relay 心跳
// @Description Relay 节点定期上报运行状态与 frps 观测数据，返回运行时配置
// @Tags openflare-relay
// @Accept json
// @Produce json
// @Security AgentTokenAuth
// @Param body body relay.HeartbeatPayload true "心跳载荷"
// @Success 200 {object} response.Any{data=relay.HeartbeatResponse} "心跳响应"
// @Failure 400 {object} response.Any "参数错误"
// @Failure 401 {object} response.Any "Agent Token 无效"
// @Failure 403 {object} response.Any "节点类型不匹配"
// @Router /api/v1/relay/heartbeat [post]
func PostHeartbeat(c *gin.Context) {
	var payload HeartbeatPayload
	if !apiutil.BindJSON(c, &payload) {
		return
	}
	payload.IP = resolveReportedNodeIP(payload.IP, c.Request.RemoteAddr)

	authNode, ok := c.Get(ctxRelayNodeKey)
	if !ok {
		response.AbortUnauthorized(c, errAgentTokenInvalid)
		return
	}
	node := authNode.(*model.OpenFlareNode)

	result, err := Heartbeat(c.Request.Context(), node, payload)
	if apiutil.AbortBadRequestOnError(c, err) {
		return
	}
	c.JSON(http.StatusOK, response.OK(result))
}

// GetWebSocket handles GET /relay/ws.
// @Summary 升级 Relay WebSocket 连接
// @Description 将已认证的 Relay 连接升级为 WebSocket 长连接，用于配置推送
// @Tags openflare-relay
// @Security AgentTokenAuth
// @Failure 401 {object} response.Any "Agent Token 无效"
// @Failure 403 {object} response.Any "节点类型不匹配"
// @Router /api/v1/relay/ws [get]
func GetWebSocket(c *gin.Context) {
	authNode, ok := c.Get(ctxRelayNodeKey)
	if !ok {
		response.AbortUnauthorized(c, errAgentTokenInvalid)
		return
	}
	node := authNode.(*model.OpenFlareNode)
	ofws.ServeRelay(c, node.NodeID)
}