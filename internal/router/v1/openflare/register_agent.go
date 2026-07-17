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
		discoveryRoute.Use(agent.RegisterAuth())
		{
			discoveryRoute.POST("/nodes/register", agent.RegisterHandler)
		}

		authorizedRoute := agentRoute.Group("/")
		authorizedRoute.Use(agent.Auth())
		{
			authorizedRoute.GET("/ws", agent.WebSocketHandler)
			authorizedRoute.POST("/nodes/heartbeat", agent.HeartbeatHandler)
			authorizedRoute.GET("/config-versions/active", agent.GetActiveConfigHandler)
			authorizedRoute.GET("/pages/deployments/:deployment_id/hash", agent.GetPagesDeploymentHashHandler)
			authorizedRoute.GET("/pages/deployments/:deployment_id/package", agent.DownloadPagesPackageHandler)
			authorizedRoute.GET("/pages/projects/:project_id/latest/hash", agent.GetPagesProjectLatestHashHandler)
			authorizedRoute.GET("/pages/projects/:project_id/latest/package", agent.DownloadPagesProjectLatestPackageHandler)
			authorizedRoute.POST("/waf/ip-groups/sync", agent.SyncWAFIPGroupsHandler)
			authorizedRoute.POST("/apply-logs", agent.ReportApplyLogHandler)
		}
	}
}
