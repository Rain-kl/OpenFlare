// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package v1

import (
	"github.com/Rain-kl/Wavelet/internal/apps/openflare/agent"
	"github.com/Rain-kl/Wavelet/internal/apps/openflare/flared"
	"github.com/Rain-kl/Wavelet/internal/apps/openflare/relay"
	"github.com/gin-gonic/gin"
)

func registerOpenFlareEdgeRoutes(apiV1Router *gin.RouterGroup) {
	registerOpenFlareAgentRoutes(apiV1Router)
	registerOpenFlareRelayRoutes(apiV1Router)
	registerOpenFlareTunnelRoutes(apiV1Router)
}

func registerOpenFlareAgentRoutes(apiV1Router *gin.RouterGroup) {
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

func registerOpenFlareRelayRoutes(apiV1Router *gin.RouterGroup) {
	relayRoute := apiV1Router.Group("/relay")
	relayRoute.Use(relay.Auth())
	{
		relayRoute.POST("/heartbeat", relay.PostHeartbeat)
		relayRoute.GET("/ws", relay.GetWebSocket)
	}
}

func registerOpenFlareTunnelRoutes(apiV1Router *gin.RouterGroup) {
	tunnelRoute := apiV1Router.Group("/tunnel")
	tunnelRoute.Use(flared.TunnelAuth())
	{
		tunnelRoute.POST("/heartbeat", flared.PostHeartbeat)
		tunnelRoute.GET("/config/active", flared.GetActiveConfig)
		tunnelRoute.POST("/apply-log", flared.PostApplyLog)
		tunnelRoute.GET("/ws", flared.GetWebSocket)
	}
}
