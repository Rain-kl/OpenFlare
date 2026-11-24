// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package agent

import (
	"context"
	"net/http"
	"strconv"
	"strings"

	ofexec "github.com/Rain-kl/Wavelet/internal/apps/openflare/exec"
	"github.com/Rain-kl/Wavelet/internal/apps/openflare/pages"
	"github.com/Rain-kl/Wavelet/internal/apps/openflare/ping"
	"github.com/Rain-kl/Wavelet/internal/apps/openflare/websocket"
	db "github.com/Rain-kl/Wavelet/internal/infra/persistence"
	"github.com/Rain-kl/Wavelet/internal/openflare/protocol"
	"github.com/Rain-kl/Wavelet/internal/shared/httpx"
	"github.com/Rain-kl/Wavelet/internal/shared/response"
	"github.com/gin-gonic/gin"
)

// RegisterHandler registers or discovers an agent node.
// @Summary 注册或发现 Agent 节点
// @Description 使用节点 access token 重新注册，或使用全局 discovery token 发现新节点；请求头需携带 X-Agent-Token。支持当前配置契约的 OpenFlare Edge 使用原节点 Token 注册时，可接替已登记的 OpenResty 节点。
// @Tags openflare-agent
// @Accept json
// @Produce json
// @Security AgentTokenAuth
// @Param body body protocol.RegisterNodeRequest true "节点上报数据；registration_key 为首次注册重试凭证"
// @Success 200 {object} response.Any{data=agent.RegistrationResponseDocs} "注册成功"
// @Failure 400 {object} response.Any "参数错误"
// @Failure 401 {object} response.Any "Token 无效"
// @Failure 403 {object} response.Any "节点已隔离"
// @Router /api/v1/agent/nodes/register [post]
func RegisterHandler(c *gin.Context) {
	var request protocol.RegisterNodeRequest
	if !httpx.BindJSON(c, &request) {
		return
	}
	payload := request.NodePayload
	payload.IP = resolveReportedNodeIP(payload.IP, requestClientIP(c))

	var (
		result *protocol.RegisterNodeResponse
		err    error
	)
	if authNode, ok := NodeFromContext(c); ok {
		result, err = RegisterWithAccessToken(c.Request.Context(), authNode, payload)
	} else {
		result, err = RegisterWithDiscovery(c.Request.Context(), payload, request.RegistrationKey)
	}
	if err != nil && err.Error() == errNodeQuarantined {
		c.Header(agentQuarantinedHeader, agentQuarantinedHeaderTrue)
		response.AbortForbidden(c, errNodeQuarantined)
		return
	}
	if httpx.AbortBadRequestOnError(c, err) {
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
// @Param body body protocol.NodePayload true "心跳数据"
// @Success 200 {object} response.Any{data=agent.HeartbeatResponse} "心跳成功"
// @Failure 400 {object} response.Any "参数错误"
// @Failure 401 {object} response.Any "Token 无效"
// @Failure 403 {object} response.Any "节点已隔离"
// @Router /api/v1/agent/nodes/heartbeat [post]
func HeartbeatHandler(c *gin.Context) {
	var payload protocol.NodePayload
	if !httpx.BindJSON(c, &payload) {
		return
	}
	payload.IP = resolveReportedNodeIP(payload.IP, requestClientIP(c))

	authNode, ok := NodeFromContext(c)
	if !ok {
		response.AbortUnauthorized(c, errInvalidAgentToken)
		return
	}

	heartbeat, err := HeartbeatNode(c.Request.Context(), authNode, payload)
	if err != nil && err.Error() == errNodeQuarantined {
		c.Header(agentQuarantinedHeader, agentQuarantinedHeaderTrue)
		response.AbortForbidden(c, errNodeQuarantined)
		return
	}
	if httpx.AbortBadRequestOnError(c, err) {
		return
	}
	c.JSON(http.StatusOK, response.OK(heartbeat))
}

// ReportApplyLogHandler records an agent apply log entry.
// @Summary 上报配置应用日志
// @Description 记录 Agent 配置下发与应用结果
// @Tags openflare-agent
// @Accept json
// @Produce json
// @Security AgentTokenAuth
// @Param body body protocol.ApplyLogPayload true "应用日志"
// @Success 200 {object} response.Any{data=model.OpenFlareApplyLog} "日志记录"
// @Failure 400 {object} response.Any "参数错误"
// @Failure 401 {object} response.Any "Token 无效"
// @Router /api/v1/agent/apply-logs [post]
func ReportApplyLogHandler(c *gin.Context) {
	var payload protocol.ApplyLogPayload
	if !httpx.BindJSON(c, &payload) {
		return
	}
	if authNode, ok := NodeFromContext(c); ok {
		payload.NodeID = authNode.NodeID
		payload.Engine = authNode.Engine
	}
	log, err := ReportApplyLog(c.Request.Context(), payload)
	if httpx.AbortBadRequestOnError(c, err) {
		return
	}
	c.JSON(http.StatusOK, response.OK(log))
}

// GetPagesProjectLatestHashHandler returns the hash of a project's currently active deployment.
// @Summary 查询 Pages 项目最新激活部署哈希
// @Description 按项目 ID 返回当前激活部署的包哈希（类似 latest 指针），Agent 无需关心具体部署 ID
// @Tags openflare-agent
// @Produce json
// @Security AgentTokenAuth
// @Param project_id path int true "Pages 项目 ID"
// @Success 200 {object} response.Any{data=protocol.PagesProjectLatestHashResponse}
// @Failure 400 {object} response.Any "参数错误"
// @Failure 401 {object} response.Any "Token 无效"
// @Failure 403 {object} response.Any "资源不在节点发布范围内"
// @Router /api/v1/agent/pages/projects/{project_id}/latest/hash [get]
func GetPagesProjectLatestHashHandler(c *gin.Context) {
	projectID, ok := pagesUintParam(c, "project_id")
	if !ok {
		return
	}

	ctx := db.WithPrimaryReads(c.Request.Context())
	if !allowPagesProject(ctx, c, projectID) {
		return
	}
	metadata, err := pages.GetProjectLatestPackageMetadata(ctx, projectID)
	if httpx.AbortBadRequestOnError(c, err) {
		return
	}
	c.JSON(http.StatusOK, response.OK(protocol.PagesProjectLatestHashResponse{
		ProjectID:    projectID,
		DeploymentID: metadata.DeploymentID,
		Hash:         metadata.Hash,
		PackageSize:  metadata.PackageSize,
		FileCount:    metadata.FileCount,
		TotalSize:    metadata.TotalSize,
	}))
}

// DownloadPagesProjectLatestPackageHandler streams the active deployment package for a project.
// @Summary 下载 Pages 项目最新激活部署包
// @Description 按项目 ID 下载当前激活部署的压缩包，供 Agent 边缘分发
// @Tags openflare-agent
// @Produce application/octet-stream
// @Security AgentTokenAuth
// @Param project_id path int true "Pages 项目 ID"
// @Success 200 {file} binary "部署包文件"
// @Failure 400 {object} response.Any "参数错误"
// @Failure 401 {object} response.Any "Token 无效"
// @Failure 403 {object} response.Any "资源不在节点发布范围内"
// @Router /api/v1/agent/pages/projects/{project_id}/latest/package [get]
func DownloadPagesProjectLatestPackageHandler(c *gin.Context) {
	projectID, ok := pagesUintParam(c, "project_id")
	if !ok {
		return
	}

	ctx := db.WithPrimaryReads(c.Request.Context())
	if !allowPagesProject(ctx, c, projectID) {
		return
	}
	packageObj, err := pages.OpenProjectLatestPackage(ctx, projectID)
	if httpx.AbortBadRequestOnError(c, err) {
		return
	}
	defer func() { _ = packageObj.Body.Close() }()
	c.Header("Content-Disposition", "attachment; filename="+packageObj.FileName)
	if packageObj.ContentType != "" {
		c.Header("Content-Type", packageObj.ContentType)
	}
	c.DataFromReader(http.StatusOK, packageObj.ContentLength, packageObj.ContentType, packageObj.Body, nil)
}

func allowPagesProject(ctx context.Context, c *gin.Context, projectID uint) bool {
	node, ok := NodeFromContext(c)
	if !ok || node.Engine != enginePingora {
		return true
	}
	allowed, err := pingoraPagesProjectAllowed(ctx, projectID)
	if httpx.AbortBadRequestOnError(c, err) {
		return false
	}
	if !allowed {
		response.AbortForbidden(c, errEdgeResourceOutsideRelease)
		return false
	}
	return true
}

func pagesUintParam(c *gin.Context, name string) (uint, bool) {
	raw := c.Param(name)
	if raw == "" {
		response.AbortBadRequest(c, "Invalid ID")
		return 0, false
	}
	id64, err := strconv.ParseUint(raw, 10, 64)
	if err != nil || id64 == 0 {
		response.AbortBadRequest(c, "Invalid ID")
		return 0, false
	}
	return uint(id64), true
}

// ReportPingResultHandler records a latency probe measurement reported over HTTP fallback.
// @Summary 上报延迟检测结果
// @Description Agent 通过 HTTP 回落上报单次 ICMP/TCP/HTTP 延迟检测结果；请求头需携带 X-Agent-Token
// @Tags openflare-agent
// @Accept json
// @Produce json
// @Security AgentTokenAuth
// @Param body body protocol.PingResultPayload true "延迟检测结果"
// @Success 200 {object} response.Any "上报成功"
// @Failure 400 {object} response.Any "参数错误"
// @Failure 401 {object} response.Any "Token 无效"
// @Router /api/v1/agent/ping/result [post]
func ReportPingResultHandler(c *gin.Context) {
	authNode, ok := NodeFromContext(c)
	if !ok {
		response.AbortUnauthorized(c, errInvalidAgentToken)
		return
	}
	var payload protocol.PingResultPayload
	if !httpx.BindJSON(c, &payload) {
		return
	}
	if err := ping.RecordResultWithError(c.Request.Context(), authNode.NodeID, payload); httpx.AbortBadRequestOnError(c, err) {
		return
	}
	c.JSON(http.StatusOK, response.OKNil())
}

// ReportExecResultHandler records a node's remote command execution result.
// @Summary 上报远程执行结果
// @Description Agent 上报远程 Shell 命令的执行输出与退出码；请求头需携带 X-Agent-Token
// @Tags openflare-agent
// @Accept json
// @Produce json
// @Security AgentTokenAuth
// @Param body body protocol.RemoteExecResultPayload true "远程执行结果"
// @Success 200 {object} response.Any "上报成功"
// @Failure 400 {object} response.Any "参数错误"
// @Failure 401 {object} response.Any "Token 无效"
// @Failure 404 {object} response.Any "任务不存在"
// @Router /api/v1/agent/exec/result [post]
func ReportExecResultHandler(c *gin.Context) {
	authNode, ok := NodeFromContext(c)
	if !ok {
		response.AbortUnauthorized(c, errInvalidAgentToken)
		return
	}
	var payload protocol.RemoteExecResultPayload
	if !httpx.BindJSON(c, &payload) {
		return
	}
	if err := ofexec.RecordResult(c.Request.Context(), authNode.NodeID, payload); httpx.AbortNotFoundIfMissing(c, err, "remote execution task was not found") {
		return
	}
	c.JSON(http.StatusOK, response.OKNil())
}

// WebSocketHandler upgrades an authenticated agent websocket connection.
// @Summary Agent WebSocket 连接
// @Description 升级为 WebSocket 长连接，用于实时推送配置同步、WAF IP 组等指令；需携带 X-Agent-Token
// @Tags openflare-agent
// @Security AgentTokenAuth
// @Failure 401 {object} response.Any "Token 无效"
// @Router /api/v1/agent/ws [get]
func WebSocketHandler(c *gin.Context) {
	authNode, ok := NodeFromContext(c)
	if !ok {
		response.AbortUnauthorized(c, errInvalidAgentToken)
		return
	}
	if authNode.Engine != enginePingora {
		response.AbortForbidden(c, errPingoraNodeRequired)
		return
	}
	websocket.ServeAgent(c, authNode.NodeID)
}

func requestClientIP(c *gin.Context) string {
	if ip := strings.TrimSpace(c.ClientIP()); ip != "" {
		return ip
	}
	return c.Request.RemoteAddr
}
