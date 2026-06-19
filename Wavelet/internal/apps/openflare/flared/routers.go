// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package flared

import (
	"net/http"

	"github.com/Rain-kl/Wavelet/internal/apps/openflare/apiutil"
	ofws "github.com/Rain-kl/Wavelet/internal/apps/openflare/websocket"
	"github.com/Rain-kl/Wavelet/internal/common/response"
	"github.com/Rain-kl/Wavelet/internal/model"
	"github.com/gin-gonic/gin"
)

// PostHeartbeat handles POST /tunnel/heartbeat.
// @Summary 上报 Tunnel 心跳
// @Description Tunnel 客户端定期上报运行状态与中继连接信息，返回活跃配置元数据与隧道设置
// @Tags openflare-tunnel
// @Accept json
// @Produce json
// @Security TunnelTokenAuth
// @Param body body flared.HeartbeatPayload true "心跳载荷"
// @Success 200 {object} response.Any{data=flared.HeartbeatResponse} "心跳响应"
// @Failure 400 {object} response.Any "参数错误"
// @Failure 401 {object} response.Any "Tunnel Token 无效"
// @Failure 403 {object} response.Any "节点类型不匹配"
// @Router /api/v1/tunnel/heartbeat [post]
func PostHeartbeat(c *gin.Context) {
	var payload HeartbeatPayload
	if !apiutil.BindJSON(c, &payload) {
		return
	}

	authNode, ok := c.Get(ctxFlaredNodeKey)
	if !ok {
		response.AbortUnauthorized(c, errTunnelTokenInvalid)
		return
	}
	node := authNode.(*model.OpenFlareNode)

	result, err := Heartbeat(c.Request.Context(), node, payload)
	if apiutil.AbortBadRequestOnError(c, err) {
		return
	}
	c.JSON(http.StatusOK, response.OK(result))
}

// GetActiveConfig handles GET /tunnel/config/active.
// @Summary 获取活跃隧道配置
// @Description 返回 Tunnel 客户端当前应应用的完整路由配置（含中继列表与代理定义）
// @Tags openflare-tunnel
// @Produce json
// @Security TunnelTokenAuth
// @Success 200 {object} response.Any{data=flared.TunnelConfigResponse} "隧道配置"
// @Failure 400 {object} response.Any "参数错误"
// @Failure 401 {object} response.Any "Tunnel Token 无效"
// @Failure 403 {object} response.Any "节点类型不匹配"
// @Router /api/v1/tunnel/config/active [get]
func GetActiveConfig(c *gin.Context) {
	authNode, ok := c.Get(ctxFlaredNodeKey)
	if !ok {
		response.AbortUnauthorized(c, errTunnelTokenInvalid)
		return
	}
	node := authNode.(*model.OpenFlareNode)

	config, err := GetTunnelConfig(c.Request.Context(), node)
	if apiutil.AbortBadRequestOnError(c, err) {
		return
	}
	c.JSON(http.StatusOK, response.OK(config))
}

// PostApplyLog handles POST /tunnel/apply-log.
// @Summary 上报 Tunnel 配置下发结果
// @Description Tunnel 客户端上报配置应用结果，服务端记录下发日志
// @Tags openflare-tunnel
// @Accept json
// @Produce json
// @Security TunnelTokenAuth
// @Param body body flared.ApplyLogPayload true "下发结果载荷"
// @Success 200 {object} response.Any{data=model.OpenFlareApplyLog} "下发日志记录"
// @Failure 400 {object} response.Any "参数错误"
// @Failure 401 {object} response.Any "Tunnel Token 无效"
// @Failure 403 {object} response.Any "节点类型不匹配"
// @Router /api/v1/tunnel/apply-log [post]
func PostApplyLog(c *gin.Context) {
	var payload ApplyLogPayload
	if !apiutil.BindJSON(c, &payload) {
		return
	}
	if authNode, ok := c.Get(ctxFlaredNodeKey); ok {
		payload.NodeID = authNode.(*model.OpenFlareNode).NodeID
	}

	log, err := ReportApplyLog(c.Request.Context(), payload)
	if apiutil.AbortBadRequestOnError(c, err) {
		return
	}
	c.JSON(http.StatusOK, response.OK(log))
}

// GetWebSocket handles GET /tunnel/ws.
// @Summary 升级 Tunnel WebSocket 连接
// @Description 将已认证的 Tunnel 客户端连接升级为 WebSocket 长连接，用于配置推送
// @Tags openflare-tunnel
// @Security TunnelTokenAuth
// @Failure 401 {object} response.Any "Tunnel Token 无效"
// @Failure 403 {object} response.Any "节点类型不匹配"
// @Router /api/v1/tunnel/ws [get]
func GetWebSocket(c *gin.Context) {
	authNode, ok := c.Get(ctxFlaredNodeKey)
	if !ok {
		response.AbortUnauthorized(c, errTunnelTokenInvalid)
		return
	}
	node := authNode.(*model.OpenFlareNode)
	ofws.ServeFlared(c, node.NodeID)
}