// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package openflare

import "github.com/gin-gonic/gin"

// V1BasePath is the OpenFlare console API prefix under /api/v1/custom.
const V1BasePath = "/api/v1/custom/openflare"

// RegisterV1Routes mounts OpenFlare management console APIs under /custom/openflare.
// The parent group must already be the /custom router group from v1/custom.go.
func RegisterV1Routes(customGroup *gin.RouterGroup) {
	group := customGroup.Group("/openflare")
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
	registerUpdateRoutes(group)
}
