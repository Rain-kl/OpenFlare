// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package openflare registers OpenFlare HTTP routes.
// Management console APIs are mounted via RegisterV1Routes under /api/v1/custom/openflare.
// Agent/Relay/Flared protocol routes are mounted via RegisterRoutes under /api.
package openflare

import "github.com/gin-gonic/gin"

// RegisterRoutes mounts Agent/Relay/Flared protocol routes under the /api group.
func RegisterRoutes(apiGroup *gin.RouterGroup) {
	registerAgentRoutes(apiGroup)
	registerRelayRoutes(apiGroup)
	registerFlaredRoutes(apiGroup)
}
