// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package agent

import (
	"net/http"
	"strconv"

	"github.com/Rain-kl/Wavelet/internal/apps/openflare/compat"
	"github.com/Rain-kl/Wavelet/internal/apps/openflare/pages"
	"github.com/Rain-kl/Wavelet/internal/apps/openflare/websocket"
	"github.com/gin-gonic/gin"
)

// RegisterRoutes mounts agent API routes under /agent.
func RegisterRoutes(apiGroup *gin.RouterGroup) {
	agentRoute := apiGroup.Group("/agent")
	{
		discoveryRoute := agentRoute.Group("/")
		discoveryRoute.Use(AgentRegisterAuth())
		{
			discoveryRoute.POST("/nodes/register", RegisterHandler)
		}

		authorizedRoute := agentRoute.Group("/")
		authorizedRoute.Use(AgentAuth())
		{
			authorizedRoute.GET("/ws", AgentWebSocketHandler)
			authorizedRoute.POST("/nodes/heartbeat", HeartbeatHandler)
			authorizedRoute.GET("/config-versions/active", GetActiveConfigHandler)
			authorizedRoute.GET("/pages/deployments/:deployment_id/package", DownloadPagesPackageHandler)
			authorizedRoute.POST("/waf/ip-groups/sync", SyncWAFIPGroupsHandler)
			authorizedRoute.POST("/apply-logs", ReportApplyLogHandler)
		}
	}
}

// RegisterHandler registers or discovers an agent node.
func RegisterHandler(c *gin.Context) {
	var payload NodePayload
	if !compat.BindJSON(c, &payload) {
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
	if err != nil {
		compat.Fail(c, err.Error())
		return
	}
	compat.OK(c, result)
}

// HeartbeatHandler records agent heartbeat state.
func HeartbeatHandler(c *gin.Context) {
	var payload NodePayload
	if !compat.BindJSON(c, &payload) {
		return
	}
	payload.IP = resolveReportedNodeIP(payload.IP, c.Request.RemoteAddr)

	authNode, ok := AgentNodeFromContext(c)
	if !ok {
		compat.Unauthorized(c, errInvalidAgentToken)
		return
	}

	response, err := HeartbeatNode(c.Request.Context(), authNode, payload)
	if err != nil {
		compat.Fail(c, err.Error())
		return
	}
	okWithExtras(c, response.Node, gin.H{
		"agent_settings": response.AgentSettings,
		"active_config":  response.ActiveConfig,
		"waf_ip_groups":  response.WAFIPGroups,
	})
}

// GetActiveConfigHandler returns the active configuration version.
func GetActiveConfigHandler(c *gin.Context) {
	if _, ok := AgentNodeFromContext(c); !ok {
		compat.Unauthorized(c, errNodeMissingFromContext)
		return
	}
	config, err := GetActiveConfig(c.Request.Context())
	if err != nil {
		compat.Fail(c, err.Error())
		return
	}
	compat.OK(c, config)
}

// SyncWAFIPGroupsHandler syncs WAF IP groups for an agent.
func SyncWAFIPGroupsHandler(c *gin.Context) {
	var input WAFIPGroupSyncInput
	if !compat.BindJSON(c, &input) {
		return
	}
	result, err := SyncWAFIPGroups(c.Request.Context(), input)
	if err != nil {
		compat.Fail(c, err.Error())
		return
	}
	compat.OK(c, result)
}

// ReportApplyLogHandler records an agent apply log entry.
func ReportApplyLogHandler(c *gin.Context) {
	var payload ApplyLogPayload
	if !compat.BindJSON(c, &payload) {
		return
	}
	if authNode, ok := AgentNodeFromContext(c); ok {
		payload.NodeID = authNode.NodeID
	}
	log, err := ReportApplyLog(c.Request.Context(), payload)
	if err != nil {
		compat.Fail(c, err.Error())
		return
	}
	compat.OK(c, log)
}

// DownloadPagesPackageHandler streams the Pages deployment artifact to an authenticated agent.
func DownloadPagesPackageHandler(c *gin.Context) {
	deploymentID, ok := pagesDeploymentIDParam(c)
	if !ok {
		return
	}
	packageObj, fileName, err := pages.OpenDeploymentPackage(c.Request.Context(), deploymentID)
	if err != nil {
		compat.Fail(c, err.Error())
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
		compat.Fail(c, "无效的 ID")
		return 0, false
	}
	id64, err := strconv.ParseUint(raw, 10, 64)
	if err != nil || id64 == 0 {
		compat.Fail(c, "无效的 ID")
		return 0, false
	}
	return uint(id64), true
}

// AgentWebSocketHandler upgrades an authenticated agent websocket connection.
func AgentWebSocketHandler(c *gin.Context) {
	authNode, ok := AgentNodeFromContext(c)
	if !ok {
		compat.Unauthorized(c, errInvalidAgentToken)
		return
	}
	websocket.ServeAgent(c, authNode.NodeID, HandleWSStatus)
}

func okWithExtras(c *gin.Context, data any, extras gin.H) {
	payload := gin.H{
		"success": true,
		"message": "",
		"data":    data,
	}
	for key, value := range extras {
		payload[key] = value
	}
	c.JSON(http.StatusOK, payload)
}
