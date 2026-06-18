// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package openflare

import (
	"github.com/Rain-kl/Wavelet/internal/apps/openflare/flared"
	"github.com/Rain-kl/Wavelet/internal/apps/openflare/relay"
	"github.com/gin-gonic/gin"
)

func registerRelayRoutes(apiGroup *gin.RouterGroup) {
	relayRoute := apiGroup.Group("/relay")
	relayRoute.Use(relay.RelayAuth())
	{
		relayRoute.POST("/heartbeat", relay.PostHeartbeat)
		relayRoute.GET("/ws", relay.GetWebSocket)
	}
}

func registerFlaredRoutes(apiGroup *gin.RouterGroup) {
	flaredRoute := apiGroup.Group("/flared")
	flaredRoute.Use(flared.TunnelAuth())
	{
		flaredRoute.POST("/heartbeat", flared.PostHeartbeat)
		flaredRoute.GET("/config/active", flared.GetActiveConfig)
		flaredRoute.POST("/apply-log", flared.PostApplyLog)
		flaredRoute.GET("/ws", flared.GetWebSocket)
	}
}
