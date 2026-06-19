// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package openflare

import "github.com/gin-gonic/gin"

// V1BasePath is the OpenFlare console API prefix under /api/v1.
const V1BasePath = "/api/v1/d"

// RegisterV1Routes mounts OpenFlare management console APIs under /api/v1/d.
func RegisterV1Routes(apiV1Router *gin.RouterGroup) {
	group := apiV1Router.Group("/d")
	registerOptionRoutes(group)
	registerOriginRoutes(group)
	registerApplyLogRoutes(group)
	registerProxyRouteRoutes(group)
	registerNodeRoutes(group)
	registerWAFRoutes(group)
	registerTLSRoutes(group)
	registerConfigVersionRoutes(group)
	registerPagesRoutes(group)
	registerDashboardRoutes(group)
	registerObservabilityRoutes(group)
}
