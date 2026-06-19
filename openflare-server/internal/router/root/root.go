// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package root registers custom business routes and frontend serving.
package root

import (
	"github.com/gin-gonic/gin"
)

// RegisterFrontend is a package-level variable overridden by frontend.go when built with embed_frontend.
var RegisterFrontend = func(_ *gin.Engine) {
	// No-op by default
}

// RegisterRootRoutes registers custom business routes that belong to the root path.
func RegisterRootRoutes(r *gin.Engine) {
	// 1. Default root routes (/f/:id, /robots.txt, and /swagger/*any)
	RegisterDefaultRootRoutes(r)

	// 2. Register custom serving
	RegisterCustomRootRoutes(r)

	// 3. Register frontend serving
	RegisterFrontend(r)
}
