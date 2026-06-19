// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package agent

import (
	"net/http"
	"strconv"

	"github.com/Rain-kl/Wavelet/internal/apps/openflare/apiutil"
	"github.com/Rain-kl/Wavelet/internal/apps/openflare/pages"
	"github.com/Rain-kl/Wavelet/internal/apps/openflare/websocket"
	"github.com/Rain-kl/Wavelet/internal/common/response"
	"github.com/gin-gonic/gin"
)

// RegisterHandler registers or discovers an agent node.
// @Summary 注册或发现 Agent 节点
// @Description 使用节点 access token 重新注册，或使用全局 discovery token 发现新节点；请求头需携带 X-Agent-Token
// @Tags openflare-agent
// @Accept json
// @Produce json
// @Security AgentTokenAuth
// @Param body body agent.NodePayload true "节点上报数据"
// @Success 200 {object} response.Any{data=agent.RegistrationResponse} "注册成功"
// @Failure 400 {object} response.Any "参数错误"
// @Failure 401 {object} response.Any "Token 无效"
// @Router /api/v1/agent/nodes/register [post]
func RegisterHandler(c *gin.Context) {
	var payload NodePayload
	if !apiutil.BindJSON(c, &payload) {
		return
	}
	payload.IP = resolveReportedNodeIP(payload.IP, c.Request.RemoteAddr)

	var (
		result *RegistrationResponse
		err    error
	)
	if authNode, ok := AgentNodeFromContext(c); ok {
		result, err = RegisterWithAccessToken(c.Request.Context(), authNode, payload)
	} else {
		result, err = RegisterWithDiscovery(c.Request.Context(), payload)
	}
	if apiutil.AbortBadRequestOnError(c, err) {
		return
	}
	c.JSON(http.StatusOK, response.OK(result))
}

// HeartbeatHandler records agent heartbeat state.
// @Summary Agent 心跳上报
// @Description 上报节点状态、指标与健康事件，返回远程控制配置与活跃配置元信息
// @Tags openflare-agent
// @Accept json
// @Produce json
// @Security AgentTokenAuth
// @Param body body agent.NodePayload true "心跳数据"
// @Success 200 {object} response.Any{data=agent.HeartbeatResponse} "心跳成功"
// @Failure 400 {object} response.Any "参数错误"
// @Failure 401 {object} response.Any "Token 无效"
// @Router /api/v1/agent/nodes/heartbeat [post]
func HeartbeatHandler(c *gin.Context) {
	var payload NodePayload
	if !apiutil.BindJSON(c, &payload) {
		return
	}
	payload.IP = resolveReportedNodeIP(payload.IP, c.Request.RemoteAddr)

	authNode, ok := AgentNodeFromContext(c)
	if !ok {
		response.AbortUnauthorized(c, errInvalidAgentToken)
		return
	}

	heartbeat, err := HeartbeatNode(c.Request.Context(), authNode, payload)
	if apiutil.AbortBadRequestOnError(c, err) {
		return
	}
	c.JSON(http.StatusOK, response.OK(heartbeat))
}

// GetActiveConfigHandler returns the active configuration version.
// @Summary 获取活跃配置版本
// @Description 返回当前生效的完整配置包，供 Agent 拉取并应用
// @Tags openflare-agent
// @Produce json
// @Security AgentTokenAuth
// @Success 200 {object} response.Any{data=agent.ConfigResponse} "活跃配置"
// @Failure 400 {object} response.Any "参数错误"
// @Failure 401 {object} response.Any "Token 无效"
// @Router /api/v1/agent/config-versions/active [get]
func GetActiveConfigHandler(c *gin.Context) {
	if _, ok := AgentNodeFromContext(c); !ok {
		response.AbortUnauthorized(c, errNodeMissingFromContext)
		return
	}
	config, err := GetActiveConfig(c.Request.Context())
	if apiutil.AbortBadRequestOnError(c, err) {
		return
	}
	c.JSON(http.StatusOK, response.OK(config))
}

// SyncWAFIPGroupsHandler syncs WAF IP groups for an agent.
// @Summary 同步 WAF IP 组
// @Description 按 ID 与校验和增量同步 WAF IP 组定义
// @Tags openflare-agent
// @Accept json
// @Produce json
// @Security AgentTokenAuth
// @Param body body agent.WAFIPGroupSyncInput true "同步请求"
// @Success 200 {object} response.Any{data=agent.WAFIPGroupSyncResult} "同步结果"
// @Failure 400 {object} response.Any "参数错误"
// @Failure 401 {object} response.Any "Token 无效"
// @Router /api/v1/agent/waf/ip-groups/sync [post]
func SyncWAFIPGroupsHandler(c *gin.Context) {
	var input WAFIPGroupSyncInput
	if !apiutil.BindJSON(c, &input) {
		return
	}
	result, err := SyncWAFIPGroups(c.Request.Context(), input)
	if apiutil.AbortBadRequestOnError(c, err) {
		return
	}
	c.JSON(http.StatusOK, response.OK(result))
}

// ReportApplyLogHandler records an agent apply log entry.
// @Summary 上报配置应用日志
// @Description 记录 Agent 配置下发与应用结果
// @Tags openflare-agent
// @Accept json
// @Produce json
// @Security AgentTokenAuth
// @Param body body agent.ApplyLogPayload true "应用日志"
// @Success 200 {object} response.Any{data=model.OpenFlareApplyLog} "日志记录"
// @Failure 400 {object} response.Any "参数错误"
// @Failure 401 {object} response.Any "Token 无效"
// @Router /api/v1/agent/apply-logs [post]
func ReportApplyLogHandler(c *gin.Context) {
	var payload ApplyLogPayload
	if !apiutil.BindJSON(c, &payload) {
		return
	}
	if authNode, ok := AgentNodeFromContext(c); ok {
		payload.NodeID = authNode.NodeID
	}
	log, err := ReportApplyLog(c.Request.Context(), payload)
	if apiutil.AbortBadRequestOnError(c, err) {
		return
	}
	c.JSON(http.StatusOK, response.OK(log))
}

// DownloadPagesPackageHandler streams the Pages deployment artifact to an authenticated agent.
// @Summary 下载 Pages 部署包
// @Description 流式下载指定部署的静态资源压缩包，供 Agent 边缘分发
// @Tags openflare-agent
// @Produce application/octet-stream
// @Security AgentTokenAuth
// @Param deployment_id path int true "部署 ID"
// @Success 200 {file} binary "部署包文件"
// @Failure 400 {object} response.Any "参数错误"
// @Failure 401 {object} response.Any "Token 无效"
// @Router /api/v1/agent/pages/deployments/{deployment_id}/package [get]
func DownloadPagesPackageHandler(c *gin.Context) {
	deploymentID, ok := pagesDeploymentIDParam(c)
	if !ok {
		return
	}
	packageObj, fileName, err := pages.OpenDeploymentPackage(c.Request.Context(), deploymentID)
	if apiutil.AbortBadRequestOnError(c, err) {
		return
	}
	defer packageObj.Body.Close()
	c.Header("Content-Disposition", "attachment; filename="+fileName)
	if packageObj.ContentType != "" {
		c.Header("Content-Type", packageObj.ContentType)
	}
	c.DataFromReader(http.StatusOK, packageObj.ContentLength, packageObj.ContentType, packageObj.Body, nil)
}

func pagesDeploymentIDParam(c *gin.Context) (uint, bool) {
	raw := c.Param("deployment_id")
	if raw == "" {
		response.AbortBadRequest(c, "无效的 ID")
		return 0, false
	}
	id64, err := strconv.ParseUint(raw, 10, 64)
	if err != nil || id64 == 0 {
		response.AbortBadRequest(c, "无效的 ID")
		return 0, false
	}
	return uint(id64), true
}

// AgentWebSocketHandler upgrades an authenticated agent websocket connection.
// @Summary Agent WebSocket 连接
// @Description 升级为 WebSocket 长连接，用于实时推送配置同步、WAF IP 组等指令；需携带 X-Agent-Token
// @Tags openflare-agent
// @Security AgentTokenAuth
// @Failure 401 {object} response.Any "Token 无效"
// @Router /api/v1/agent/ws [get]
func AgentWebSocketHandler(c *gin.Context) {
	authNode, ok := AgentNodeFromContext(c)
	if !ok {
		response.AbortUnauthorized(c, errInvalidAgentToken)
		return
	}
	websocket.ServeAgent(c, authNode.NodeID, HandleWSStatus)
}