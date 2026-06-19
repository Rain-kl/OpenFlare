// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package openflare

import (
	"github.com/Rain-kl/Wavelet/internal/apps/openflare/agent"
	"github.com/gin-gonic/gin"
)

func registerAgentRoutes(apiV1Router *gin.RouterGroup) {
	agentRoute := apiV1Router.Group("/agent")
	{
		discoveryRoute := agentRoute.Group("/")
		discoveryRoute.Use(agent.AgentRegisterAuth())
		{
			discoveryRoute.POST("/nodes/register", agent.RegisterHandler)
		}

		authorizedRoute := agentRoute.Group("/")
		authorizedRoute.Use(agent.AgentAuth())
		{
			authorizedRoute.GET("/ws", agent.AgentWebSocketHandler)
			authorizedRoute.POST("/nodes/heartbeat", agent.HeartbeatHandler)
			authorizedRoute.GET("/config-versions/active", agent.GetActiveConfigHandler)
			authorizedRoute.GET("/pages/deployments/:deployment_id/package", agent.DownloadPagesPackageHandler)
			authorizedRoute.POST("/waf/ip-groups/sync", agent.SyncWAFIPGroupsHandler)
			authorizedRoute.POST("/apply-logs", agent.ReportApplyLogHandler)
		}
	}
}
