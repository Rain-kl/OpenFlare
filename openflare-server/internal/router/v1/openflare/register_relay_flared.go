// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package openflare

import (
	"github.com/Rain-kl/Wavelet/internal/apps/openflare/flared"
	"github.com/Rain-kl/Wavelet/internal/apps/openflare/relay"
	"github.com/gin-gonic/gin"
)

func registerRelayRoutes(apiV1Router *gin.RouterGroup) {
	relayRoute := apiV1Router.Group("/relay")
	relayRoute.Use(relay.RelayAuth())
	{
		relayRoute.POST("/heartbeat", relay.PostHeartbeat)
		relayRoute.GET("/ws", relay.GetWebSocket)
	}
}

func registerTunnelRoutes(apiV1Router *gin.RouterGroup) {
	tunnelRoute := apiV1Router.Group("/tunnel")
	tunnelRoute.Use(flared.TunnelAuth())
	{
		tunnelRoute.POST("/heartbeat", flared.PostHeartbeat)
		tunnelRoute.GET("/config/active", flared.GetActiveConfig)
		tunnelRoute.POST("/apply-log", flared.PostApplyLog)
		tunnelRoute.GET("/ws", flared.GetWebSocket)
	}
}
