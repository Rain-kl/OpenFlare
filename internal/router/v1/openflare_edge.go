// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package v1

import (
	"github.com/Rain-kl/Wavelet/internal/apps/openflare/agent"
	"github.com/Rain-kl/Wavelet/internal/apps/openflare/tunnel"
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
			authorizedRoute.GET("/config/pull", agent.PullConfigHandler)
			authorizedRoute.POST("/tls/keys", agent.FetchTLSKeysHandler)
			authorizedRoute.POST("/nodes/heartbeat", agent.HeartbeatHandler)
			authorizedRoute.GET("/pages/projects/:project_id/latest/hash", agent.GetPagesProjectLatestHashHandler)
			authorizedRoute.GET("/pages/projects/:project_id/latest/package", agent.DownloadPagesProjectLatestPackageHandler)
			authorizedRoute.POST("/apply-logs", agent.ReportApplyLogHandler)
		}
	}
}

func registerOpenFlareRelayRoutes(apiV1Router *gin.RouterGroup) {
	relayRoute := apiV1Router.Group("/relay")
	{
		discoveryRoute := relayRoute.Group("/")
		discoveryRoute.Use(tunnel.RelayRegisterAuth())
		{
			discoveryRoute.POST("/register", tunnel.RelayRegisterHandler)
		}

		authorizedRoute := relayRoute.Group("/")
		authorizedRoute.Use(tunnel.RelayAuth())
		{
			authorizedRoute.POST("/heartbeat", tunnel.RelayHeartbeatHandler)
			authorizedRoute.GET("/config", tunnel.RelayConfigHandler)
			authorizedRoute.GET("/ws", tunnel.RelayWebSocketHandler)
		}
	}
}

func registerOpenFlareTunnelRoutes(apiV1Router *gin.RouterGroup) {
	tunnelRoute := apiV1Router.Group("/tunnel")
	tunnelRoute.Use(tunnel.TunnelAuth())
	{
		tunnelRoute.POST("/heartbeat", tunnel.TunnelHeartbeatHandler)
		tunnelRoute.GET("/config", tunnel.TunnelConfigHandler)
		tunnelRoute.POST("/attachments", tunnel.TunnelNegotiateAttachmentsHandler)
		tunnelRoute.GET("/ws", tunnel.TunnelWebSocketHandler)
	}
}
