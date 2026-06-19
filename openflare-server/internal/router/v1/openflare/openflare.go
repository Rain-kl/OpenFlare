// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package openflare registers OpenFlare HTTP routes.
// Management console APIs are mounted via RegisterV1Routes under /api/v1/d.
// Agent/Relay/Tunnel protocol routes are mounted via RegisterRoutes under /api/v1.
package openflare

import "github.com/gin-gonic/gin"

// RegisterRoutes mounts Agent/Relay/Tunnel protocol routes under the /api/v1 group.
func RegisterRoutes(apiV1Router *gin.RouterGroup) {
	registerAgentRoutes(apiV1Router)
	registerRelayRoutes(apiV1Router)
	registerTunnelRoutes(apiV1Router)
}
